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
	"github.com/GilmanLab/platform/services/dns-mirror/internal/fetcher"
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
	command, args, err := parseCommand(os.Args[1:])
	if err != nil {
		slog.Error("parse command", "error", err)
		return 1
	}

	switch command {
	case "serve":
		return runServe(args)
	case "fetch":
		return runFetch(args)
	default:
		slog.Error("unsupported command", "command", command)
		return 1
	}
}

func parseCommand(args []string) (string, []string, error) {
	if len(args) == 0 {
		return "serve", nil, nil
	}

	switch args[0] {
	case "serve", "fetch":
		return args[0], args[1:], nil
	default:
		if len(args[0]) > 0 && args[0][0] == '-' {
			return "serve", args, nil
		}

		return "", nil, errors.New("first argument must be serve, fetch, or a serve-mode flag")
	}
}

func runFetch(args []string) int {
	cfg, err := config.LoadFetch(args)
	if err != nil {
		slog.Error("load fetch config", "error", err)
		return 1
	}

	logger := newLogger(cfg.LogLevel)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	client := &http.Client{
		Timeout: cfg.Timeout,
	}
	fetcher := fetcher.New(client, snapshot.NewStore())

	snapshot, err := fetcher.Fetch(ctx, cfg.SourceURL, cfg.OutputPath)
	if err != nil {
		logger.Error("fetch zonefile", "source_url", cfg.SourceURL, "output_path", cfg.OutputPath, "error", err)
		return 1
	}

	logger.Info("fetched zonefile", "source_url", cfg.SourceURL, "output_path", cfg.OutputPath, "bytes", len(snapshot.Content))

	return 0
}

func runServe(args []string) int {
	cfg, err := config.LoadServe(args)
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
