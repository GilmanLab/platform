package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/GilmanLab/platform/services/dns-mirror/internal/mirror"
)

// HTTPClient executes HTTP requests for zonefile snapshots.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// SnapshotStore persists fetched zonefile snapshots.
type SnapshotStore interface {
	Save(ctx context.Context, path string, content []byte) (mirror.Snapshot, error)
}

// Fetcher copies a remote zonefile snapshot to local storage.
type Fetcher struct {
	client HTTPClient
	store  SnapshotStore
}

// New constructs a Fetcher.
func New(client HTTPClient, store SnapshotStore) *Fetcher {
	return &Fetcher{
		client: client,
		store:  store,
	}
}

// Fetch retrieves a zonefile from sourceURL and writes it atomically to outputPath.
func (f *Fetcher) Fetch(ctx context.Context, sourceURL string, outputPath string) (mirror.Snapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return mirror.Snapshot{}, fmt.Errorf("create request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return mirror.Snapshot{}, fmt.Errorf("fetch zonefile: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return mirror.Snapshot{}, fmt.Errorf("fetch zonefile: unexpected status %d", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return mirror.Snapshot{}, fmt.Errorf("read zonefile response: %w", err)
	}

	if len(content) == 0 {
		return mirror.Snapshot{}, fmt.Errorf("fetch zonefile: empty response")
	}

	snapshot, err := f.store.Save(ctx, outputPath, content)
	if err != nil {
		return mirror.Snapshot{}, fmt.Errorf("save fetched zonefile: %w", err)
	}

	return snapshot, nil
}
