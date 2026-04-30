package secretslocal

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appsecrets "github.com/gilmanlab/platform/tools/labctl/internal/app/secrets"
)

func TestSourceFetchEncrypted(t *testing.T) {
	repoDir := t.TempDir()
	writeSecret(t, repoDir, "network/vyos/router.sops.yaml", "encrypted: true\n")

	tests := []struct {
		name      string
		request   appsecrets.Request
		lookupEnv func(string) (string, bool)
		wantError string
	}{
		{
			name:    "uses explicit repo directory",
			request: appsecrets.Request{Path: "network/vyos/router.sops.yaml", LocalRepoDir: repoDir},
		},
		{
			name:    "uses GLAB_SECRETS_DIR",
			request: appsecrets.Request{Path: "network/vyos/router.sops.yaml"},
			lookupEnv: func(key string) (string, bool) {
				return map[string]string{EnvSecretsDir: repoDir}[key], key == EnvSecretsDir
			},
		},
		{
			name:      "requires local configuration",
			request:   appsecrets.Request{Path: "network/vyos/router.sops.yaml"},
			lookupEnv: func(string) (string, bool) { return "", false },
			wantError: "not configured",
		},
		{
			name:      "propagates missing file",
			request:   appsecrets.Request{Path: "missing.sops.yaml", LocalRepoDir: repoDir},
			wantError: "read local encrypted secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := NewSource(tt.lookupEnv)

			data, err := source.FetchEncrypted(context.Background(), tt.request)

			if tt.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, []byte("encrypted: true\n"), data)
		})
	}
}

func TestSourceConfigured(t *testing.T) {
	source := NewSource(func(key string) (string, bool) {
		return map[string]string{EnvSecretsDir: "/secrets"}[key], key == EnvSecretsDir
	})

	assert.True(t, source.Configured(appsecrets.Request{}))
	assert.True(t, source.Configured(appsecrets.Request{LocalRepoDir: "/explicit"}))
	assert.False(t, NewSource(func(string) (string, bool) { return "", false }).Configured(appsecrets.Request{}))
}

func writeSecret(t *testing.T, repoDir string, secretPath string, contents string) {
	t.Helper()

	fullPath := filepath.Join(repoDir, filepath.FromSlash(secretPath))
	require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
	require.NoError(t, os.WriteFile(fullPath, []byte(contents), 0o600))
}
