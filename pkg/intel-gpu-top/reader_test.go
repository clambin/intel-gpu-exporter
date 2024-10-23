package intel_gpu_top

import (
	"context"
	"github.com/clambin/gpumon/pkg/intel-gpu-top/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestReadGPUStats(t *testing.T) {
	tests := []struct {
		name   string
		array  bool
		commas bool
	}{
		{"v1.17", false, false},
		{"v1.18a", true, true},
		{"v1.18b", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			const recordCount = 10
			r := testutil.FakeServer(ctx, []byte(testutil.SinglePayload), recordCount, tt.array, tt.commas, 100*time.Millisecond)

			var got int
			for _, err := range ReadGPUStats(&V118toV117{Reader: r}) {
				require.NoError(t, err)
				got++
			}
			assert.Equal(t, recordCount, got)
		})
	}
}
