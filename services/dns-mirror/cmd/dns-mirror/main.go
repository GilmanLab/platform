package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/GilmanLab/platform/services/dns-mirror/internal/config"
	"github.com/GilmanLab/platform/services/dns-mirror/internal/httpapi"
	"github.com/GilmanLab/platform/services/dns-mirror/internal/mirror"
	"github.com/GilmanLab/platform/services/dns-mirror/internal/route53source"
	"github.com/GilmanLab/platform/services/dns-mirror/internal/snapshot"
	"github.com/GilmanLab/platform/services/dns-mirror/internal/zonefile"
)

func main() {
	os.Exit(run())
}

func run() int {
	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		slog.Error("load config", "error", err)
		return 1
	}

	logger := newLogger(cfg.LogLevel)
	source, err := route53source.New(context.Background(), cfg.AWSRegion)
	if err != nil {
		logger.Error("create route53 source", "error", err)
		return 1
	}

	service := mirror.NewService(source, zonefile.NewRenderer(), snapshot.NewStore(), logger)

	if err := service.LoadSnapshot(context.Background(), cfg.OutputPath); err != nil && !errors.Is(err, mirror.ErrSnapshotNotFound) {
		logger.Error("load existing snapshot", "path", cfg.OutputPath, "error", err)
		return 1
	}

	if cfg.Once {
		if err := service.SyncOnce(context.Background(), cfg.HostedZoneID, cfg.OutputPath); err != nil {
			logger.Error("sync zone", "error", err)
			return 1
		}

		return 0
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           httpapi.NewHandler(service),
		ReadHeaderTimeout: 5 * time.Second,
	}

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return service.Run(groupCtx, cfg.HostedZoneID, cfg.OutputPath, cfg.SyncInterval)
	})
	group.Go(func() error {
		<-groupCtx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		return server.Shutdown(shutdownCtx)
	})
	group.Go(func() error {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}

		return nil
	})

	if err := group.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("run service", "error", err)
		return 1
	}

	return 0
}

func newLogger(level string) *slog.Logger {
	var slogLevel slog.Level

	switch level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	return slog.New(slog.NewTextHandler(io.Writer(os.Stdout), &slog.HandlerOptions{
		Level: slogLevel,
	}))
}
