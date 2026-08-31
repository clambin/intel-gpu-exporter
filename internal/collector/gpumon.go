package collector

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"codeberg.org/clambin/go-common/flagger"
	igt "github.com/clambin/intel-gpu-exporter/intel-gpu-top"
)

type Configuration struct {
	flagger.Log
	flagger.Prom
	Device   string        `flagger.usage:"Device to collect statistics from (-d parameter of intel_gpu_top)"`
	Interval time.Duration `flagger.usage:"Interval to collect statistics"`
	Pprof    bool          `flagger.usage:"Enable pprof"`
}

func (c Configuration) buildCommand() []string {
	topCommand := []string{
		"/usr/bin/intel_gpu_top",
		"-J",
		"-s", strconv.FormatInt(cmp.Or(c.Interval.Milliseconds(), 1000), 10),
	}
	if c.Device != "" {
		topCommand = append(topCommand, "-d", c.Device)
	}
	return topCommand
}

type gpuMon struct {
	topRunner  topRunner
	aggregator *aggregator
	logger     *slog.Logger
	cfg        Configuration
	timeout    time.Duration
	lastUpdate atomic.Pointer[time.Time]
	reader     igt.Reader
}

type topRunner interface {
	start(ctx context.Context, args ...string) (io.Reader, error)
	stop()
	running() bool
}

func (g *gpuMon) run(ctx context.Context) error {
	aliveTicker := time.NewTicker(g.timeout)
	defer aliveTicker.Stop()

	for {
		if err := g.ensureIsRunning(ctx); err != nil {
			g.logger.Error("failed to start intel_gpu_top", "err", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-aliveTicker.C:
		}
	}
}

func (g *gpuMon) ensureIsRunning(ctx context.Context) error {
	// check we're still receiving updates
	lastUpdate := g.lastUpdate.Load()
	if lastUpdate != nil && time.Since(*lastUpdate) < g.timeout {
		return nil
	}

	// not receiving updates: we need to (re-)start intel_gpu_top
	g.logger.Warn("(re)starting intel-gpu-top")

	if g.topRunner.running() {
		// shut down the current instance of intel_gpu_top
		g.topRunner.stop()
	}

	// start a new instance of intel_gpu_top
	cmdline := g.cfg.buildCommand()
	g.logger.Debug("top command built", "cmd", strings.Join(cmdline, " "))
	stdout, err := g.topRunner.start(ctx, cmdline...)
	if err != nil {
		return fmt.Errorf("intel-gpu-top: %w", err)
	}

	// start aggregating from the new instance's output.
	// any previous goroutines stop as soon as the previous stdout is closed (when we call g.topRunner.stop() above).
	go func() {
		for stat := range g.reader.Seq(stdout) {
			g.logger.Debug("collected gpu stat", "stat", stat)
			g.aggregator.add(stat)
			g.lastUpdate.Store(new(time.Now()))
		}
		if err := g.reader.Err(); err != nil {
			g.logger.Warn("error reading intel-gpu-top output", "err", err)
		}
	}()
	return nil
}
