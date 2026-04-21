package fetcher_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/GilmanLab/platform/services/dns-mirror/internal/fetcher"
	"github.com/GilmanLab/platform/services/dns-mirror/internal/snapshot"
)

func TestFetchWritesSuccessfulResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("glab.lol.\t900\tIN\tSOA\tns.example. hostmaster.example. 1 2 3 4 5\n"))
	}))
	t.Cleanup(server.Close)

	outputPath := filepath.Join(t.TempDir(), "glab.lol.zone")
	fetcher := fetcher.New(server.Client(), snapshot.NewStore())

	snapshot, err := fetcher.Fetch(context.Background(), server.URL, outputPath)
	require.NoError(t, err)

	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Equal(t, snapshot.Content, content)
	assert.Contains(t, string(content), "glab.lol.")
}

func TestFetchRejectsHTTPFailureWithoutReplacingExistingFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	outputPath := filepath.Join(t.TempDir(), "glab.lol.zone")
	require.NoError(t, os.WriteFile(outputPath, []byte("last good"), 0o644))

	fetcher := fetcher.New(server.Client(), snapshot.NewStore())
	_, err := fetcher.Fetch(context.Background(), server.URL, outputPath)
	require.Error(t, err)

	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Equal(t, "last good", string(content))
}

func TestFetchRejectsEmptyResponseWithoutReplacingExistingFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	t.Cleanup(server.Close)

	outputPath := filepath.Join(t.TempDir(), "glab.lol.zone")
	require.NoError(t, os.WriteFile(outputPath, []byte("last good"), 0o644))

	fetcher := fetcher.New(server.Client(), snapshot.NewStore())
	_, err := fetcher.Fetch(context.Background(), server.URL, outputPath)
	require.Error(t, err)

	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Equal(t, "last good", string(content))
}

func TestFetchReturnsClientErrors(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "glab.lol.zone")
	fetcher := fetcher.New(errorClient{}, snapshot.NewStore())

	_, err := fetcher.Fetch(context.Background(), "http://example.invalid/zonefile", outputPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetch zonefile")
}

type errorClient struct{}

func (errorClient) Do(_ *http.Request) (*http.Response, error) {
	return nil, errors.New("network unavailable")
}
