package tmp

import (
	"context"
	igt "github.com/clambin/intel-gpu-exporter/pkg/intel-gpu-top"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"log/slog"
	"slices"
	"testing"
	"time"
)

func TestAggregator(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	const payloadCount = 4
	fake := fakeRunner{}
	r, _ := fake.Start(ctx, time.Millisecond)
	var a Aggregator
	a.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	errCh := make(chan error)
	go func() { errCh <- a.Read(r) }()

	// a.Read works asynchronously. Wait for all data to be read.
	assert.Eventually(t, func() bool { return len(a.EngineStats()) >= payloadCount }, time.Second, time.Millisecond)

	wantEngines := []string{"Render/3D", "Blitter", "Video", "VideoEnhance"}

	engineStats := a.EngineStats()
	require.Len(t, engineStats, len(wantEngines))
	for i, engineName := range wantEngines {
		assert.Contains(t, engineStats, engineName)
		assert.Equal(t, float64(i+1), engineStats[engineName].Busy)
		assert.Equal(t, "%", engineStats[engineName].Unit)
	}

	assert.Equal(t, 1.0, a.ClientStats())
	gpu, pkg := a.PowerStats()
	assert.Equal(t, 1.0, gpu)
	assert.Equal(t, 4.0, pkg)

	cancel()
	assert.NoError(t, <-errCh)
}

func TestAggregator_Reset(t *testing.T) {
	var a Aggregator
	a.logger = slog.New(slog.NewTextHandler(io.Discard, nil))

	assert.Len(t, a.stats, 0)
	a.Reset()
	assert.Len(t, a.stats, 0)
	var stat igt.GPUStats
	for i := range 5 {
		stat.Power.GPU = float64(i)
		a.add(stat)
	}
	assert.Len(t, a.stats, 5)
	a.Reset()
	require.Len(t, a.stats, 1)
	assert.Equal(t, 4.0, a.stats[0].Power.GPU)

}

func TestEngineStats_LogValue(t *testing.T) {
	stats := EngineStats{
		"FOO": {},
		"BAR": {},
	}
	assert.Equal(t, "BAR,FOO", stats.LogValue().String())
	clear(stats)
	assert.Equal(t, "", stats.LogValue().String())
}

func Test_medianFunc(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		want   float64
	}{
		{"odd number of values", []float64{0, 1, 2, 3, 4}, 2},
		{"even number of values", []float64{0, 1, 2, 3, 4, 5}, 2.5},
		{"empty slice", nil, 0.0},
		{"handle duplicates", []float64{1, 1, 1, 2}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slices.Reverse(tt.values)
			assert.Equal(t, tt.want, medianFunc(tt.values, func(f float64) float64 { return f }))
		})
	}
}

// Benchmark_medianFunc/current-16           295650              3847 ns/op            8192 B/op          1 allocs/op
func Benchmark_medianFunc(b *testing.B) {
	const count = 1001
	values := make([]float64, count)
	for i := range values {
		values[i] = float64(i)
	}
	slices.Reverse(values)
	want := float64(count / 2)
	b.ResetTimer()
	b.Run("current", func(b *testing.B) {
		for range b.N {
			if value := medianFunc(values, func(f float64) float64 { return f }); value != want {
				b.Fatalf("expected %f, got %f", want, value)
			}
		}
	})
}

// Current:
// BenchmarkAggregator_EngineStats-16          4962            234863 ns/op          457877 B/op         58 allocs/op
// After:
// BenchmarkAggregator_EngineStats-16          4868            234483 ns/op          385557 B/op         26 allocs/op
func BenchmarkAggregator_EngineStats(b *testing.B) {
	const count = 1001
	var engineNames = []string{"Render/3D", "Blitter", "Video", "VideoEnhance"}
	var a Aggregator
	a.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	for range count {
		var stats igt.GPUStats
		stats.Engines = make(map[string]igt.EngineStats, len(engineNames))
		for _, engine := range engineNames {
			stats.Engines[engine] = igt.EngineStats{}
		}
		a.add(stats)
	}
	b.ResetTimer()
	for range b.N {
		if stats := a.EngineStats(); len(stats) != len(engineNames) {
			b.Fatalf("expected %d engines, got %d", len(engineNames), len(stats))
		}
	}
}
