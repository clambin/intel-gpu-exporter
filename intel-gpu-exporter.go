package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"codeberg.org/clambin/go-common/flagger"
	"github.com/clambin/intel-gpu-exporter/internal/collector"
	"github.com/prometheus/client_golang/prometheus"
)

func main() {
	var cfg collector.Configuration
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

	if err := collector.Run(ctx, prometheus.DefaultRegisterer, cfg, logger); err != nil {
		logger.Error("collector failed to start", "err", err)
		os.Exit(1)
	}
}
