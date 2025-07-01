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

	"codeberg.org/clambin/go-common/set"
	igt "github.com/clambin/intel-gpu-exporter/intel-gpu-top"
)

// TopReader starts intel-gpu-top, reads/decodes its output and collects the sampler for the Collector to export them to Prometheus.
//
// TopReader regularly checks if it's still receiving data from intel-gpu-top. After a timeout, it stops the running instance
// of intel-gpu-top and start a new instance.
type TopReader struct {
	cfg Configuration
	topRunner
	logger *slog.Logger
	Collector
	timeout time.Duration
}

// topRunner interface allows us to override runner during testing.
type topRunner interface {
	start(ctx context.Context, cmdline []string) (io.Reader, error)
	stop()
	running() bool
}

// NewTopReader returns a new TopReader that measures GPU usage at `interval` seconds.
func NewTopReader(cfg Configuration, logger *slog.Logger) *TopReader {
	r := TopReader{
		logger:    logger,
		Collector: Collector{clients: set.New[string](), logger: logger.With("subsystem", "aggregator")},
		topRunner: &runner{logger: logger.With("subsystem", "runner")},
		cfg:       cfg,
		timeout:   15 * time.Second,
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
			r.stop()
			return nil
		case <-ticker.C:
		}
	}
}

func (r *TopReader) ensureReaderIsRunning(ctx context.Context) (err error) {
	// if we have received data  `timeout` seconds, do nothing
	last, ok := r.lastUpdate()
	if ok && time.Since(last) < r.timeout {
		return nil
	}
	if r.running() {
		// Shut down the current instance of igt.
		r.logger.Warn("timed out waiting for data. restarting intel-gpu-top", "waitTime", time.Since(last))
		r.stop()
	}

	// start a new instance of igt
	cmdline := buildCommand(r.cfg)
	r.logger.Debug("top command built", "cmd", strings.Join(cmdline, " "))

	stdout, err := r.start(ctx, cmdline)
	if err != nil {
		return fmt.Errorf("intel-gpu-top: %w", err)
	}
	// start aggregating from the new instance's output.
	// any previous goroutines will stop as soon as the previous stdout is closed.
	go func() {
		stdout = &igt.V118toV117{Source: stdout}
		if err := r.read(stdout); err != nil {
			r.logger.Error("failed to start reader", "err", err)
		}
	}()
	// reset the timer
	r.lastUpdated.Store(time.Now())
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
