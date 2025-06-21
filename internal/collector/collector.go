package collector

import (
	"context"
	"log/slog"
	"time"

	"codeberg.org/clambin/go-common/flagger"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	version = "change-me"
)

type Configuration struct {
	Interval time.Duration `flagger.usage:"Interval to collect statistics"`
	Device   string        `flagger.usage:"Device to collect statistics from (-d parameter of intel_gpu_top)"`
	flagger.Log
	flagger.Prom
}

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
