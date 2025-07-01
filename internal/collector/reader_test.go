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

func TestTopReader_Run(t *testing.T) {
	l := slog.New(slog.DiscardHandler)
	r := newTopReader(Configuration{Interval: 100 * time.Millisecond}, l)
	fake := fakeRunner{interval: 100 * time.Millisecond}
	r.topRunner = &fake
	r.timeout = time.Second

	// start the reader
	go func() { assert.NoError(t, r.Run(t.Context())) }()

	// wait for at least 5 measurements to be made
	assert.Eventually(t, func() bool {
		return r.Collector.len() >= 5
	}, time.Second, 100*time.Millisecond)

	// remember the current number of measurements
	got := r.Collector.len()

	// stop the current writer
	fake.stop()

	// wait for reader to time out and start a new writer.
	assert.Eventually(t, func() bool {
		return r.Collector.len() > got
	}, 2*time.Second, 100*time.Millisecond)
}

var _ topRunner = &fakeRunner{}

type fakeRunner struct {
	interval time.Duration
	cancel   atomic.Value
}

func (f *fakeRunner) start(ctx context.Context, _ []string) (io.Reader, error) {
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
	if cancel := f.cancel.Load().(context.CancelFunc); cancel != nil {
		cancel()
	}
}

func (f *fakeRunner) running() bool {
	return f.cancel.Load() != nil
}
