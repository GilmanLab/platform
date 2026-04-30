package githubauth

import (
	"context"
	"errors"
	"os"
	"strings"

	appsecrets "github.com/gilmanlab/platform/tools/labctl/internal/app/secrets"
)

const (
	// EnvGitHubToken supplies a manual GitHub token for labctl secrets fetches.
	EnvGitHubToken = "LABCTL_GITHUB_TOKEN" //nolint:gosec // This is an environment variable name, not a token value.
)

// Broker mints short-lived GitHub tokens.
type Broker interface {
	Token(ctx context.Context, request appsecrets.Request) (string, error)
}

// Provider returns GitHub tokens from the environment or broker.
type Provider struct {
	lookupEnv func(string) (string, bool)
	broker    Broker
}

// NewProvider constructs a GitHub auth provider.
func NewProvider(lookupEnv func(string) (string, bool), broker Broker) Provider {
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}

	return Provider{
		lookupEnv: lookupEnv,
		broker:    broker,
	}
}

// Token returns a GitHub token without logging or persisting it.
func (p Provider) Token(ctx context.Context, request appsecrets.Request) (string, error) {
	if token, ok := p.lookupEnv(EnvGitHubToken); ok && strings.TrimSpace(token) != "" {
		return token, nil
	}

	if p.broker == nil {
		return "", errors.New("GitHub token broker is not configured")
	}

	token, err := p.broker.Token(ctx, request)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(token) == "" {
		return "", errors.New("GitHub token broker returned an empty token")
	}

	return token, nil
}
