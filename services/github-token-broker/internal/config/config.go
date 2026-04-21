package config

import (
	"fmt"
	"os"
	"strings"
)

const (
	defaultClientIDParameter       = "/glab/bootstrap/github-app/client-id"
	defaultInstallationIDParameter = "/glab/bootstrap/github-app/installation-id"
	defaultPrivateKeyParameter     = "/glab/bootstrap/github-app/private-key-pem"
	defaultGitHubAPIBaseURL        = "https://api.github.com"
	defaultRepositoryOwner         = "GilmanLab"
	defaultRepositoryName          = "secrets"
	defaultLogLevel                = "info"
)

// Config is the runtime configuration for github-token-broker.
type Config struct {
	// AWSRegion is the AWS region used for SDK configuration.
	AWSRegion string
	// ClientIDParameter is the SSM parameter that stores the GitHub App client ID.
	ClientIDParameter string
	// InstallationIDParameter is the SSM parameter that stores the GitHub App installation ID.
	InstallationIDParameter string
	// PrivateKeyParameter is the SSM SecureString parameter that stores the GitHub App private key.
	PrivateKeyParameter string
	// GitHubAPIBaseURL is the GitHub API base URL.
	GitHubAPIBaseURL string
	// RepositoryOwner is the fixed GitHub owner for the token request.
	RepositoryOwner string
	// RepositoryName is the fixed GitHub repository for the token request.
	RepositoryName string
	// LogLevel is the slog level string.
	LogLevel string
}

// Load reads environment variables into a Config.
func Load() (Config, error) {
	cfg := Config{
		AWSRegion:               os.Getenv("AWS_REGION"),
		ClientIDParameter:       envOrDefault("GITHUB_TOKEN_BROKER_CLIENT_ID_PARAM", defaultClientIDParameter),
		InstallationIDParameter: envOrDefault("GITHUB_TOKEN_BROKER_INSTALLATION_ID_PARAM", defaultInstallationIDParameter),
		PrivateKeyParameter:     envOrDefault("GITHUB_TOKEN_BROKER_PRIVATE_KEY_PARAM", defaultPrivateKeyParameter),
		GitHubAPIBaseURL:        envOrDefault("GITHUB_TOKEN_BROKER_GITHUB_API_BASE_URL", defaultGitHubAPIBaseURL),
		RepositoryOwner:         envOrDefault("GITHUB_TOKEN_BROKER_REPOSITORY_OWNER", defaultRepositoryOwner),
		RepositoryName:          envOrDefault("GITHUB_TOKEN_BROKER_REPOSITORY_NAME", defaultRepositoryName),
		LogLevel:                envOrDefault("GITHUB_TOKEN_BROKER_LOG_LEVEL", defaultLogLevel),
	}

	if cfg.AWSRegion == "" {
		return Config{}, fmt.Errorf("AWS_REGION is required")
	}

	if !strings.HasPrefix(cfg.ClientIDParameter, "/") {
		return Config{}, fmt.Errorf("GITHUB_TOKEN_BROKER_CLIENT_ID_PARAM must be an absolute SSM parameter path")
	}

	if !strings.HasPrefix(cfg.InstallationIDParameter, "/") {
		return Config{}, fmt.Errorf("GITHUB_TOKEN_BROKER_INSTALLATION_ID_PARAM must be an absolute SSM parameter path")
	}

	if !strings.HasPrefix(cfg.PrivateKeyParameter, "/") {
		return Config{}, fmt.Errorf("GITHUB_TOKEN_BROKER_PRIVATE_KEY_PARAM must be an absolute SSM parameter path")
	}

	if cfg.RepositoryOwner != defaultRepositoryOwner || cfg.RepositoryName != defaultRepositoryName {
		return Config{}, fmt.Errorf("repository target must remain %s/%s", defaultRepositoryOwner, defaultRepositoryName)
	}

	if cfg.GitHubAPIBaseURL == "" {
		return Config{}, fmt.Errorf("GITHUB_TOKEN_BROKER_GITHUB_API_BASE_URL must not be empty")
	}

	return cfg, nil
}

func envOrDefault(key string, defaultValue string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}

	return defaultValue
}
