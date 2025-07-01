package collector

import (
	"context"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"codeberg.org/clambin/go-common/flagger"
	"codeberg.org/clambin/go-common/gomathic"
	"codeberg.org/clambin/go-common/set"
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

func Run(ctx context.Context, r prometheus.Registerer, cfg Configuration, logger *slog.Logger) error {
	a := gpuMon{
		logger:    logger.With("component", "aggregator"),
		timeout:   cfg.Interval * 5,
		topRunner: &runner{logger: logger},
		cfg:       cfg,
	}
	return runWithAggregator(ctx, r, &a, logger)
}

func runWithAggregator(ctx context.Context, r prometheus.Registerer, a aggregator, logger *slog.Logger) error {
	logger.Info("intel-gpu-exporter starting", "version", version)
	defer logger.Info("intel-gpu-exporter shutting down")

	c := Collector{
		logger:     logger.With("component", "collector"),
		clients:    set.New[string](),
		aggregator: a,
	}
	r.MustRegister(&c)
	return a.run(ctx)
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
		[]string{"name"},
		nil,
	)
)

// An Collector collects the GPUStats received from intel_gpu_top and produces a consolidated sample to be reported to Prometheus.
// Consolidation is done by calculating the median of each attribute.
type Collector struct {
	logger     *slog.Logger
	clients    set.Set[string]
	aggregator aggregator
	lock       sync.RWMutex
}

type aggregator interface {
	collect() []igt.GPUStats
	run(ctx context.Context) error
}

// Describe implements the prometheus.Collector interface.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- engineMetric
	ch <- powerMetric
	ch <- clientMetric
}

// Collect implements the prometheus.Collector interface.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	stats := c.aggregator.collect()
	for module, power := range c.powerStats(stats) {
		ch <- prometheus.MustNewConstMetric(powerMetric, prometheus.GaugeValue, power, module)
	}
	for engine, engineStats := range c.engineStats(stats) {
		ch <- prometheus.MustNewConstMetric(engineMetric, prometheus.GaugeValue, engineStats.Busy, engine, "busy")
		ch <- prometheus.MustNewConstMetric(engineMetric, prometheus.GaugeValue, engineStats.Sema, engine, "sema")
		ch <- prometheus.MustNewConstMetric(engineMetric, prometheus.GaugeValue, engineStats.Wait, engine, "wait")
	}
	for client, count := range c.clientStats(stats) {
		ch <- prometheus.MustNewConstMetric(clientMetric, prometheus.GaugeValue, float64(count), client)
	}
}

// powerStats returns the median Power Stats for GPU & Package
func (c *Collector) powerStats(stats []igt.GPUStats) map[string]float64 {
	return map[string]float64{
		"gpu": gomathic.MedianFunc(stats, func(stats igt.GPUStats) float64 { return stats.Power.GPU }),
		"pkg": gomathic.MedianFunc(stats, func(stats igt.GPUStats) float64 { return stats.Power.Package }),
	}
}

// engineStats returns the median GPU Stats for each of the GPU's engines.
func (c *Collector) engineStats(stats []igt.GPUStats) engineStats {
	// group engine stats by engine name
	const engineCount = 4 // GPUs (typically) have 4 engines
	statsByEngine := make(map[string][]igt.EngineStats, engineCount)
	for _, stat := range stats {
		for engineName, engineStat := range stat.Engines {
			// pre-allocate so slices don't need to grow as we add stats
			if statsByEngine[engineName] == nil {
				statsByEngine[engineName] = make([]igt.EngineStats, 0, len(stats))
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
	c.logger.Debug("engine stats collected", "samples", len(stats), "engines", engineStats)
	return engineStats
}

// clientStats returns the median number of clients using the GPU.
func (c *Collector) clientStats(stats []igt.GPUStats) clientStats {
	c.lock.RLock()
	defer c.lock.RUnlock()
	// count the clients in each sample.
	// we want to report zero for clients that have stopped, otherwise we continue to report the last value to Prometheus.
	// so we prefill the list with known clients
	count := make(map[string][]int)
	for clientName := range c.clients {
		count[clientName] = make([]int, len(stats))
	}
	for i, entry := range stats {
		for _, client := range entry.Clients {
			if _, ok := count[client.Name]; !ok {
				// new client
				count[client.Name] = make([]int, len(stats))
				c.clients.Add(client.Name)
			}
			count[client.Name][i]++
		}
	}
	// tally the results.
	result := make(clientStats, len(count))
	for clientName, sessions := range count {
		result[clientName] = gomathic.Median(sessions)
	}
	c.logger.Debug("client stats collected", "samples", len(stats), "clients", result)
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
