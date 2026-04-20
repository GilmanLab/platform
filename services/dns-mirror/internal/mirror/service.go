package mirror

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"
)

// ErrSnapshotNotFound indicates that no on-disk snapshot exists yet.
var ErrSnapshotNotFound = errors.New("snapshot not found")

// RecordSet is the normalized DNS record set model used inside the mirror.
type RecordSet struct {
	// Name is the fully qualified record owner name.
	Name string
	// Type is the DNS RR type, for example `A` or `TXT`.
	Type string
	// TTL is the per-record-set TTL in seconds.
	TTL int64
	// Values are the textual record values as expected by a zonefile parser.
	Values []string
}

// Zone is the normalized hosted-zone snapshot loaded from Route 53.
type Zone struct {
	// Name is the fully qualified origin name for the zone.
	Name string
	// RecordSets is the complete set of supported record sets in the zone.
	RecordSets []RecordSet
}

// Snapshot is the current rendered zonefile and the time it was last refreshed.
type Snapshot struct {
	// Content is the rendered zonefile bytes.
	Content []byte
	// UpdatedAt is the time the snapshot was loaded or written.
	UpdatedAt time.Time
}

// Renderer converts a normalized Zone into a zonefile.
type Renderer interface {
	Render(zone Zone) ([]byte, error)
}

// SnapshotStore persists rendered zonefiles.
type SnapshotStore interface {
	Load(ctx context.Context, path string) (Snapshot, error)
	Save(ctx context.Context, path string, content []byte) (Snapshot, error)
}

// ZoneSource loads hosted-zone data from an upstream system.
type ZoneSource interface {
	LoadZone(ctx context.Context, hostedZoneID string) (Zone, error)
}

// Service runs the DNS mirror sync loop and tracks the latest snapshot.
type Service struct {
	logger   *slog.Logger
	renderer Renderer
	store    SnapshotStore
	source   ZoneSource

	mu       sync.RWMutex
	snapshot *Snapshot
}

// NewService constructs a Service from the supplied ports.
func NewService(source ZoneSource, renderer Renderer, store SnapshotStore, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &Service{
		logger:   logger,
		renderer: renderer,
		store:    store,
		source:   source,
	}
}

// CurrentSnapshot returns a copy of the latest snapshot, if one exists.
func (s *Service) CurrentSnapshot() (Snapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.snapshot == nil {
		return Snapshot{}, false
	}

	return Snapshot{
		Content:   append([]byte(nil), s.snapshot.Content...),
		UpdatedAt: s.snapshot.UpdatedAt,
	}, true
}

// LoadSnapshot loads the last rendered snapshot from disk, if present.
func (s *Service) LoadSnapshot(ctx context.Context, path string) error {
	snapshot, err := s.store.Load(ctx, path)
	if err != nil {
		return err
	}

	s.setSnapshot(snapshot)
	s.logger.Info("loaded snapshot from disk", "path", path, "age", time.Since(snapshot.UpdatedAt).Round(time.Second))

	return nil
}

// Run performs an immediate sync and then continues syncing until the context is canceled.
func (s *Service) Run(ctx context.Context, hostedZoneID string, outputPath string, interval time.Duration) error {
	s.syncAndLog(ctx, hostedZoneID, outputPath)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			s.syncAndLog(ctx, hostedZoneID, outputPath)
		}
	}
}

// SyncOnce performs one upstream sync and persists the rendered snapshot.
func (s *Service) SyncOnce(ctx context.Context, hostedZoneID string, outputPath string) error {
	zone, err := s.source.LoadZone(ctx, hostedZoneID)
	if err != nil {
		return err
	}

	content, err := s.renderer.Render(zone)
	if err != nil {
		return err
	}

	snapshot, err := s.store.Save(ctx, outputPath, content)
	if err != nil {
		return err
	}

	s.setSnapshot(snapshot)
	s.logger.Info("synchronized zone", "hosted_zone_id", hostedZoneID, "path", outputPath, "bytes", len(snapshot.Content))

	return nil
}

func (s *Service) setSnapshot(snapshot Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshot = &Snapshot{
		Content:   append([]byte(nil), snapshot.Content...),
		UpdatedAt: snapshot.UpdatedAt,
	}
}

func (s *Service) syncAndLog(ctx context.Context, hostedZoneID string, outputPath string) {
	if err := s.SyncOnce(ctx, hostedZoneID, outputPath); err != nil {
		attrs := []any{
			"hosted_zone_id", hostedZoneID,
			"path", outputPath,
			"error", err,
		}

		if snapshot, ok := s.CurrentSnapshot(); ok {
			attrs = append(attrs, "snapshot_age", time.Since(snapshot.UpdatedAt).Round(time.Second))
		}

		s.logger.Error("synchronize zone", attrs...)
	}
}
