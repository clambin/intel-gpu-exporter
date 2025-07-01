package collector

import (
	"log/slog"
	"strings"
	"testing"
	"time"

	"codeberg.org/clambin/go-common/set"
	igt "github.com/clambin/intel-gpu-exporter/intel-gpu-top"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	//l := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	l := slog.New(slog.DiscardHandler)

	reader := newTopReader(Configuration{Interval: 100 * time.Millisecond}, l)
	reader.topRunner = &fakeRunner{interval: 100 * time.Millisecond}
	r := prometheus.NewRegistry()

	go func() {
		assert.NoError(t, runWithTopReader(t.Context(), r, reader, l))
	}()

	assert.Eventually(t, func() bool {
		n, err := testutil.GatherAndCount(r)
		return err == nil && n == 15
	}, 5*time.Second, 100*time.Millisecond)
}

func TestCollector(t *testing.T) {
	const payloadCount = 4
	fake := fakeRunner{interval: time.Millisecond}
	r, _ := fake.start(t.Context(), nil)
	var a Collector
	a.logger = slog.New(slog.DiscardHandler)
	a.clients = set.New[string]()
	go func() { assert.NoError(t, a.read(r)) }()

	// a.read works asynchronously. Wait for all data to be read.
	assert.Eventually(t, func() bool { return len(a.engineStats()) >= payloadCount }, time.Second, time.Millisecond)

	wantEngines := []string{"Render/3D", "Blitter", "Video", "VideoEnhance"}

	engineStats := a.engineStats()
	require.Len(t, engineStats, len(wantEngines))
	for i, engineName := range wantEngines {
		assert.Contains(t, engineStats, engineName)
		assert.Equal(t, float64(i+1), engineStats[engineName].Busy)
		assert.Equal(t, "%", engineStats[engineName].Unit)
	}
	assert.Equal(t, map[string]float64{"gpu": 1.0, "pkg": 4.0}, a.powerStats())
	assert.Equal(t, map[string]int{"foo": 1}, a.clientStats())
}

func TestCollector_Reset(t *testing.T) {
	var a Collector
	a.logger = slog.New(slog.DiscardHandler)

	assert.Len(t, a.stats, 0)
	a.reset()
	assert.Len(t, a.stats, 0)
	var stat igt.GPUStats
	for i := range 5 {
		stat.Power.GPU = float64(i)
		a.add(stat)
	}
	assert.Len(t, a.stats, 5)
	a.reset()
	assert.Empty(t, a.stats)
}

func TestCollector_Collect(t *testing.T) {
	fake := fakeRunner{interval: time.Millisecond}
	r, _ := fake.start(t.Context(), nil)
	var a Collector
	a.logger = slog.New(slog.DiscardHandler)
	a.clients = set.New[string]()
	go func() { assert.NoError(t, a.read(r)) }()

	// wait for the aggregator to read in the data
	assert.Eventually(t, func() bool { return a.len() > 0 }, time.Second, time.Millisecond)

	assert.NoError(t, testutil.CollectAndCompare(&a, strings.NewReader(`
# HELP gpumon_clients_count Number of active clients
# TYPE gpumon_clients_count gauge
gpumon_clients_count{name="foo"} 1

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

func TestCollector_clientStats(t *testing.T) {
	c := Collector{clients: set.New[string]()}
	c.stats = []igt.GPUStats{
		{Clients: map[string]igt.ClientStats{"_1": {Name: "foo"}}},
	}
	assert.Equal(t, map[string]int{"foo": 1}, c.clientStats())
	c.reset()
	assert.Equal(t, map[string]int{"foo": 0}, c.clientStats())
}

func TestEngineStats_LogValue(t *testing.T) {
	stats := engineStats{
		"FOO": {},
		"BAR": {},
	}
	assert.Equal(t, "BAR,FOO", stats.LogValue().String())
	clear(stats)
	assert.Equal(t, "", stats.LogValue().String())
}

func BenchmarkCollector_EngineStats(b *testing.B) {
	// // BenchmarkAggregator_EngineStats-10    	    7832	    152706 ns/op	  262690 B/op	      19 allocs/op
	c := Collector{logger: slog.New(slog.DiscardHandler)}
	var engineNames = []string{"Render/3D", "Blitter", "Video", "VideoEnhance"}
	for range 1000 {
		var stats igt.GPUStats
		stats.Engines = make(map[string]igt.EngineStats, len(engineNames))
		for _, engine := range engineNames {
			stats.Engines[engine] = igt.EngineStats{}
		}
		c.add(stats)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		if stats := c.engineStats(); len(stats) != len(engineNames) {
			b.Fatalf("expected %d engines, got %d", len(engineNames), len(stats))
		}
	}
}
