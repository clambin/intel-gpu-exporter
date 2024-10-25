package collector

import (
	igt "github.com/clambin/intel-gpu-exporter/pkg/intel-gpu-top"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestCollector(t *testing.T) {
	f := fakeTop{
		stats: igt.GPUStats{
			Engines: map[string]igt.EngineStats{
				"VCS": {Busy: 95, Sema: 1, Wait: 10},
			},
		},
	}
	c := Collector{StatFetcher: f}

	assert.NoError(t, testutil.CollectAndCompare(&c, strings.NewReader(`
# HELP gpumon_clients_count 
# TYPE gpumon_clients_count gauge
gpumon_clients_count 5

# HELP gpumon_power 
# TYPE gpumon_power gauge
gpumon_power{type="gpu"} 10
gpumon_power{type="pkg"} 20

# HELP gpumon_engine_usage 
# TYPE gpumon_engine_usage gauge
gpumon_engine_usage{attrib="busy",engine="VCS"} 95
gpumon_engine_usage{attrib="sema",engine="VCS"} 1
gpumon_engine_usage{attrib="wait",engine="VCS"} 10
`)))
}

var _ StatFetcher = &fakeTop{}

type fakeTop struct {
	stats igt.GPUStats
}

func (f fakeTop) EngineStats() map[string]igt.EngineStats {
	return f.stats.Engines
}

func (f fakeTop) PowerStats() (float64, float64) {
	return 10, 20
}

func (f fakeTop) ClientStats() float64 {
	return 5
}

func (f fakeTop) Reset() {}
