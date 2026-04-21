package config

import (
	"flag"
	"fmt"
	"os"
	"time"
)

// ServeConfig is the runtime configuration for dns-mirror serve mode.
type ServeConfig struct {
	// AWSRegion is the AWS region used for SDK configuration.
	AWSRegion string
	// HostedZoneID is the Route 53 hosted zone ID to mirror.
	HostedZoneID string
	// OutputPath is the on-disk path for the rendered zonefile snapshot.
	OutputPath string
	// SyncInterval is the cadence at which Route 53 is resynced.
	SyncInterval time.Duration
	// ListenAddr is the HTTP listen address for health and zonefile serving.
	ListenAddr string
	// LogLevel is the slog level string.
	LogLevel string
	// Once performs a single sync and then exits.
	Once bool
}

// FetchConfig is the runtime configuration for dns-mirror fetch mode.
type FetchConfig struct {
	// SourceURL is the HTTP URL that serves the current zonefile.
	SourceURL string
	// OutputPath is the on-disk path for the fetched zonefile snapshot.
	OutputPath string
	// Timeout bounds the fetch operation.
	Timeout time.Duration
	// LogLevel is the slog level string.
	LogLevel string
}

// Load parses serve-mode flags and environment variables into a ServeConfig.
func Load(args []string) (ServeConfig, error) {
	return LoadServe(args)
}

// LoadServe parses serve-mode flags and environment variables into a ServeConfig.
func LoadServe(args []string) (ServeConfig, error) {
	flags := flag.NewFlagSet("dns-mirror", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	cfg := ServeConfig{}
	flags.BoolVar(&cfg.Once, "once", false, "sync the zone once and exit")

	if err := flags.Parse(args); err != nil {
		return ServeConfig{}, err
	}

	cfg.AWSRegion = os.Getenv("AWS_REGION")
	cfg.HostedZoneID = os.Getenv("DNS_MIRROR_HOSTED_ZONE_ID")
	cfg.OutputPath = os.Getenv("DNS_MIRROR_OUTPUT_PATH")
	cfg.ListenAddr = envOrDefault("DNS_MIRROR_LISTEN_ADDR", ":8080")
	cfg.LogLevel = envOrDefault("DNS_MIRROR_LOG_LEVEL", "info")

	syncInterval := envOrDefault("DNS_MIRROR_SYNC_INTERVAL", "1m")
	parsedSyncInterval, err := time.ParseDuration(syncInterval)
	if err != nil {
		return ServeConfig{}, fmt.Errorf("parse DNS_MIRROR_SYNC_INTERVAL: %w", err)
	}

	cfg.SyncInterval = parsedSyncInterval

	if cfg.AWSRegion == "" {
		return ServeConfig{}, fmt.Errorf("AWS_REGION is required")
	}

	if cfg.HostedZoneID == "" {
		return ServeConfig{}, fmt.Errorf("DNS_MIRROR_HOSTED_ZONE_ID is required")
	}

	if cfg.OutputPath == "" {
		return ServeConfig{}, fmt.Errorf("DNS_MIRROR_OUTPUT_PATH is required")
	}

	if cfg.SyncInterval <= 0 {
		return ServeConfig{}, fmt.Errorf("DNS_MIRROR_SYNC_INTERVAL must be greater than zero")
	}

	return cfg, nil
}

// LoadFetch parses fetch-mode flags into a FetchConfig.
func LoadFetch(args []string) (FetchConfig, error) {
	flags := flag.NewFlagSet("dns-mirror fetch", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	cfg := FetchConfig{}
	flags.StringVar(&cfg.SourceURL, "source-url", "", "HTTP URL for the source zonefile")
	flags.StringVar(&cfg.OutputPath, "output-path", "", "path where the fetched zonefile is written")
	flags.DurationVar(&cfg.Timeout, "timeout", 15*time.Second, "HTTP fetch timeout")
	flags.StringVar(&cfg.LogLevel, "log-level", envOrDefault("DNS_MIRROR_LOG_LEVEL", "info"), "log level")

	if err := flags.Parse(args); err != nil {
		return FetchConfig{}, err
	}

	if cfg.SourceURL == "" {
		return FetchConfig{}, fmt.Errorf("--source-url is required")
	}

	if cfg.OutputPath == "" {
		return FetchConfig{}, fmt.Errorf("--output-path is required")
	}

	if cfg.Timeout <= 0 {
		return FetchConfig{}, fmt.Errorf("--timeout must be greater than zero")
	}

	return cfg, nil
}

func envOrDefault(key string, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return defaultValue
}
