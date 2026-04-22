package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("AWS_REGION", "us-west-2")

	cfg, err := Load()

	require.NoError(t, err)
	assert.Equal(t, "us-west-2", cfg.AWSRegion)
	assert.Equal(t, defaultClientIDParameter, cfg.ClientIDParameter)
	assert.Equal(t, defaultInstallationIDParameter, cfg.InstallationIDParameter)
	assert.Equal(t, defaultPrivateKeyParameter, cfg.PrivateKeyParameter)
	assert.Equal(t, defaultGitHubAPIBaseURL, cfg.GitHubAPIBaseURL)
	assert.Equal(t, "GilmanLab", cfg.RepositoryOwner)
	assert.Equal(t, "secrets", cfg.RepositoryName)
}

func TestLoadRejectsMissingRegion(t *testing.T) {
	t.Setenv("AWS_REGION", "")

	_, err := Load()

	require.Error(t, err)
	assert.ErrorContains(t, err, "AWS_REGION is required")
}

func TestLoadRejectsRepositoryOverride(t *testing.T) {
	t.Setenv("AWS_REGION", "us-west-2")
	t.Setenv("GITHUB_TOKEN_BROKER_REPOSITORY_NAME", "other")

	_, err := Load()

	require.Error(t, err)
	assert.ErrorContains(t, err, "repository target must remain GilmanLab/secrets")
}

func TestLoadRejectsRelativeParameterPath(t *testing.T) {
	t.Setenv("AWS_REGION", "us-west-2")
	t.Setenv("GITHUB_TOKEN_BROKER_PRIVATE_KEY_PARAM", "glab/bootstrap/github-app/private-key-pem")

	_, err := Load()

	require.Error(t, err)
	assert.ErrorContains(t, err, "absolute SSM parameter path")
}
