package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/clambin/intel-gpu-exporter/internal/aggregator"
	"github.com/clambin/intel-gpu-exporter/internal/collector"
	igt "github.com/clambin/intel-gpu-exporter/pkg/intel-gpu-top"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
)

var (
	version = "change-me"
	debug   = flag.Bool("debug", false, "Enable debug logging")
	addr    = flag.String("addr", ":9090", "Prometheus metrics listener address")
)

func main() {
	flag.Parse()

	var handlerOpts slog.HandlerOptions
	if *debug {
		handlerOpts.Level = slog.LevelDebug
	}
	l := slog.New(slog.NewTextHandler(os.Stderr, &handlerOpts))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := Main(ctx, prometheus.DefaultRegisterer, l); err != nil {
		slog.Error("intel-gpu-exporter failed to run", "err", err)
		os.Exit(1)
	}
}

func Main(ctx context.Context, r prometheus.Registerer, l *slog.Logger) error {
	l.Info("intel-gpu-exporter starting", "version", version, "addr", *addr)

	cmd, stdout, err := runTop(ctx)
	if err != nil {
		return fmt.Errorf("intel_gpu_top failed to start: %w", err)
	}

	l.Debug("intel_gpu_top started", "cmd", cmd.String())

	var a aggregator.Aggregator
	go func() {
		if err := a.Read(&igt.V118toV117{Reader: stdout}); err != nil {
			l.Error("intel_gpu_top read failed", "err", err)
			//os.Exit(1)
		}
	}()

	l.Debug("reader started")

	r.MustRegister(collector.Collector{StatFetcher: &a})

	http.Handle("/metrics", promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{}))
	go func() {
		if err := http.ListenAndServe(*addr, nil); !errors.Is(err, http.ErrServerClosed) {
			l.Error("failed to start metrics server", "err", err)
		}
	}()

	l.Debug("metrics server started")

	<-ctx.Done()
	return cmd.Wait()
}

const gpuTopCommand = "intel_gpu_top -J -s 5000"

func runTop(ctx context.Context) (*exec.Cmd, io.ReadCloser, error) {
	cmdline := strings.Split(gpuTopCommand, " ")
	cmd := exec.CommandContext(ctx, cmdline[0], cmdline[1:]...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("stdout pipe failed: %w", err)
	}
	return cmd, stdout, cmd.Start()
}
