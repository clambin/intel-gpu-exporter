package collector

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	igt "github.com/clambin/intel-gpu-exporter/pkg/intel-gpu-top"
)

// TopReader starts intel-gpu-top, reads/decodes its output and collects the sampler for the Collector to export them to Prometheus.
//
// TopReader regularly checks if it's still receiving data from intel-gpu-top. After a timeout, it stops the running instance
// of intel-gpu-top and start a new instance.
type TopReader struct {
	cfg Configuration
	topRunner
	logger *slog.Logger
	Aggregator
	timeout time.Duration
}

// topRunner interface allows us to override Runner during testing.
type topRunner interface {
	Start(ctx context.Context, cmdline []string) (io.Reader, error)
	Stop()
	Running() bool
}

// NewTopReader returns a new TopReader that will measure GPU usage at `interval` seconds.
func NewTopReader(cfg Configuration, logger *slog.Logger) *TopReader {
	r := TopReader{
		logger:     logger,
		Aggregator: Aggregator{logger: logger.With("subsystem", "aggregator")},
		topRunner:  &Runner{logger: logger.With("subsystem", "runner")},
		cfg:        cfg,
		timeout:    15 * time.Second,
	}
	return &r
}

func (r *TopReader) Run(ctx context.Context) error {
	r.logger.Debug("starting reader")
	defer r.logger.Debug("shutting down reader")

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		if err := r.ensureReaderIsRunning(ctx); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			r.Stop()
			return nil
		case <-ticker.C:
		}
	}
}

func (r *TopReader) ensureReaderIsRunning(ctx context.Context) (err error) {
	// if we have received data  `timeout` seconds, do nothing
	last, ok := r.LastUpdate()
	if ok && time.Since(last) < r.timeout {
		return nil
	}
	if r.Running() {
		// Shut down the current instance of igt.
		r.logger.Warn("timed out waiting for data. restarting intel-gpu-top", "waitTime", time.Since(last))
		r.Stop()
	}

	// start a new instance of igt
	cmdline := buildCommand(r.cfg)
	r.logger.Debug("top command built", "cmd", strings.Join(cmdline, " "))

	stdout, err := r.Start(ctx, cmdline)
	if err != nil {
		return fmt.Errorf("intel-gpu-top: %w", err)
	}
	// start aggregating from the new instance's output.
	// any previous goroutines will stop as soon as the previous stdout is closed.
	go func() {
		stdout = &igt.V118toV117{Source: stdout}
		if err := r.Read(stdout); err != nil {
			r.logger.Error("failed to start reader", "err", err)
		}
	}()
	// reset the timer
	r.lastUpdate.Store(time.Now())
	return nil
}

func buildCommand(cfg Configuration) []string {
	topCommand := []string{
		"intel_gpu_top",
		"-J",
		"-s", strconv.FormatInt(cmp.Or(cfg.Interval.Milliseconds(), 1000), 10),
	}
	if cfg.Device != "" {
		topCommand = append(topCommand, "-d", cfg.Device)
	}

	return topCommand
}
