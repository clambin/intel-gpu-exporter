package collector

import (
	"context"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"io"
	"log/slog"
	"testing"
	"time"
)

func Test_run(t *testing.T) {
	//l := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	l := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	r := prometheus.NewRegistry()
	reader := NewTopReader(l, 100*time.Millisecond)
	reader.topRunner = &fakeRunner{interval: 100 * time.Millisecond}

	errCh := make(chan error)
	go func() {
		errCh <- runWithTopReader(ctx, r, reader, l)
	}()

	assert.Eventually(t, func() bool {
		n, err := testutil.GatherAndCount(r)
		return err == nil && n == 15
	}, 5*time.Second, 100*time.Millisecond)

	cancel()
	assert.NoError(t, <-errCh)
}
