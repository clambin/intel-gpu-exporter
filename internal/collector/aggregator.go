package collector

import (
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"codeberg.org/clambin/go-common/gomathic"
	"codeberg.org/clambin/go-common/set"
	igt "github.com/clambin/intel-gpu-exporter/intel-gpu-top"
)

type aggregator struct {
	samples    []igt.GPUStats
	clients    set.Set[string]
	lock       sync.RWMutex
	lastUpdate time.Time
}

func (a *aggregator) add(sample igt.GPUStats) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.samples = append(a.samples, sample)
	a.lastUpdate = time.Now()
}

func (a *aggregator) len() int {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return len(a.samples)
}

func (a *aggregator) lastUpdated() time.Time {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.lastUpdate
}

func (a *aggregator) flush() {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.samples = a.samples[:0]
}

// engineStats returns the median GPU Stats for each of the GPU's engines.
func (a *aggregator) engineStats() engineStats {
	a.lock.RLock()
	defer a.lock.RUnlock()
	// group engine stats by engine name
	const engineCount = 4 // GPUs (typically) have 4 engines
	statsByEngine := make(map[string][]igt.EngineStats, engineCount)
	for _, stat := range a.samples {
		for engineName, engineStat := range stat.Engines {
			// pre-allocate so slices don't need to grow as we add stats
			if statsByEngine[engineName] == nil {
				statsByEngine[engineName] = make([]igt.EngineStats, 0, len(a.samples))
			}
			statsByEngine[engineName] = append(statsByEngine[engineName], engineStat)
		}
	}

	// for each engine, aggregate its stats
	engineStats := make(engineStats, len(statsByEngine))
	for engine, stats := range statsByEngine {
		engineStats[engine] = igt.EngineStats{
			Busy: gomathic.MedianFunc(stats, func(stats igt.EngineStats) float64 { return stats.Busy }),
			Sema: gomathic.MedianFunc(stats, func(stats igt.EngineStats) float64 { return stats.Sema }),
			Wait: gomathic.MedianFunc(stats, func(stats igt.EngineStats) float64 { return stats.Wait }),
			Unit: stats[0].Unit,
		}
	}
	return engineStats
}

// powerStats returns the median Power Stats for GPU & Package
func (a *aggregator) powerStats() map[string]float64 {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return map[string]float64{
		"gpu": gomathic.MedianFunc(a.samples, func(stats igt.GPUStats) float64 { return stats.Power.GPU }),
		"pkg": gomathic.MedianFunc(a.samples, func(stats igt.GPUStats) float64 { return stats.Power.Package }),
	}
}

// clientStats returns the median number of clients using the GPU.
func (a *aggregator) clientStats() clientStats {
	a.lock.RLock()
	defer a.lock.RUnlock()
	// count the clients in each sample.
	// we want to report zero for clients that have stopped, otherwise we continue to report the last value to Prometheus.
	// so we prefill the list with known clients
	count := make(map[string][]int)
	for clientName := range a.clients {
		count[clientName] = make([]int, len(a.samples))
	}
	for i, entry := range a.samples {
		for _, client := range entry.Clients {
			if _, ok := count[client.Name]; !ok {
				// new client
				count[client.Name] = make([]int, len(a.samples))
				a.clients.Add(client.Name)
			}
			count[client.Name][i]++
		}
	}
	// tally the results.
	result := make(clientStats, len(count))
	for clientName, sessions := range count {
		result[clientName] = gomathic.Median(sessions)
	}
	return result
}

var _ slog.LogValuer = engineStats{}

type engineStats map[string]igt.EngineStats

func (e engineStats) LogValue() slog.Value {
	engineNames := make([]string, 0, len(e))
	for engineName := range e {
		engineNames = append(engineNames, engineName)
	}
	slices.Sort(engineNames)
	return slog.StringValue(strings.Join(engineNames, ","))
}

var _ slog.LogValuer = clientStats{}

type clientStats map[string]int

func (c clientStats) LogValue() slog.Value {
	clientNames := slices.Collect(maps.Keys(c))
	slices.Sort(clientNames)
	return slog.StringValue(strings.Join(clientNames, ","))
}
