package collector

import (
	"context"
	"log/slog"

	"codeberg.org/clambin/go-common/set"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	version = "change-me"
)

func Run(ctx context.Context, r prometheus.Registerer, cfg Configuration, logger *slog.Logger) error {
	return runWithRunner(ctx, r, &runner{logger: logger}, cfg, logger)
}

func runWithRunner(ctx context.Context, r prometheus.Registerer, t topRunner, cfg Configuration, logger *slog.Logger) error {
	logger.Info("intel-gpu-exporter starting", "version", version)
	defer logger.Info("intel-gpu-exporter shutting down")

	c := Collector{
		aggregator: &aggregator{clients: set.New[string]()},
		logger:     logger.With("component", "collector"),
	}
	r.MustRegister(&c)

	m := gpuMon{
		aggregator: c.aggregator,
		timeout:    cfg.Interval * 5,
		topRunner:  t,
		cfg:        cfg,
		logger:     logger.With("component", "aggregator"),
	}

	return m.run(ctx)
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

	freqMetric = prometheus.NewDesc(
		prometheus.BuildFQName("gpumon", "", "frequency"),
		"GPU frequency statistics, in MHz",
		[]string{"type"},
		nil,
	)

	imcReadMetric = prometheus.NewDesc(
		prometheus.BuildFQName("gpumon", "imc", "read"),
		"IMC read operations, in MiB/s",
		nil,
		nil,
	)

	imcWriteMetric = prometheus.NewDesc(
		prometheus.BuildFQName("gpumon", "imc", "write"),
		"IMC write operations, in MiB/s",
		nil,
		nil,
	)
)

// A Collector collects the GPUStats received from intel_gpu_top and produces a consolidated sample to be reported to Prometheus.
// Consolidation is done by calculating the median of each attribute.
type Collector struct {
	logger     *slog.Logger
	aggregator *aggregator
}

// Describe implements the prometheus.Collector interface.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- engineMetric
	ch <- powerMetric
	ch <- clientMetric
	ch <- freqMetric
	ch <- imcReadMetric
	ch <- imcWriteMetric
}

// Collect implements the prometheus.Collector interface.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	for module, power := range c.aggregator.powerStats() {
		ch <- prometheus.MustNewConstMetric(powerMetric, prometheus.GaugeValue, power, module)
	}
	for engine, engineStats := range c.aggregator.engineStats() {
		ch <- prometheus.MustNewConstMetric(engineMetric, prometheus.GaugeValue, engineStats.Busy, engine, "busy")
		ch <- prometheus.MustNewConstMetric(engineMetric, prometheus.GaugeValue, engineStats.Sema, engine, "sema")
		ch <- prometheus.MustNewConstMetric(engineMetric, prometheus.GaugeValue, engineStats.Wait, engine, "wait")
	}

	clientStats := c.aggregator.clientStats()
	for client, count := range clientStats {
		ch <- prometheus.MustNewConstMetric(clientMetric, prometheus.GaugeValue, float64(count), client)
	}
	if len(clientStats) == 0 {
		ch <- prometheus.MustNewConstMetric(clientMetric, prometheus.GaugeValue, 0, "")
	}

	requestedFreq, actualFreq := c.aggregator.freqStats()
	ch <- prometheus.MustNewConstMetric(freqMetric, prometheus.GaugeValue, requestedFreq, "requested")
	ch <- prometheus.MustNewConstMetric(freqMetric, prometheus.GaugeValue, actualFreq, "actual")

	imcRead, imcWrite := c.aggregator.bandwidthStats()
	ch <- prometheus.MustNewConstMetric(imcReadMetric, prometheus.GaugeValue, imcRead)
	ch <- prometheus.MustNewConstMetric(imcWriteMetric, prometheus.GaugeValue, imcWrite)

	c.aggregator.flush()
}
