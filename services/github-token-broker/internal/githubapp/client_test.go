package githubapp

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateInstallationToken(t *testing.T) {
	now := time.Date(2026, 4, 21, 23, 0, 0, 0, time.UTC)
	httpClient := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodPost, req.Method)
		assert.Equal(t, "https://api.github.test/app/installations/123/access_tokens", req.URL.String())
		assert.Equal(t, "application/vnd.github+json", req.Header.Get("Accept"))
		assert.Equal(t, "github-token-broker", req.Header.Get("User-Agent"))

		authorization := req.Header.Get("Authorization")
		require.True(t, strings.HasPrefix(authorization, "Bearer "))
		assertJWTClaims(t, strings.TrimPrefix(authorization, "Bearer "), map[string]any{
			"iss": "Iv1.client",
			"iat": float64(now.Add(-60 * time.Second).Unix()),
			"exp": float64(now.Add(jwtLifetime).Unix()),
		})

		var body struct {
			Repositories []string          `json:"repositories"`
			Permissions  map[string]string `json:"permissions"`
		}
		require.NoError(t, json.NewDecoder(req.Body).Decode(&body))
		assert.Equal(t, []string{"secrets"}, body.Repositories)
		assert.Equal(t, map[string]string{"contents": "read"}, body.Permissions)

		return jsonResponse(http.StatusCreated, `{"token":"ghs_test","expires_at":"2026-04-22T00:00:00Z"}`), nil
	})
	client, err := NewClient(httpClient, "https://api.github.test", func() time.Time { return now })
	require.NoError(t, err)

	token, err := client.CreateInstallationToken(context.Background(), AppConfig{
		ClientID:       "Iv1.client",
		InstallationID: "123",
		PrivateKeyPEM:  testPrivateKeyPEM(t),
	}, Target{
		Owner:      "GilmanLab",
		Repository: "secrets",
		Permissions: map[string]string{
			"contents": "read",
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "ghs_test", token.Token)
	assert.Equal(t, time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC), token.ExpiresAt)
}

func TestCreateInstallationTokenSurfacesGitHubErrorsWithoutPrivateKey(t *testing.T) {
	privateKey := testPrivateKeyPEM(t)
	httpClient := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusForbidden, `{"message":"bad credentials"}`), nil
	})
	client, err := NewClient(httpClient, "https://api.github.test", nil)
	require.NoError(t, err)

	_, err = client.CreateInstallationToken(context.Background(), AppConfig{
		ClientID:       "Iv1.client",
		InstallationID: "123",
		PrivateKeyPEM:  privateKey,
	}, Target{
		Repository: "secrets",
		Permissions: map[string]string{
			"contents": "read",
		},
	})

	require.Error(t, err)
	assert.ErrorContains(t, err, "status 403")
	assert.NotContains(t, err.Error(), privateKey)
}

func TestNewClientRejectsRelativeBaseURL(t *testing.T) {
	_, err := NewClient(nil, "/relative", nil)

	require.Error(t, err)
	assert.ErrorContains(t, err, "absolute")
}

func assertJWTClaims(t *testing.T, token string, expected map[string]any) {
	t.Helper()

	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)

	var claims map[string]any
	require.NoError(t, json.Unmarshal(payload, &claims))

	for key, value := range expected {
		assert.Equal(t, value, claims[key])
	}
}

func testPrivateKeyPEM(t *testing.T) string {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	encoded := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	require.NotNil(t, encoded)

	return string(encoded)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}
