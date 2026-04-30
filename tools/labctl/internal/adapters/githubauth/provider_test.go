package githubauth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appsecrets "github.com/gilmanlab/platform/tools/labctl/internal/app/secrets"
)

func TestProviderTokenUsesEnvironmentTokenBeforeBroker(t *testing.T) {
	broker := &fakeBroker{token: "ghs_broker"}
	provider := NewProvider(func(key string) (string, bool) {
		return map[string]string{EnvGitHubToken: "ghs_env"}[key], key == EnvGitHubToken
	}, broker)

	token, err := provider.Token(context.Background(), appsecrets.Request{})

	require.NoError(t, err)
	assert.Equal(t, "ghs_env", token)
	assert.Zero(t, broker.calls)
}

func TestProviderTokenUsesBrokerWhenEnvironmentTokenIsMissing(t *testing.T) {
	broker := &fakeBroker{token: "ghs_broker"}
	provider := NewProvider(func(string) (string, bool) { return "", false }, broker)

	token, err := provider.Token(context.Background(), appsecrets.Request{Path: "secret.sops.yaml"})

	require.NoError(t, err)
	assert.Equal(t, "ghs_broker", token)
	assert.Equal(t, 1, broker.calls)
}

type fakeBroker struct {
	token string
	calls int
}

func (b *fakeBroker) Token(context.Context, appsecrets.Request) (string, error) {
	b.calls++

	return b.token, nil
}
