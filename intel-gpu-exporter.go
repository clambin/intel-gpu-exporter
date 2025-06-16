package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"codeberg.org/clambin/go-common/flagger"
	"github.com/clambin/intel-gpu-exporter/internal/collector"
	"github.com/prometheus/client_golang/prometheus"
)

type configuration struct {
	Interval time.Duration `flagger.usage:"Interval to collect statistics"`
	flagger.Log
	flagger.Prom
}

func main() {
	cfg := configuration{Interval: time.Second}
	flagger.SetFlags(flag.CommandLine, &cfg)
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger := cfg.Logger(os.Stderr, nil)
	go func() {
		if err := cfg.Serve(ctx); !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Prometheus server error", "err", err)
		}
	}()

	if err := collector.Run(ctx, prometheus.DefaultRegisterer, cfg.Interval, logger); err != nil {
		logger.Error("collector failed to start", "err", err)
		os.Exit(1)
	}
}
