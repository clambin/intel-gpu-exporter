package collector

import (
	"log/slog"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestRun(t *testing.T) {
	//l := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	l := slog.New(slog.DiscardHandler)

	reader := NewTopReader(Configuration{Interval: 100 * time.Millisecond}, l)
	reader.topRunner = &fakeRunner{interval: 100 * time.Millisecond}
	r := prometheus.NewRegistry()

	go func() {
		assert.NoError(t, Run(t.Context(), r, reader, l))
	}()

	assert.Eventually(t, func() bool {
		n, err := testutil.GatherAndCount(r)
		return err == nil && n == 15
	}, 5*time.Second, 100*time.Millisecond)
}
