package intel_gpu_top

import (
	"bytes"
	"context"
	"errors"
	"github.com/clambin/intel-gpu-exporter/pkg/intel-gpu-top/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
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

// Current:
// BenchmarkV118toV117-16             89842             13269 ns/op              64 B/op          2 allocs/op
// After removal of bytes.LastIndex:
// BenchmarkV118toV117-16           2265168               525.1 ns/op            64 B/op          2 allocs/op
func BenchmarkV118toV117(b *testing.B) {
	// generate input outside the benchmark
	var payload bytes.Buffer
	r := testutil.FakeServer(context.Background(), []byte(testutil.SinglePayload), 10, true, true, 0*time.Millisecond)
	if _, err := payload.ReadFrom(r); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	buf := make([]byte, 512)
	for range b.N {
		r = &V118toV117{Reader: bytes.NewReader(payload.Bytes())}

		var err error
		for !errors.Is(err, io.EOF) {
			clear(buf)
			_, err = r.Read(buf)
		}
	}
}
