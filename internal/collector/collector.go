package collector

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/clambin/go-common/flagger"
	"codeberg.org/clambin/go-common/gomathic"
	igt "github.com/clambin/intel-gpu-exporter/intel-gpu-top"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	version = "change-me"
)

type Configuration struct {
	flagger.Log
	flagger.Prom
	Device   string        `flagger.usage:"Device to collect statistics from (-d parameter of intel_gpu_top)"`
	Interval time.Duration `flagger.usage:"Interval to collect statistics"`
}

func Run(ctx context.Context, r prometheus.Registerer, reader *TopReader, logger *slog.Logger) error {
	logger.Info("intel-gpu-exporter starting", "version", version)
	defer logger.Info("intel-gpu-exporter shutting down")

	r.MustRegister(&reader.Collector)

	errCh := make(chan error)
	go func() {
		errCh <- reader.Run(ctx)
	}()

	logger.Debug("collector is running")
	defer logger.Debug("collector is shutting down")

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return nil
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

var (
	engineMetric = prometheus.NewDesc(
		prometheus.BuildFQName("gpumon", "engine", "usage"),
		"Usage statistics for the different GPU engines",
		[]string{"engine", "attrib"},
		nil,
	)
	powerMetric = prometheus.NewDesc(
		prometheus.BuildFQName("gpumon", "", "power"),
		"Power consumption by type",
		[]string{"type"},
		nil,
	)
	clientMetric = prometheus.NewDesc(
		prometheus.BuildFQName("gpumon", "clients", "count"),
		"Number of active clients",
		nil,
		nil,
	)
)

// An Collector collects the GPUStats received from intel_gpu_top and produces a consolidated sample to be reported to Prometheus.
// Consolidation is done by calculating the median of each attribute.
type Collector struct {
	lastUpdated atomic.Value
	logger      *slog.Logger
	stats       []igt.GPUStats
	lock        sync.RWMutex
}

// Describe implements the prometheus.Collector interface.
func (a *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- engineMetric
	ch <- powerMetric
	ch <- clientMetric
}

// Collect implements the prometheus.Collector interface.
func (a *Collector) Collect(ch chan<- prometheus.Metric) {
	for engine, engineStats := range a.engineStats() {
		ch <- prometheus.MustNewConstMetric(engineMetric, prometheus.GaugeValue, engineStats.Busy, engine, "busy")
		ch <- prometheus.MustNewConstMetric(engineMetric, prometheus.GaugeValue, engineStats.Sema, engine, "sema")
		ch <- prometheus.MustNewConstMetric(engineMetric, prometheus.GaugeValue, engineStats.Wait, engine, "wait")
	}
	for module, power := range a.powerStats() {
		ch <- prometheus.MustNewConstMetric(powerMetric, prometheus.GaugeValue, power, module)
	}
	ch <- prometheus.MustNewConstMetric(clientMetric, prometheus.GaugeValue, a.clientStats())
	a.reset()
}

// read reads in all GPU stats from an io.Reader and adds them to the Collector.
func (a *Collector) read(r io.Reader) error {
	a.logger.Debug("reading from new stream")
	defer a.logger.Debug("stream closed")
	for stat, err := range igt.ReadGPUStats(r) {
		if err != nil {
			return fmt.Errorf("error while reading stats: %w", err)
		}
		a.add(stat)
		a.lastUpdated.Store(time.Now())
		a.logger.Debug("found stats", "stat", stat)
	}
	return nil
}

// lastUpdate returns the timestamp when data was last received. Returns false if no data has been received yet.
func (a *Collector) lastUpdate() (time.Time, bool) {
	last := a.lastUpdated.Load()
	if last == nil {
		return time.Time{}, false
	}
	return last.(time.Time), true
}

func (a *Collector) add(stats igt.GPUStats) {
	a.lock.Lock()
	defer a.lock.Unlock()
	// TODO: if no one is collecting, this will grow until OOM.  should we clear a certain number of measurements?
	a.stats = append(a.stats, stats)
}

func (a *Collector) len() int {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return len(a.stats)
}

// reset clears all received GPU stats.
func (a *Collector) reset() {
	a.lock.Lock()
	defer a.lock.Unlock()
	if len(a.stats) > 0 {
		a.stats = a.stats[:0]
	}
}

// powerStats returns the median Power Stats for GPU & Package
func (a *Collector) powerStats() map[string]float64 {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return map[string]float64{
		"gpu": gomathic.MedianFunc(a.stats, func(stats igt.GPUStats) float64 { return stats.Power.GPU }),
		"pkg": gomathic.MedianFunc(a.stats, func(stats igt.GPUStats) float64 { return stats.Power.Package }),
	}
}

// engineStats returns the median GPU Stats for each of the GPU's engines.
func (a *Collector) engineStats() engineStats {
	a.lock.RLock()
	defer a.lock.RUnlock()

	// group engine stats by engine name
	const engineCount = 4 // GPUs (typically) have 4 engines
	statsByEngine := make(map[string][]igt.EngineStats, engineCount)
	for _, stat := range a.stats {
		for engineName, engineStat := range stat.Engines {
			// pre-allocate so slices don't need to grow as we add stats
			if statsByEngine[engineName] == nil {
				statsByEngine[engineName] = make([]igt.EngineStats, 0, len(a.stats))
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
	a.logger.Debug("engine stats collected", "samples", len(a.stats), "engines", engineStats)
	return engineStats
}

// clientStats returns the median number of clients using the GPU.
func (a *Collector) clientStats() float64 {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return gomathic.MedianFunc(a.stats, func(stats igt.GPUStats) float64 { return float64(len(stats.Clients)) })
}

var _ slog.LogValuer = engineStats{}

type engineStats map[string]igt.EngineStats

func (e engineStats) LogValue() slog.Value {
	engineNames := make([]string, 0, len(e))
	for engineName := range e {
		engineNames = append(engineNames, engineName)
	}
	sort.Strings(engineNames)
	return slog.StringValue(strings.Join(engineNames, ","))
}
