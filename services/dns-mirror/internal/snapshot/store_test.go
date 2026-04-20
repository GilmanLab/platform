package snapshot_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/GilmanLab/platform/services/dns-mirror/internal/mirror"
	"github.com/GilmanLab/platform/services/dns-mirror/internal/snapshot"
)

func TestSaveAndLoadSnapshot(t *testing.T) {
	store := snapshot.NewStore()
	path := filepath.Join(t.TempDir(), "glab.lol.zone")

	savedSnapshot, err := store.Save(context.Background(), path, []byte("zonefile"))
	require.NoError(t, err)

	loadedSnapshot, err := store.Load(context.Background(), path)
	require.NoError(t, err)

	assert.Equal(t, []byte("zonefile"), loadedSnapshot.Content)
	assert.WithinDuration(t, savedSnapshot.UpdatedAt, loadedSnapshot.UpdatedAt, 2)
}

func TestLoadMissingSnapshot(t *testing.T) {
	store := snapshot.NewStore()

	_, err := store.Load(context.Background(), filepath.Join(t.TempDir(), "missing.zone"))
	require.ErrorIs(t, err, mirror.ErrSnapshotNotFound)
}
