package collector

import (
	igt "github.com/clambin/intel-gpu-exporter/pkg/intel-gpu-top"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	engineMetric = prometheus.NewDesc(
		prometheus.BuildFQName("gpumon", "engine", "usage"),
		"",
		[]string{"engine", "attrib"},
		nil,
	)
	powerMetric = prometheus.NewDesc(
		prometheus.BuildFQName("gpumon", "", "power"),
		"",
		[]string{"type"},
		nil,
	)
	clientMetric = prometheus.NewDesc(
		prometheus.BuildFQName("gpumon", "clients", "count"),
		"",
		nil,
		nil,
	)
)

type StatFetcher interface {
	EngineStats() map[string]igt.EngineStats
	PowerStats() (float64, float64)
	ClientStats() float64
	Reset()
}

type Collector struct {
	StatFetcher
}

func (c Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- engineMetric
	ch <- powerMetric
	ch <- clientMetric
}

func (c Collector) Collect(ch chan<- prometheus.Metric) {
	for engine, engineStats := range c.EngineStats() {
		ch <- prometheus.MustNewConstMetric(engineMetric, prometheus.GaugeValue, engineStats.Busy, engine, "busy")
		ch <- prometheus.MustNewConstMetric(engineMetric, prometheus.GaugeValue, engineStats.Sema, engine, "sema")
		ch <- prometheus.MustNewConstMetric(engineMetric, prometheus.GaugeValue, engineStats.Wait, engine, "wait")
	}
	gpuPower, packagePower := c.StatFetcher.PowerStats()
	ch <- prometheus.MustNewConstMetric(powerMetric, prometheus.GaugeValue, packagePower, "pkg")
	ch <- prometheus.MustNewConstMetric(powerMetric, prometheus.GaugeValue, gpuPower, "gpu")
	ch <- prometheus.MustNewConstMetric(clientMetric, prometheus.GaugeValue, c.ClientStats())
	c.StatFetcher.Reset()
}
