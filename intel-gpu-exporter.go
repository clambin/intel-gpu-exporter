package main

import (
	"context"
	"errors"
	"flag"
	"github.com/clambin/intel-gpu-exporter/internal/collector"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	debug    = flag.Bool("debug", false, "Enable debug logging")
	addr     = flag.String("addr", ":9090", "Prometheus metrics listener address")
	interval = flag.Duration("interval", time.Second, "Interval to collect statistics")
)

func main() {
	flag.Parse()

	var handlerOpts slog.HandlerOptions
	if *debug {
		handlerOpts.Level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &handlerOpts))

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		if err := http.ListenAndServe(*addr, nil); !errors.Is(err, http.ErrServerClosed) {
			logger.Error("failed to start metrics server", "err", err)
			os.Exit(1)
		}
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := collector.Run(ctx, prometheus.DefaultRegisterer, *interval, logger); err != nil {
		logger.Error("collector failed to start", "err", err)
		os.Exit(1)
	}
}
