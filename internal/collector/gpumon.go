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

	igt "github.com/clambin/intel-gpu-exporter/intel-gpu-top"
)

type gpuMon struct {
	aggregator *aggregator
	logger     *slog.Logger
	timeout    time.Duration
	topRunner  topRunner
	cfg        Configuration
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

	// we need (re-)start intel_gpu_top
	if g.topRunner.running() {
		// Shut down the current instance of intel_gpu_top
		g.logger.Warn("timed out waiting for data. restarting intel-gpu-top", "waitTime", timeSince)
		g.topRunner.stop()
	}

	// start a new instance of intel_gpu_top
	cmdline := buildCommand(g.cfg)
	g.logger.Debug("top command built", "cmd", strings.Join(cmdline, " "))
	stdout, err := g.topRunner.start(ctx, cmdline...)
	if err != nil {
		return fmt.Errorf("intel-gpu-top: %w", err)
	}

	// start aggregating from the new instance's output.
	// any previous goroutines stop as soon as the previous stdout is closed (when we call g.topRunner.stop() above).
	go func() {
		for stat := range igt.ReadGPUStats(&igt.V118toV117{Source: stdout}) {
			g.logger.Debug("collected gpu stat", "stat", stat)
			g.aggregator.add(stat)
		}
	}()
	return nil
}

func buildCommand(cfg Configuration) []string {
	topCommand := []string{
		"/usr/bin/intel_gpu_top",
		"-J",
		"-s", strconv.FormatInt(cmp.Or(cfg.Interval.Milliseconds(), 1000), 10),
	}
	if cfg.Device != "" {
		topCommand = append(topCommand, "-d", cfg.Device)
	}
	return topCommand
}
