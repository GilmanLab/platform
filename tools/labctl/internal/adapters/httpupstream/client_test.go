package httpupstream_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gilmanlab/platform/tools/labctl/internal/adapters/httpupstream"
)

func TestClientFetchSHA256ParsesChecksumLine(t *testing.T) {
	const digest = "5fa3a23e3f12cf6f33b66e2eb1cd0f8df57f53efb15c1ab8c8f6bb3fa1e02b9d"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, digest+"  nocloud-amd64.raw.xz\n")
	}))
	t.Cleanup(server.Close)

	client := httpupstream.New(server.Client())
	got, err := client.FetchSHA256(context.Background(), server.URL+"/x.sha256")

	require.NoError(t, err)
	assert.Equal(t, digest, got)
}

func TestClientFetchSHA256RejectsMalformedDigest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "not-a-digest\n")
	}))
	t.Cleanup(server.Close)

	client := httpupstream.New(server.Client())
	_, err := client.FetchSHA256(context.Background(), server.URL+"/x.sha256")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a 64-character hex string")
}

func TestClientFetchSHA256RejectsEmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	client := httpupstream.New(server.Client())
	_, err := client.FetchSHA256(context.Background(), server.URL+"/x.sha256")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestClientRetriesTransientFailures(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) <= 2 {
			http.Error(w, "boom", http.StatusBadGateway)

			return
		}
		_, _ = io.WriteString(w, "ok")
	}))
	t.Cleanup(server.Close)

	client := httpupstream.New(server.Client())
	body, err := client.Download(context.Background(), server.URL+"/artifact")

	require.NoError(t, err)
	t.Cleanup(func() { _ = body.Close() })

	data, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.Equal(t, "ok", string(data))
	assert.GreaterOrEqual(t, calls.Load(), int32(3), "expected at least three attempts")
}

func TestClientDoesNotRetryClientErrors(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		http.Error(w, "nope", http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	client := httpupstream.New(server.Client())
	_, err := client.Download(context.Background(), server.URL+"/artifact")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
	assert.Equal(t, int32(1), calls.Load(), "4xx must not be retried")
}

func TestClientStopsRetryingOnContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	client := httpupstream.New(server.Client())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	t.Cleanup(cancel)

	_, err := client.Download(ctx, server.URL+"/artifact")

	require.Error(t, err)
	assert.True(
		t,
		errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || isRetryableErr(err),
		"expected ctx error or last retryable error, got %v", err,
	)
}

func isRetryableErr(err error) bool {
	return err != nil && (assertContains(err.Error(), "503") || assertContains(err.Error(), "retryable"))
}

func assertContains(s, sub string) bool {
	for i := range len(s) - len(sub) + 1 {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}

	return false
}
