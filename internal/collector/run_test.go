package collector

import (
	"context"
	"github.com/clambin/intel-gpu-exporter/pkg/intel-gpu-top/testutil"
	"github.com/prometheus/client_golang/prometheus"
	testutil2 "github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"io"
	"log/slog"
	"testing"
	"time"
)

func Test_run(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	r := prometheus.NewRegistry()
	top := testutil.FakeServer(ctx, []byte(testutil.SinglePayload), 5, false, false, 0)
	errCh := make(chan error)
	go func() {
		errCh <- run(ctx, r, top, slog.New(slog.NewTextHandler(io.Discard, nil)))
	}()

	assert.Eventually(t, func() bool {
		n, err := testutil2.GatherAndCount(r)
		return err == nil && n == 15
	}, time.Second*5, time.Millisecond*200)

	cancel()
	assert.NoError(t, <-errCh)
}

func Test_buildCommand(t *testing.T) {
	assert.Equal(t, []string{"intel_gpu_top", "-J", "-s", "5000"}, buildCommand(5*time.Second))
	assert.Equal(t, []string{"intel_gpu_top", "-J", "-s", "500"}, buildCommand(500*time.Millisecond))
	assert.Equal(t, []string{"intel_gpu_top", "-J", "-s", "60000"}, buildCommand(time.Minute))
}
