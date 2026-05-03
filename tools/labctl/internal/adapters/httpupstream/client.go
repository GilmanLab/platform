package httpupstream

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"

	"github.com/gilmanlab/platform/tools/labctl/internal/app/incusosimage"
)

const (
	// DefaultResponseHeaderTimeout bounds how long the client will wait for
	// upstream response headers before failing the connection.
	DefaultResponseHeaderTimeout = 30 * time.Second
	// DefaultIdleConnTimeout bounds how long an idle keep-alive connection
	// will be held open in the transport's idle pool.
	DefaultIdleConnTimeout = 90 * time.Second

	retryMaxAttempts     = 5
	retryInitialInterval = 500 * time.Millisecond
	retryMaxInterval     = 16 * time.Second
	retryRandomization   = 0.2

	sha256HexLen = 64
)

// NewHTTPClient constructs a [*http.Client] with explicit transport timeouts
// suited to upstream artifact downloads.
//
// The returned client has no top-level Timeout — long-running downloads use
// context cancellation. ResponseHeaderTimeout prevents stalled connections
// from hanging forever waiting for response headers.
func NewHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			ResponseHeaderTimeout: DefaultResponseHeaderTimeout,
			IdleConnTimeout:       DefaultIdleConnTimeout,
		},
	}
}

// Client fetches IncusOS and Talos upstream metadata and artifacts over HTTP.
type Client struct {
	httpClient *http.Client
}

// New constructs an HTTP upstream adapter. A nil httpClient selects the
// timeout-defaulted client returned by NewHTTPClient.
func New(httpClient *http.Client) Client {
	if httpClient == nil {
		httpClient = NewHTTPClient()
	}

	return Client{
		httpClient: httpClient,
	}
}

// FetchIndex loads an IncusOS image index.
func (c Client) FetchIndex(ctx context.Context, url string) (incusosimage.Index, error) {
	response, err := c.get(ctx, url)
	if err != nil {
		return incusosimage.Index{}, err
	}
	defer response.Body.Close()

	var index incusosimage.Index
	if err := json.NewDecoder(response.Body).Decode(&index); err != nil {
		return incusosimage.Index{}, fmt.Errorf("decode image index: %w", err)
	}

	return index, nil
}

// Download opens a response body for an upstream artifact.
//
// The returned ReadCloser must be closed by the caller. Body reads are not
// retried; transient mid-stream failures are surfaced as errors and the
// caller is expected to verify integrity (for example, by streaming through
// a SHA256 hasher and comparing against a sidecar fetched via FetchSHA256).
func (c Client) Download(ctx context.Context, url string) (io.ReadCloser, error) {
	response, err := c.get(ctx, url)
	if err != nil {
		return nil, err
	}

	return response.Body, nil
}

// FetchSHA256 fetches a checksum sidecar and returns the lowercase hex digest.
//
// The response body is expected to be in `sha256sum -c` format: a single
// line of "<hex>  <filename>". Only the first whitespace-delimited token is
// parsed; the filename and any trailing lines are ignored.
func (c Client) FetchSHA256(ctx context.Context, url string) (string, error) {
	response, err := c.get(ctx, url)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	scanner := bufio.NewScanner(response.Body)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("read sha256 sidecar %q: %w", url, err)
		}

		return "", fmt.Errorf("sha256 sidecar %q is empty", url)
	}

	fields := strings.Fields(scanner.Text())
	if len(fields) == 0 {
		return "", fmt.Errorf("sha256 sidecar %q has no digest", url)
	}

	digest := strings.ToLower(fields[0])
	if !isHexSHA256(digest) {
		return "", fmt.Errorf("sha256 sidecar %q digest %q is not a 64-character hex string", url, digest)
	}

	return digest, nil
}

func (c Client) get(ctx context.Context, url string) (*http.Response, error) {
	var response *http.Response

	operation := func() error {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return backoff.Permanent(fmt.Errorf("create request %q: %w", url, err))
		}

		resp, err := c.httpClient.Do(request)
		if err != nil {
			return err
		}

		if isRetryableStatus(resp.StatusCode) {
			status := resp.Status
			_ = resp.Body.Close()

			return fmt.Errorf("GET %q: retryable HTTP status %s", url, status)
		}
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			status := resp.Status
			_ = resp.Body.Close()

			return backoff.Permanent(fmt.Errorf("GET %q: unexpected HTTP status %s", url, status))
		}

		response = resp

		return nil
	}

	policy := backoff.WithContext(newRetryBackoff(), ctx)
	if err := backoff.Retry(operation, policy); err != nil {
		if permanent, ok := errors.AsType[*backoff.PermanentError](err); ok {
			return nil, permanent.Err
		}

		return nil, err
	}

	return response, nil
}

func newRetryBackoff() backoff.BackOff {
	expo := backoff.NewExponentialBackOff()
	expo.InitialInterval = retryInitialInterval
	expo.MaxInterval = retryMaxInterval
	expo.RandomizationFactor = retryRandomization
	expo.MaxElapsedTime = 0

	return backoff.WithMaxRetries(expo, retryMaxAttempts-1)
}

func isRetryableStatus(code int) bool {
	if code == http.StatusTooManyRequests {
		return true
	}

	return code >= http.StatusInternalServerError && code <= 599
}

func isHexSHA256(s string) bool {
	if len(s) != sha256HexLen {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		default:
			return false
		}
	}

	return true
}
