package collector

import (
	"context"
	"github.com/clambin/intel-gpu-exporter/pkg/intel-gpu-top/testutil"
	"github.com/stretchr/testify/assert"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

func Test_buildCommand(t *testing.T) {
	assert.Equal(t, []string{"intel_gpu_top", "-J", "-s", "1000"}, buildCommand(time.Second))
}

func TestTopReader_Run(t *testing.T) {
	//l := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	l := slog.New(slog.DiscardHandler)
	r := NewTopReader(l, 100*time.Millisecond)
	fake := fakeRunner{interval: 100 * time.Millisecond}
	r.topRunner = &fake
	r.timeout = time.Second

	// start the reader
	go func() { assert.NoError(t, r.Run(t.Context())) }()

	// wait for at least 5 measurements to be made
	assert.Eventually(t, func() bool {
		return r.Aggregator.len() >= 5
	}, time.Second, 100*time.Millisecond)

	// remember the current number of measurements
	got := r.Aggregator.len()

	// stop the current writer
	fake.Stop()

	// wait for reader to time out and start a new writer.
	assert.Eventually(t, func() bool {
		return r.Aggregator.len() > got
	}, 2*time.Second, 100*time.Millisecond)
}

var _ topRunner = &fakeRunner{}

type fakeRunner struct {
	interval time.Duration
	cancel   atomic.Value
}

func (f *fakeRunner) Start(ctx context.Context, _ []string) (io.Reader, error) {
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

func (f *fakeRunner) Stop() {
	if cancel := f.cancel.Load().(context.CancelFunc); cancel != nil {
		cancel()
	}
}

func (f *fakeRunner) Running() bool {
	return f.cancel.Load() != nil
}
