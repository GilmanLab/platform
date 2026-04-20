package snapshot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/GilmanLab/platform/services/dns-mirror/internal/mirror"
)

// Store persists snapshots on the local filesystem.
type Store struct{}

// NewStore constructs a filesystem-backed snapshot store.
func NewStore() *Store {
	return &Store{}
}

// Load loads the current snapshot from disk.
func (s *Store) Load(_ context.Context, path string) (mirror.Snapshot, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return mirror.Snapshot{}, mirror.ErrSnapshotNotFound
		}

		return mirror.Snapshot{}, fmt.Errorf("stat snapshot: %w", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return mirror.Snapshot{}, fmt.Errorf("read snapshot: %w", err)
	}

	return mirror.Snapshot{
		Content:   content,
		UpdatedAt: info.ModTime().UTC(),
	}, nil
}

// Save writes the snapshot atomically and returns the persisted snapshot metadata.
func (s *Store) Save(_ context.Context, path string, content []byte) (mirror.Snapshot, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return mirror.Snapshot{}, fmt.Errorf("create snapshot directory: %w", err)
	}

	file, err := os.CreateTemp(filepath.Dir(path), ".dns-mirror-*.tmp")
	if err != nil {
		return mirror.Snapshot{}, fmt.Errorf("create temp snapshot: %w", err)
	}

	defer os.Remove(file.Name())

	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		return mirror.Snapshot{}, fmt.Errorf("write temp snapshot: %w", err)
	}

	if err := file.Sync(); err != nil {
		_ = file.Close()
		return mirror.Snapshot{}, fmt.Errorf("sync temp snapshot: %w", err)
	}

	if err := file.Close(); err != nil {
		return mirror.Snapshot{}, fmt.Errorf("close temp snapshot: %w", err)
	}

	if err := os.Rename(file.Name(), path); err != nil {
		return mirror.Snapshot{}, fmt.Errorf("rename temp snapshot: %w", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return mirror.Snapshot{}, fmt.Errorf("stat saved snapshot: %w", err)
	}

	return mirror.Snapshot{
		Content:   append([]byte(nil), content...),
		UpdatedAt: info.ModTime().UTC(),
	}, nil
}
