package collector

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"codeberg.org/clambin/go-common/flagger"
	igt "github.com/clambin/intel-gpu-exporter/intel-gpu-top"
)

type Configuration struct {
	flagger.Log
	flagger.Prom
	Device   string        `flagger.usage:"Device to collect statistics from (-d parameter of intel_gpu_top)"`
	Interval time.Duration `flagger.usage:"Interval to collect statistics"`
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
	lock       sync.RWMutex
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
	g.lock.RLock()
	defer g.lock.RUnlock()

	// check we're still receiving updates
	timeSince := time.Since(g.aggregator.lastUpdated())
	if timeSince < g.timeout {
		return nil
	}

	// not receiving updates: we need to (re-)start intel_gpu_top
	if g.topRunner.running() {
		// Shut down the current instance of intel_gpu_top
		g.logger.Warn("timed out waiting for data. restarting intel-gpu-top", "waitTime", timeSince)
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
		for stat, err := range igt.ReadGPUStats(stdout) {
			g.logger.Debug("collected gpu stat", "stat", stat, "err", err)
			if err == nil {
				g.aggregator.add(stat)
			}
		}
	}()
	return nil
}
