package tmp

import (
	"context"
	"github.com/clambin/intel-gpu-exporter/pkg/intel-gpu-top/testutil"
	"github.com/stretchr/testify/assert"
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestTopReader_Run(t *testing.T) {
	//l := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	l := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := NewTopReader(l, 100*time.Millisecond)
	fake := fakeRunner{}
	r.topRunner = &fake
	r.timeout = time.Second

	// start the reader
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error)
	go func() { errCh <- r.Run(ctx) }()

	// wait for at least 5 measurements to be made
	assert.Eventually(t, func() bool {
		return r.Aggregator.len() >= 5
	}, time.Second, 100*time.Millisecond)

	// remember the current number of measurements
	got := r.Aggregator.len()

	// stop the current writer
	fake.cancel()

	// wait for reader to time out and start a new writer.
	assert.Eventually(t, func() bool {
		return r.Aggregator.len() > got
	}, 2*time.Second, 100*time.Millisecond)

	// shutdown
	cancel()
	assert.NoError(t, <-errCh)
}

var _ topRunner = &fakeRunner{}

type fakeRunner struct {
	cancel context.CancelFunc
}

func (f *fakeRunner) Start(ctx context.Context, interval time.Duration) (io.Reader, error) {
	var subCtx context.Context
	subCtx, f.cancel = context.WithCancel(ctx)
	r, w := io.Pipe()
	go func() {
		defer func() { _ = r.Close() }()
		for {
			select {
			case <-subCtx.Done():
				return
			case <-time.After(interval):
				if _, err := w.Write([]byte(testutil.SinglePayload)); err != nil {
					panic(err)
				}
			}
		}
	}()
	return r, nil
}

func (f *fakeRunner) Stop() {
	f.cancel()
}
