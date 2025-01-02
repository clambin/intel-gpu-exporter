package collector

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"sync/atomic"
)

// Runner starts / stops a process and collects its stdout output.
type Runner struct {
	logger     *slog.Logger
	cmd        atomic.Pointer[exec.Cmd]
	runCounter atomic.Int32
}

func (t *Runner) Start(ctx context.Context, cmdline []string) (io.Reader, error) {
	cmd := exec.CommandContext(ctx, cmdline[0], cmdline[1:]...)
	stdout, _ := cmd.StdoutPipe()
	t.runCounter.Add(1)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("could not start command: %w", err)
	}
	t.logger.Debug("started top command", "count", t.runCounter.Load(), "pid", cmd.Process.Pid)
	t.cmd.Store(cmd)
	return stdout, nil
}

func (t *Runner) Stop() {
	if cmd := t.cmd.Load(); cmd != nil {
		t.logger.Debug("stopping top command", "count", t.runCounter.Load(), "pid", cmd.Process.Pid)
		t.cmd.Store(nil)
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
}

func (t *Runner) Running() bool {
	return t.cmd.Load() != nil
}
