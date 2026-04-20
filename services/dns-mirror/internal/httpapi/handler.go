package httpapi

import (
	"net/http"

	"github.com/GilmanLab/platform/services/dns-mirror/internal/mirror"
)

// SnapshotReader exposes the latest rendered zonefile snapshot.
type SnapshotReader interface {
	CurrentSnapshot() (mirror.Snapshot, bool)
}

// NewHandler creates the dns-mirror HTTP handler set.
func NewHandler(reader SnapshotReader) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if _, ok := reader.CurrentSnapshot(); !ok {
			http.Error(w, "snapshot unavailable", http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})

	mux.HandleFunc("/zonefile", func(w http.ResponseWriter, _ *http.Request) {
		snapshot, ok := reader.CurrentSnapshot()
		if !ok {
			http.Error(w, "snapshot unavailable", http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(snapshot.Content)
	})

	return mux
}
