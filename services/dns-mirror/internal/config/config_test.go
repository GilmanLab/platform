package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/GilmanLab/platform/services/dns-mirror/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("AWS_REGION", "us-west-2")
	t.Setenv("DNS_MIRROR_HOSTED_ZONE_ID", "Z123")
	t.Setenv("DNS_MIRROR_OUTPUT_PATH", "/tmp/glab.zone")

	cfg, err := config.Load(nil)
	require.NoError(t, err)

	assert.Equal(t, "us-west-2", cfg.AWSRegion)
	assert.Equal(t, "Z123", cfg.HostedZoneID)
	assert.Equal(t, "/tmp/glab.zone", cfg.OutputPath)
	assert.Equal(t, time.Minute, cfg.SyncInterval)
	assert.Equal(t, ":8080", cfg.ListenAddr)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.False(t, cfg.Once)
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("AWS_REGION", "us-west-2")
	t.Setenv("DNS_MIRROR_HOSTED_ZONE_ID", "Z123")
	t.Setenv("DNS_MIRROR_OUTPUT_PATH", "/tmp/glab.zone")
	t.Setenv("DNS_MIRROR_SYNC_INTERVAL", "30s")
	t.Setenv("DNS_MIRROR_LISTEN_ADDR", "127.0.0.1:9090")
	t.Setenv("DNS_MIRROR_LOG_LEVEL", "debug")

	cfg, err := config.Load([]string{"--once"})
	require.NoError(t, err)

	assert.Equal(t, 30*time.Second, cfg.SyncInterval)
	assert.Equal(t, "127.0.0.1:9090", cfg.ListenAddr)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.True(t, cfg.Once)
}

func TestLoadRejectsMissingRequiredValues(t *testing.T) {
	_, err := config.Load(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AWS_REGION")
}

func TestLoadRejectsInvalidInterval(t *testing.T) {
	t.Setenv("AWS_REGION", "us-west-2")
	t.Setenv("DNS_MIRROR_HOSTED_ZONE_ID", "Z123")
	t.Setenv("DNS_MIRROR_OUTPUT_PATH", "/tmp/glab.zone")
	t.Setenv("DNS_MIRROR_SYNC_INTERVAL", "not-a-duration")

	_, err := config.Load(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DNS_MIRROR_SYNC_INTERVAL")
}
