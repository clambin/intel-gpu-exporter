package tmp

import (
	"fmt"
	igt "github.com/clambin/intel-gpu-exporter/pkg/intel-gpu-top"
	"io"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// An Aggregator collects the GPUStats received from intel_gpu_top and produces a consolidated sample to be reported to Prometheus.
// Consolidation is done by calculating the median of each attribute.
type Aggregator struct {
	lastUpdate atomic.Value
	logger     *slog.Logger
	stats      []igt.GPUStats
	lock       sync.RWMutex
}

func (a *Aggregator) Read(r io.Reader) error {
	a.logger.Debug("reading from new stream")
	defer a.logger.Debug("stream closed")
	for stat, err := range igt.ReadGPUStats(r) {
		if err != nil {
			return fmt.Errorf("error while reading stats: %w", err)
		}
		a.add(stat)
		a.lastUpdate.Store(time.Now())
		a.logger.Debug("found stats", "stat", stat)
	}
	return nil
}

func (a *Aggregator) LastUpdate() (time.Time, bool) {
	last := a.lastUpdate.Load()
	if last == nil {
		return time.Time{}, false
	}
	return last.(time.Time), true
}

func (a *Aggregator) add(stats igt.GPUStats) {
	a.lock.Lock()
	defer a.lock.Unlock()
	// TODO: if no one is collecting, this will grow until OOM.  should we clear a certain number of measurements?
	a.stats = append(a.stats, stats)
}

func (a *Aggregator) len() int {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return len(a.stats)
}

func (a *Aggregator) Reset() {
	a.lock.Lock()
	defer a.lock.Unlock()
	if len(a.stats) > 0 {
		a.stats = a.stats[len(a.stats)-1:]
	}
}

func (a *Aggregator) PowerStats() (float64, float64) {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return medianFunc(a.stats, func(stats igt.GPUStats) float64 { return stats.Power.GPU }),
		medianFunc(a.stats, func(stats igt.GPUStats) float64 { return stats.Power.Package })
}

func (a *Aggregator) EngineStats() EngineStats {
	a.lock.RLock()
	defer a.lock.RUnlock()

	// group engine stats by engine name
	const engineCount = 4 // GPUs (typically) have 4 engines
	statsByEngine := make(map[string][]igt.EngineStats, engineCount)
	for _, stat := range a.stats {
		for engineName, engineStat := range stat.Engines {
			// pre-allocate so slices don't need to grow as we add stats
			if statsByEngine[engineName] == nil {
				statsByEngine[engineName] = make([]igt.EngineStats, 0, len(a.stats)/engineCount)
			}
			statsByEngine[engineName] = append(statsByEngine[engineName], engineStat)
		}
	}

	// for each engine, aggregate its stats
	engineStats := make(EngineStats, len(statsByEngine))
	for engine, stats := range statsByEngine {
		engineStats[engine] = igt.EngineStats{
			Busy: medianFunc(stats, func(stats igt.EngineStats) float64 { return stats.Busy }),
			Sema: medianFunc(stats, func(stats igt.EngineStats) float64 { return stats.Sema }),
			Wait: medianFunc(stats, func(stats igt.EngineStats) float64 { return stats.Wait }),
			Unit: stats[0].Unit,
		}
	}
	a.logger.Debug("engine stats collected", "samples", len(a.stats), "engines", engineStats)
	return engineStats
}

func (a *Aggregator) ClientStats() float64 {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return medianFunc(a.stats, func(stats igt.GPUStats) float64 { return float64(len(stats.Clients)) })
}

var _ slog.LogValuer = EngineStats{}

type EngineStats map[string]igt.EngineStats

func (e EngineStats) LogValue() slog.Value {
	engineNames := make([]string, 0, len(e))
	for engineName := range e {
		engineNames = append(engineNames, engineName)
	}
	sort.Strings(engineNames)
	return slog.StringValue(strings.Join(engineNames, ","))
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
