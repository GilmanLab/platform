package broker

import (
	"context"
	"testing"
	"time"

	"github.com/GilmanLab/platform/services/github-token-broker/internal/githubapp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMint(t *testing.T) {
	expiresAt := time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC)
	source := &fakeAppConfigSource{
		cfg: githubapp.AppConfig{
			ClientID:       "Iv1.client",
			InstallationID: "123",
			PrivateKeyPEM:  "private-key",
		},
	}
	issuer := &fakeTokenIssuer{
		token: githubapp.InstallationToken{
			Token:     "ghs_test",
			ExpiresAt: expiresAt,
		},
	}
	service := NewService(source, issuer, githubapp.Target{
		Owner:      "GilmanLab",
		Repository: "secrets",
		Permissions: map[string]string{
			"contents": "read",
		},
	})

	response, err := service.Mint(context.Background())

	require.NoError(t, err)
	assert.Equal(t, source.cfg, issuer.app)
	assert.Equal(t, "ghs_test", response.Token)
	assert.Equal(t, expiresAt, response.ExpiresAt)
	assert.Equal(t, []string{"GilmanLab/secrets"}, response.Repositories)
	assert.Equal(t, map[string]string{"contents": "read"}, response.Permissions)
}

type fakeAppConfigSource struct {
	cfg githubapp.AppConfig
}

func (f *fakeAppConfigSource) LoadAppConfig(context.Context) (githubapp.AppConfig, error) {
	return f.cfg, nil
}

type fakeTokenIssuer struct {
	app   githubapp.AppConfig
	token githubapp.InstallationToken
}

func (f *fakeTokenIssuer) CreateInstallationToken(_ context.Context, app githubapp.AppConfig, _ githubapp.Target) (githubapp.InstallationToken, error) {
	f.app = app
	return f.token, nil
}
