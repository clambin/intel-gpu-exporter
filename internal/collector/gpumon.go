package collector

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	igt "github.com/clambin/intel-gpu-exporter/intel-gpu-top"
)

type gpuMon struct {
	logger     *slog.Logger
	samples    []igt.GPUStats
	lastUpdate time.Time
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
	if timeSince := time.Since(g.lastUpdate); timeSince < g.timeout {
		return nil
	}

	// we need (re-)start intel_gpu_top
	if g.topRunner.running() {
		// Shut down the current instance of intel_gpu_top
		g.logger.Warn("timed out waiting for data. restarting intel-gpu-top", "waitTime", time.Since(g.lastUpdate))
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
	go g.aggregate(&igt.V118toV117{Source: stdout})
	return nil
}

func (g *gpuMon) aggregate(r io.Reader) {
	for stat := range igt.ReadGPUStats(r) {
		g.logger.Debug("collected gpu stat", "stat", stat)
		g.lock.Lock()
		g.samples = append(g.samples, stat)
		g.lock.Unlock()
		g.lastUpdate = time.Now()
	}
}

func (g *gpuMon) stats() []igt.GPUStats {
	g.lock.RLock()
	defer g.lock.RUnlock()
	return slices.Clone(g.samples)
}

func (g *gpuMon) clear() {
	g.lock.Lock()
	defer g.lock.Unlock()
	g.samples = g.samples[:0]
}

func (g *gpuMon) collect() []igt.GPUStats {
	g.lock.RLock()
	defer g.lock.RUnlock()
	samples := slices.Clone(g.samples)
	g.samples = g.samples[:0]
	return samples
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
