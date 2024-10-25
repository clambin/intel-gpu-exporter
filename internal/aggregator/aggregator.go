package aggregator

import (
	"fmt"
	igt "github.com/clambin/intel-gpu-exporter/pkg/intel-gpu-top"
	"io"
	"slices"
	"sync"
)

// An Aggregator collects the GPUStats received from intel_gpu_top and produces a consolidated sample to be reported to Prometheus.
// Consolidation is done by calculating the median of each attribute.
type Aggregator struct {
	stats []igt.GPUStats
	lock  sync.RWMutex
}

func (a *Aggregator) Read(r io.Reader) error {
	for stat, err := range igt.ReadGPUStats(r) {
		if err != nil {
			return fmt.Errorf("error while reading stats: %w", err)
		}
		a.add(stat)
	}
	return nil
}

func (a *Aggregator) add(stats igt.GPUStats) {
	a.lock.Lock()
	defer a.lock.Unlock()
	// TODO: if no one is collecting, this will grow until OOM.  should we clear a certain number of measurements?
	a.stats = append(a.stats, stats)
}

func (a *Aggregator) Reset() {
	a.lock.Lock()
	defer a.lock.Unlock()
	// TODO: keep the last measurement so we have something to report to the collector?
	a.stats = a.stats[:0]
}

func (a *Aggregator) PowerStats() (float64, float64) {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return medianFunc(a.stats, func(stats igt.GPUStats) float64 { return stats.Power.GPU }),
		medianFunc(a.stats, func(stats igt.GPUStats) float64 { return stats.Power.Package })
}

func (a *Aggregator) EngineStats() map[string]igt.EngineStats {
	a.lock.RLock()
	defer a.lock.RUnlock()

	statsByEngine := a.getEngineStatsByEngineName()

	engineStats := make(map[string]igt.EngineStats, len(statsByEngine))

	for engine, stats := range statsByEngine {
		if len(stats) > 0 {
			engineStats[engine] = igt.EngineStats{
				Busy: medianFunc(stats, func(stats igt.EngineStats) float64 { return stats.Busy }),
				Sema: medianFunc(stats, func(stats igt.EngineStats) float64 { return stats.Sema }),
				Wait: medianFunc(stats, func(stats igt.EngineStats) float64 { return stats.Wait }),
				Unit: stats[0].Unit,
			}
		}
	}
	return engineStats
}

func (a *Aggregator) getEngineStatsByEngineName() map[string][]igt.EngineStats {
	stats := make(map[string][]igt.EngineStats)
	for _, stat := range a.stats {
		for engine, stat := range stat.Engines {
			stats[engine] = append(stats[engine], stat)
		}
	}
	return stats
}

func (a *Aggregator) ClientStats() float64 {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return medianFunc(a.stats, func(stats igt.GPUStats) float64 { return float64(len(stats.Clients)) })
}

func medianFunc[T any](entries []T, f func(T) float64) float64 {
	if len(entries) == 0 {
		return 0
	}
	n := len(entries)
	values := make([]float64, 0, len(entries))
	for _, entry := range entries {
		values = append(values, f(entry))
	}
	slices.Sort(values)
	// Check if the number of elements is odd or even
	if n%2 == 1 {
		// Odd length, return the middle element
		return values[n/2]
	}
	// Even length, return the average of the two middle elements
	return (values[n/2-1] + values[n/2]) / 2
}
