package collector

import (
	"context"
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

func TestCollector_Collect(t *testing.T) {
	l := slog.New(slog.DiscardHandler)
	g := gpuMon{
		logger:    l,
		timeout:   time.Minute,
		topRunner: &fakeRunner{interval: time.Millisecond},
		cfg:       Configuration{},
	}
	r := prometheus.NewPedanticRegistry()
	go func() {
		require.NoError(t, runWithAggregator(t.Context(), r, &g, l))
	}()

	// wait for the aggregator to read in the data
	require.Eventually(t, func() bool {
		return testutil.CollectAndCompare(r, strings.NewReader(`
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

`), "gpumon_clients_count") == nil
	}, time.Second, time.Millisecond)
}

func TestCollector_clientStats(t *testing.T) {
	c := Collector{clients: set.New[string](), logger: slog.New(slog.DiscardHandler)}
	stats := []igt.GPUStats{
		{Clients: map[string]igt.ClientStats{"_1": {Name: "foo"}}},
	}
	assert.Equal(t, clientStats{"foo": 1}, c.clientStats(stats))
	assert.Equal(t, clientStats{"foo": 0}, c.clientStats(nil))
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

func TestClientStats_LogValue(t *testing.T) {
	stats := clientStats{
		"FOO": 1,
		"BAR": 2,
	}
	assert.Equal(t, "BAR,FOO", stats.LogValue().String())
	clear(stats)
	assert.Equal(t, "", stats.LogValue().String())
}

func BenchmarkCollector_EngineStats(b *testing.B) {
	// // BenchmarkAggregator_EngineStats-10    	    7832	    152706 ns/op	  262690 B/op	      19 allocs/op
	c := Collector{logger: slog.New(slog.DiscardHandler)}
	stats := make([]igt.GPUStats, 1000)
	var engineNames = []string{"Render/3D", "Blitter", "Video", "VideoEnhance"}
	for i := range len(stats) {
		var s igt.GPUStats
		s.Engines = make(map[string]igt.EngineStats, len(engineNames))
		for _, engine := range engineNames {
			s.Engines[engine] = igt.EngineStats{}
		}
		stats[i] = s
	}
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		if stats := c.engineStats(stats); len(stats) != len(engineNames) {
			b.Fatalf("expected %d engines, got %d", len(engineNames), len(stats))
		}
	}
}

func BenchmarkCollector_clientStats(b *testing.B) {
	// BenchmarkCollector_clientStats-10    	   99816	     10549 ns/op	    2944 B/op	       5 allocs/op
	c := Collector{logger: slog.New(slog.DiscardHandler), clients: set.New[string]()}
	stats := make([]igt.GPUStats, 100)
	for i := range 100 {
		stats[i] = igt.GPUStats{
			Clients: map[string]igt.ClientStats{
				"_1": {Name: "foo"},
				"_2": {Name: "bar"},
				"_3": {Name: "baz"},
				"_4": {Name: "baz"},
				"_5": {Name: "baz"},
			},
		}
	}
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		if stats := c.clientStats(stats); len(stats) != 3 {
			b.Fatalf("expected 3 clients, got %d", len(stats))
		}
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

var _ aggregator = &fakeAggregator{}

type fakeAggregator struct {
	stats []igt.GPUStats
}

func (f fakeAggregator) collect() []igt.GPUStats {
	return f.stats
}

func (f fakeAggregator) run(_ context.Context) error {
	return nil
}
