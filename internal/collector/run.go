package collector

import (
	"context"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	version = "change-me"
)

func Run(ctx context.Context, r prometheus.Registerer, reader *TopReader, logger *slog.Logger) error {
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
