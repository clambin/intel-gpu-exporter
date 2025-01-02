package collector

import (
	"context"
	"fmt"
	igt "github.com/clambin/intel-gpu-exporter/pkg/intel-gpu-top"
	"io"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// TopReader starts intel-gpu-top, reads/decodes its output and collects the sampler for the Collector to export them to Prometheus.
//
// TopReader regularly checks if it's still receiving data from intel-gpu-top. After a timeout, it stops the running instance
// of intel-gpu-top and start a new instance.
type TopReader struct {
	topRunner
	logger *slog.Logger
	Aggregator
	interval    time.Duration
	timeout     time.Duration
	writerCount int
	readerCount int
}

// topRunner interface allows us to override TopRunner during testing.
type topRunner interface {
	Start(ctx context.Context, interval time.Duration) (io.Reader, error)
	Stop()
}

// NewTopReader returns a new TopReader that will measure GPU usage at `interval` seconds.
func NewTopReader(logger *slog.Logger, interval time.Duration) *TopReader {
	r := TopReader{
		logger:     logger,
		Aggregator: Aggregator{logger: logger.With("subsystem", "aggregator")},
		topRunner:  &TopRunner{logger: logger.With("subsystem", "runner")},
		interval:   interval,
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
			r.topRunner.Stop()
			return nil
		case <-ticker.C:
		}
	}
}

func (r *TopReader) ensureReaderIsRunning(ctx context.Context) (err error) {
	// if we have received data  `timeout` seconds, do nothing
	last, ok := r.Aggregator.LastUpdate()
	if ok && time.Since(last) < r.timeout {
		return nil
	}
	if r.topRunner != nil {
		// Shut down the current instance of igt.
		r.logger.Debug("timed out waiting for data. restarting intel-gpu-top", "waitTime", last)
		r.topRunner.Stop()
	}

	// start a new instance of igt
	stdout, err := r.topRunner.Start(ctx, r.interval)
	if err != nil {
		return fmt.Errorf("intel-gpu-top: %w", err)
	}
	// start aggregating from the new instance's output.
	// any previous goroutines will stop as soon as the previous stdout is closed.
	go func() {
		stdout = igt.V118toV117{Reader: stdout}
		if err := r.Aggregator.Read(stdout); err != nil {
			r.logger.Error("failed to start reader", "err", err)
		}
	}()
	// reset the timer
	r.Aggregator.lastUpdate.Store(time.Now())
	return nil
}

// TopRunner starts / stops an instance of intel-gpu-top
type TopRunner struct {
	logger     *slog.Logger
	cmd        *exec.Cmd
	startCount int
}

func (t *TopRunner) Start(ctx context.Context, interval time.Duration) (io.Reader, error) {
	cmdline := buildCommand(interval)
	t.logger.Debug("top command built", "interval", interval, "cmd", strings.Join(cmdline, " "))
	t.cmd = exec.CommandContext(ctx, cmdline[0], cmdline[1:]...)
	stdout, err := t.cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("could not get stdout pipe: %w", err)
	}
	t.startCount++
	if err = t.cmd.Start(); err != nil {
		return nil, fmt.Errorf("could not start command: %w", err)
	}
	t.logger.Debug("started top command", "count", t.startCount, "pid", t.cmd.Process.Pid)
	return stdout, nil
}

func buildCommand(scanInterval time.Duration) []string {
	//const gpuTopCommand = "ssh ubuntu@nuc1 sudo intel_gpu_top -J -s"
	const gpuTopCommand = "intel_gpu_top -J -s"

	return append(
		strings.Split(gpuTopCommand, " "),
		strconv.Itoa(int(scanInterval.Milliseconds())),
	)
}

func (t *TopRunner) Stop() {
	if t.cmd != nil {
		t.logger.Debug("stopping top command", "count", t.startCount, "pid", t.cmd.Process.Pid)
		_ = t.cmd.Process.Kill()
		_ = t.cmd.Wait()
		t.cmd = nil
	}
}
