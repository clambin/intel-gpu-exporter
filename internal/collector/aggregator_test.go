package collector

import (
	"log/slog"
	"strings"
	"testing"
	"time"

	igt "github.com/clambin/intel-gpu-exporter/intel-gpu-top"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAggregator(t *testing.T) {
	const payloadCount = 4
	fake := fakeRunner{interval: time.Millisecond}
	r, _ := fake.Start(t.Context(), nil)
	var a Aggregator
	a.logger = slog.New(slog.DiscardHandler)
	go func() { assert.NoError(t, a.Read(r)) }()

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
}

func TestAggregator_Reset(t *testing.T) {
	var a Aggregator
	a.logger = slog.New(slog.DiscardHandler)

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
	assert.Empty(t, a.stats)
}

func TestAggregator_Collect(t *testing.T) {
	fake := fakeRunner{interval: time.Millisecond}
	r, _ := fake.Start(t.Context(), nil)
	var a Aggregator
	a.logger = slog.New(slog.DiscardHandler)
	go func() { assert.NoError(t, a.Read(r)) }()

	// wait for the aggregator to read in the data
	assert.Eventually(t, func() bool { return a.len() > 0 }, time.Second, time.Millisecond)

	assert.NoError(t, testutil.CollectAndCompare(&a, strings.NewReader(`
# HELP gpumon_clients_count Number of active clients
# TYPE gpumon_clients_count gauge
gpumon_clients_count 1

# HELP gpumon_engine_usage Usage statistics for the different GPU engines
# TYPE gpumon_engine_usage gauge
gpumon_engine_usage{attrib="busy",engine="Blitter"} 2
gpumon_engine_usage{attrib="busy",engine="Render/3D"} 1
gpumon_engine_usage{attrib="busy",engine="Video"} 3
gpumon_engine_usage{attrib="busy",engine="VideoEnhance"} 4
gpumon_engine_usage{attrib="sema",engine="Blitter"} 0
gpumon_engine_usage{attrib="sema",engine="Render/3D"} 0
gpumon_engine_usage{attrib="sema",engine="Video"} 0
gpumon_engine_usage{attrib="sema",engine="VideoEnhance"} 0
gpumon_engine_usage{attrib="wait",engine="Blitter"} 0
gpumon_engine_usage{attrib="wait",engine="Render/3D"} 0
gpumon_engine_usage{attrib="wait",engine="Video"} 0
gpumon_engine_usage{attrib="wait",engine="VideoEnhance"} 0

# HELP gpumon_power Power consumption by type
# TYPE gpumon_power gauge
gpumon_power{type="gpu"} 1
gpumon_power{type="pkg"} 4
`)))
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

// Current:
// BenchmarkAggregator_EngineStats-10    	    7832	    152706 ns/op	  262690 B/op	      19 allocs/op
func BenchmarkAggregator_EngineStats(b *testing.B) {
	a := Aggregator{logger: slog.New(slog.DiscardHandler)}
	var engineNames = []string{"Render/3D", "Blitter", "Video", "VideoEnhance"}
	const count = 1001
	for range count {
		var stats igt.GPUStats
		stats.Engines = make(map[string]igt.EngineStats, len(engineNames))
		for _, engine := range engineNames {
			stats.Engines[engine] = igt.EngineStats{}
		}
		a.add(stats)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		if stats := a.EngineStats(); len(stats) != len(engineNames) {
			b.Fatalf("expected %d engines, got %d", len(engineNames), len(stats))
		}
	}
}
