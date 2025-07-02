package collector

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/clambin/intel-gpu-exporter/intel-gpu-top/testutil"
	"github.com/stretchr/testify/assert"
)

func Test_gpuMon_Run(t *testing.T) {
	fake := fakeRunner{interval: 50 * time.Millisecond}
	g := gpuMon{
		topRunner: &fake,
		timeout:   5 * fake.interval,
		logger:    slog.New(slog.DiscardHandler), //slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})),
	}

	// start the reader
	go func() { assert.NoError(t, g.run(t.Context())) }()

	// wait for at least 5 measurements to be made
	assert.Eventually(t, func() bool {
		return len(g.stats()) > 4
	}, time.Second, 100*time.Millisecond)

	// stop the current writer
	fake.stop()

	// clear the collected samples
	g.clear()

	// wait for reader to time out and start a new writer.
	assert.Eventually(t, func() bool {
		return len(g.stats()) > 0
	}, time.Second, time.Millisecond)
}

func Test_buildCommand(t *testing.T) {
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
			assert.Equal(t, tt.want, buildCommand(tt.cfg))
		})
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

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
		for {
			select {
			case <-subCtx.Done():
				return
			case <-time.After(f.interval):
				if _, err := w.Write([]byte(testutil.SinglePayload)); err != nil {
					panic(err)
				}
			}
		}
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
