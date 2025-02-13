package collector

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"log/slog"
	"testing"
	"time"
)

func Test_run(t *testing.T) {
	//l := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	l := slog.New(slog.DiscardHandler)

	r := prometheus.NewRegistry()
	reader := NewTopReader(l, 100*time.Millisecond)
	reader.topRunner = &fakeRunner{interval: 100 * time.Millisecond}

	go func() {
		assert.NoError(t, runWithTopReader(t.Context(), r, reader, l))
	}()

	assert.Eventually(t, func() bool {
		n, err := testutil.GatherAndCount(r)
		return err == nil && n == 15
	}, 5*time.Second, 100*time.Millisecond)
}
