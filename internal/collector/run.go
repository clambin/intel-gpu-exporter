package collector

import (
	"context"
	"github.com/prometheus/client_golang/prometheus"
	"log/slog"
	"time"
)

var (
	version = "change-me"
)

func Run(ctx context.Context, r prometheus.Registerer, scanInterval time.Duration, logger *slog.Logger) error {
	return runWithTopReader(ctx, r, NewTopReader(logger, scanInterval), logger)
}

func runWithTopReader(ctx context.Context, r prometheus.Registerer, reader *TopReader, logger *slog.Logger) error {
	logger.Info("intel-gpu-exporter starting", "version", version)
	defer logger.Info("intel-gpu-exporter shutting down")

	r.MustRegister(&reader.Aggregator)

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
