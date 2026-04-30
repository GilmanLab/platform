package githubcontents

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appsecrets "github.com/gilmanlab/platform/tools/labctl/internal/app/secrets"
)

func TestSourceFetchEncrypted(t *testing.T) {
	tokenProvider := staticTokenProvider{token: "ghs_test"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/GilmanLab/secrets/contents/network/vyos/router.sops.yaml", r.URL.Path)
		assert.Equal(t, "master", r.URL.Query().Get("ref"))
		assert.Equal(t, acceptRawHeader, r.Header.Get("Accept"))
		assert.Equal(t, "Bearer ghs_test", r.Header.Get("Authorization"))
		assert.Equal(t, userAgent, r.Header.Get("User-Agent"))
		assert.Equal(t, apiVersion, r.Header.Get(apiVersionHeader))

		_, _ = w.Write([]byte("encrypted: true\n"))
	}))
	defer server.Close()

	source := NewSource(server.Client(), tokenProvider, server.URL)

	data, err := source.FetchEncrypted(context.Background(), appsecrets.Request{
		Path: "network/vyos/router.sops.yaml",
		Ref:  appsecrets.DefaultRef,
	})

	require.NoError(t, err)
	assert.Equal(t, []byte("encrypted: true\n"), data)
}

func TestSourceFetchEncryptedReturnsGitHubError(t *testing.T) {
	tokenProvider := staticTokenProvider{token: "ghs_test"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	source := NewSource(server.Client(), tokenProvider, server.URL)

	_, err := source.FetchEncrypted(context.Background(), appsecrets.Request{
		Path: "missing.sops.yaml",
		Ref:  appsecrets.DefaultRef,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 404")
	assert.NotContains(t, err.Error(), "ghs_test")
}

func TestSourceFetchEncryptedPropagatesTokenError(t *testing.T) {
	source := NewSource(http.DefaultClient, staticTokenProvider{err: assert.AnError}, DefaultBaseURL)

	_, err := source.FetchEncrypted(context.Background(), appsecrets.Request{Path: "secret.sops.yaml"})

	require.ErrorIs(t, err, assert.AnError)
}

type staticTokenProvider struct {
	token string
	err   error
}

func (p staticTokenProvider) Token(context.Context, appsecrets.Request) (string, error) {
	if p.err != nil {
		return "", p.err
	}

	return p.token, nil
}
