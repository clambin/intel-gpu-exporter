package collector

import (
	"context"
	"log/slog"
	"time"

	"codeberg.org/clambin/go-common/flagger"
	"codeberg.org/clambin/go-common/set"
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
	return runWithRunner(ctx, r, &runner{logger: logger}, cfg, logger)
}

func runWithRunner(ctx context.Context, r prometheus.Registerer, t topRunner, cfg Configuration, logger *slog.Logger) error {
	a := aggregator{clients: set.New[string]()}
	c := Collector{
		logger:     logger.With("component", "collector"),
		aggregator: &a,
	}
	r.MustRegister(&c)

	logger.Info("intel-gpu-exporter starting", "version", version)
	defer logger.Info("intel-gpu-exporter shutting down")

	m := gpuMon{
		logger:     logger.With("component", "aggregator"),
		aggregator: &a,
		timeout:    cfg.Interval * 5,
		topRunner:  t,
		cfg:        cfg,
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
)

// An Collector collects the GPUStats received from intel_gpu_top and produces a consolidated sample to be reported to Prometheus.
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
	for client, count := range c.aggregator.clientStats() {
		ch <- prometheus.MustNewConstMetric(clientMetric, prometheus.GaugeValue, float64(count), client)
	}
	c.aggregator.flush()
}
