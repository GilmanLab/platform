package httpupstream

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gilmanlab/platform/tools/labctl/internal/app/incusosimage"
)

// Client fetches IncusOS upstream metadata and artifacts over HTTP.
type Client struct {
	httpClient *http.Client
}

// New constructs an HTTP upstream adapter.
func New(httpClient *http.Client) Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
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
func (c Client) Download(ctx context.Context, url string) (io.ReadCloser, error) {
	response, err := c.get(ctx, url)
	if err != nil {
		return nil, err
	}

	return response.Body, nil
}

func (c Client) get(ctx context.Context, url string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request %q: %w", url, err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("GET %q: %w", url, err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		defer response.Body.Close()

		return nil, fmt.Errorf("GET %q: unexpected HTTP status %s", url, response.Status)
	}

	return response, nil
}
