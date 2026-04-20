package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/GilmanLab/platform/services/dns-mirror/internal/httpapi"
	"github.com/GilmanLab/platform/services/dns-mirror/internal/mirror"
)

type snapshotReader struct {
	snapshot mirror.Snapshot
	ok       bool
}

func (s snapshotReader) CurrentSnapshot() (mirror.Snapshot, bool) {
	return s.snapshot, s.ok
}

func TestZonefileReturnsSnapshot(t *testing.T) {
	handler := httpapi.NewHandler(snapshotReader{
		snapshot: mirror.Snapshot{
			Content:   []byte("glab.lol.\t300\tIN\tNS\tns-1.example.\n"),
			UpdatedAt: time.Now(),
		},
		ok: true,
	})

	request := httptest.NewRequest(http.MethodGet, "/zonefile", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "text/plain; charset=utf-8", recorder.Header().Get("Content-Type"))
	assert.Contains(t, recorder.Body.String(), "glab.lol.")
}

func TestZonefileReturnsUnavailableWithoutSnapshot(t *testing.T) {
	handler := httpapi.NewHandler(snapshotReader{})

	request := httptest.NewRequest(http.MethodGet, "/zonefile", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestReadyzReflectsSnapshotAvailability(t *testing.T) {
	readyHandler := httpapi.NewHandler(snapshotReader{
		snapshot: mirror.Snapshot{Content: []byte("zone")},
		ok:       true,
	})
	notReadyHandler := httpapi.NewHandler(snapshotReader{})

	readyRequest := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	readyRecorder := httptest.NewRecorder()
	readyHandler.ServeHTTP(readyRecorder, readyRequest)

	notReadyRequest := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	notReadyRecorder := httptest.NewRecorder()
	notReadyHandler.ServeHTTP(notReadyRecorder, notReadyRequest)

	assert.Equal(t, http.StatusOK, readyRecorder.Code)
	assert.Equal(t, http.StatusServiceUnavailable, notReadyRecorder.Code)
}
