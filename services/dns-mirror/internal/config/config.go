package config

import (
	"flag"
	"fmt"
	"os"
	"time"
)

// Config is the runtime configuration for dns-mirror.
type Config struct {
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

// Load parses flags and environment variables into a Config.
func Load(args []string) (Config, error) {
	flags := flag.NewFlagSet("dns-mirror", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	cfg := Config{}
	flags.BoolVar(&cfg.Once, "once", false, "sync the zone once and exit")

	if err := flags.Parse(args); err != nil {
		return Config{}, err
	}

	cfg.AWSRegion = os.Getenv("AWS_REGION")
	cfg.HostedZoneID = os.Getenv("DNS_MIRROR_HOSTED_ZONE_ID")
	cfg.OutputPath = os.Getenv("DNS_MIRROR_OUTPUT_PATH")
	cfg.ListenAddr = envOrDefault("DNS_MIRROR_LISTEN_ADDR", ":8080")
	cfg.LogLevel = envOrDefault("DNS_MIRROR_LOG_LEVEL", "info")

	syncInterval := envOrDefault("DNS_MIRROR_SYNC_INTERVAL", "1m")
	parsedSyncInterval, err := time.ParseDuration(syncInterval)
	if err != nil {
		return Config{}, fmt.Errorf("parse DNS_MIRROR_SYNC_INTERVAL: %w", err)
	}

	cfg.SyncInterval = parsedSyncInterval

	if cfg.AWSRegion == "" {
		return Config{}, fmt.Errorf("AWS_REGION is required")
	}

	if cfg.HostedZoneID == "" {
		return Config{}, fmt.Errorf("DNS_MIRROR_HOSTED_ZONE_ID is required")
	}

	if cfg.OutputPath == "" {
		return Config{}, fmt.Errorf("DNS_MIRROR_OUTPUT_PATH is required")
	}

	if cfg.SyncInterval <= 0 {
		return Config{}, fmt.Errorf("DNS_MIRROR_SYNC_INTERVAL must be greater than zero")
	}

	return cfg, nil
}

func envOrDefault(key string, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return defaultValue
}
