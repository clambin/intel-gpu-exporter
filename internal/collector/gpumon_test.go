package collector

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"codeberg.org/clambin/go-common/set"
	"github.com/clambin/intel-gpu-exporter/intel-gpu-top/testutil"
	"github.com/stretchr/testify/assert"
)

func TestConfiguration_buildCommand(t *testing.T) {
	tests := []struct {
		name string
		cfg  Configuration
		want []string
	}{
		{
			name: "defaults",
			cfg:  Configuration{},
			want: []string{"/usr/bin/intel_gpu_top", "-J", "-s", "1000"},
		},
		{
			name: "with device",
			cfg:  Configuration{Device: "/dev/sda"},
			want: []string{"/usr/bin/intel_gpu_top", "-J", "-s", "1000", "-d", "/dev/sda"},
		},
		{
			name: "with interval",
			cfg:  Configuration{Interval: 5 * time.Second},
			want: []string{"/usr/bin/intel_gpu_top", "-J", "-s", "5000"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.cfg.buildCommand())
		})
	}
}

func TestGpuMon_run(t *testing.T) {
	fake := fakeRunner{interval: 50 * time.Millisecond}
	g := gpuMon{
		topRunner:  &fake,
		aggregator: &aggregator{clients: set.New[string]()},
		timeout:    5 * fake.interval,
		logger:     slog.New(slog.DiscardHandler), //slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})),
	}

	// start the reader
	go func() { assert.NoError(t, g.run(t.Context())) }()

	// wait for at least 5 measurements to be made
	assert.Eventually(t, func() bool {
		return g.aggregator.len() > 4
	}, time.Second, 100*time.Millisecond)

	// stop the current writer
	fake.stop()

	// clear the collected samples
	g.aggregator.flush()

	// wait for reader to time out and start a new writer.
	assert.Eventually(t, func() bool {
		return g.aggregator.len() > 0
	}, time.Second, time.Millisecond)
}

var _ topRunner = &fakeRunner{}

type fakeRunner struct {
	interval time.Duration
	cancel   atomic.Value
}

func (f *fakeRunner) start(ctx context.Context, _ ...string) (io.Reader, error) {
	subCtx, cancel := context.WithCancel(ctx)
	f.cancel.Store(cancel)
	r, w := io.Pipe()
	go func() {
		defer func() { _ = r.Close() }()
		f.sendPayloads(subCtx, w)
	}()
	return r, nil
}

func (f *fakeRunner) stop() {
	if cancel, ok := f.cancel.Load().(context.CancelFunc); ok && cancel != nil {
		cancel()
	}
}

func (f *fakeRunner) running() bool {
	return f.cancel.Load() != nil
}

func (f *fakeRunner) sendPayloads(ctx context.Context, w io.Writer) {
	_, _ = w.Write([]byte("["))
	defer func() { _, _ = w.Write([]byte("]\n")) }()
	first := true
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(f.interval):
			payload := testutil.SinglePayload
			if !first {
				payload = "," + payload
			}
			first = false

			if _, err := w.Write([]byte(payload)); err != nil {
				panic(err)
			}
		}
	}
}
