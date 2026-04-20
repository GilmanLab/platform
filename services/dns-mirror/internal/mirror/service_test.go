package mirror_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/GilmanLab/platform/services/dns-mirror/internal/mirror"
)

type sourceStub struct {
	err  error
	zone mirror.Zone
}

func (s sourceStub) LoadZone(_ context.Context, _ string) (mirror.Zone, error) {
	if s.err != nil {
		return mirror.Zone{}, s.err
	}

	return s.zone, nil
}

type rendererStub struct {
	content []byte
	err     error
}

func (r rendererStub) Render(_ mirror.Zone) ([]byte, error) {
	if r.err != nil {
		return nil, r.err
	}

	return append([]byte(nil), r.content...), nil
}

type storeStub struct {
	loadErr      error
	loadSnapshot mirror.Snapshot
	saveErr      error
	savedPath    string
	savedContent []byte
}

func (s *storeStub) Load(_ context.Context, _ string) (mirror.Snapshot, error) {
	if s.loadErr != nil {
		return mirror.Snapshot{}, s.loadErr
	}

	return s.loadSnapshot, nil
}

func (s *storeStub) Save(_ context.Context, path string, content []byte) (mirror.Snapshot, error) {
	if s.saveErr != nil {
		return mirror.Snapshot{}, s.saveErr
	}

	s.savedPath = path
	s.savedContent = append([]byte(nil), content...)

	return mirror.Snapshot{
		Content:   append([]byte(nil), content...),
		UpdatedAt: time.Now().UTC(),
	}, nil
}

func TestLoadSnapshotSetsCurrentSnapshot(t *testing.T) {
	store := &storeStub{
		loadSnapshot: mirror.Snapshot{
			Content:   []byte("cached zonefile"),
			UpdatedAt: time.Now().Add(-time.Minute).UTC(),
		},
	}

	service := mirror.NewService(sourceStub{}, rendererStub{}, store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, service.LoadSnapshot(context.Background(), "/tmp/glab.zone"))

	snapshot, ok := service.CurrentSnapshot()
	require.True(t, ok)
	assert.Equal(t, []byte("cached zonefile"), snapshot.Content)
}

func TestSyncOncePersistsAndPublishesSnapshot(t *testing.T) {
	store := &storeStub{}
	service := mirror.NewService(
		sourceStub{zone: mirror.Zone{Name: "glab.lol."}},
		rendererStub{content: []byte("rendered zonefile")},
		store,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	require.NoError(t, service.SyncOnce(context.Background(), "Z123", "/var/lib/dns-mirror/glab.lol.zone"))

	snapshot, ok := service.CurrentSnapshot()
	require.True(t, ok)
	assert.Equal(t, []byte("rendered zonefile"), snapshot.Content)
	assert.Equal(t, "/var/lib/dns-mirror/glab.lol.zone", store.savedPath)
	assert.Equal(t, []byte("rendered zonefile"), store.savedContent)
}

func TestSyncOnceKeepsLastGoodSnapshotOnFailure(t *testing.T) {
	store := &storeStub{
		loadSnapshot: mirror.Snapshot{
			Content:   []byte("last good snapshot"),
			UpdatedAt: time.Now().Add(-time.Minute).UTC(),
		},
	}

	service := mirror.NewService(
		sourceStub{err: errors.New("route53 unavailable")},
		rendererStub{},
		store,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	require.NoError(t, service.LoadSnapshot(context.Background(), "/tmp/glab.zone"))

	err := service.SyncOnce(context.Background(), "Z123", "/tmp/glab.zone")
	require.Error(t, err)

	snapshot, ok := service.CurrentSnapshot()
	require.True(t, ok)
	assert.Equal(t, []byte("last good snapshot"), snapshot.Content)
}
