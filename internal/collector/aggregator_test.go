package collector

import (
	"testing"

	"codeberg.org/clambin/go-common/set"
	igt "github.com/clambin/intel-gpu-exporter/intel-gpu-top"
	"github.com/stretchr/testify/assert"
)

func TestAggregator_clientStats(t *testing.T) {
	a := aggregator{
		clients: set.New[string](),
		samples: []igt.GPUStats{
			{Clients: map[string]igt.ClientStats{"_1": {Name: "foo"}}},
		},
	}
	assert.Equal(t, clientStats{"foo": 1}, a.clientStats())
	a.flush()
	assert.Equal(t, clientStats{"foo": 0}, a.clientStats())
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

func BenchmarkAggregator_EngineStats(b *testing.B) {
	// BenchmarkAggregator_EngineStats-10    	    6993	    155878 ns/op	  262683 B/op	      18 allocs/op
	a := aggregator{clients: set.New[string]()}
	var engineNames = []string{"Render/3D", "Blitter", "Video", "VideoEnhance"}
	for range 1000 {
		var s igt.GPUStats
		s.Engines = make(map[string]igt.EngineStats, len(engineNames))
		for _, engine := range engineNames {
			s.Engines[engine] = igt.EngineStats{}
		}
		a.add(s)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		if stats := a.engineStats(); len(stats) != len(engineNames) {
			b.Fatalf("expected %d engines, got %d", len(engineNames), len(stats))
		}
	}
}

func BenchmarkAggregator_clientStats(b *testing.B) {
	// BenchmarkAggregator_clientStats-10    	  107694	     10454 ns/op	    2944 B/op	       5 allocs/op
	a := aggregator{clients: set.New[string]()}
	for range 100 {
		a.add(igt.GPUStats{
			Clients: map[string]igt.ClientStats{
				"_1": {Name: "foo"},
				"_2": {Name: "bar"},
				"_3": {Name: "baz"},
				"_4": {Name: "baz"},
				"_5": {Name: "baz"},
			},
		})
	}
	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		if stats := a.clientStats(); len(stats) != 3 {
			b.Fatalf("expected 3 clients, got %d", len(stats))
		}
	}
}
