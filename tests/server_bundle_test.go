package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Noooblien/timecapsule/pkg/archive"
	"github.com/Noooblien/timecapsule/pkg/bundle"
	"github.com/Noooblien/timecapsule/pkg/server"
	"github.com/Noooblien/timecapsule/pkg/storage"
)

func TestServer_BundleEndpoints(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "timecapsule-server-bundle-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := storage.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	targetURL := "https://example.com/bundle-test"
	now := time.Now().UTC()
	htmlContent := `<!DOCTYPE html>
<html>
<head>
<title>Bundle Server Test</title>
<script>alert("runaway script")</script>
</head>
<body>
<h1>Hermetic Document</h1>
<p>Test content for single-file bundle endpoint</p>
</body>
</html>`

	snap := &archive.Snapshot{
		ID:          "snap_bundle_test",
		URL:         targetURL,
		Timestamp:   now,
		WaybackTS:   archive.FormatWaybackTS(now),
		Title:       "Bundle Server Test",
		StatusCode:  200,
		ContentType: "text/html; charset=utf-8",
		HTML:        htmlContent,
		ContentHash: archive.ComputeHash([]byte(htmlContent)),
		SizeBytes:   int64(len(htmlContent)),
	}

	if err := store.Save(snap); err != nil {
		t.Fatalf("failed to save test snapshot: %v", err)
	}

	srv := server.NewServer(store, nil)
	handler := srv.Handler()

	// 1. GET /bundle/{id}
	reqBundle, _ := http.NewRequest(http.MethodGet, "/bundle/"+snap.ID, nil)
	recBundle := httptest.NewRecorder()
	handler.ServeHTTP(recBundle, reqBundle)

	if recBundle.Code != http.StatusOK {
		t.Fatalf("expected 200 on /bundle/{id}, got %d", recBundle.Code)
	}
	bodyBundle := recBundle.Body.String()
	if !strings.Contains(bodyBundle, "Hermetic Document") {
		t.Fatalf("expected bundled HTML to contain 'Hermetic Document': %s", bodyBundle)
	}
	// Check that script was neutralized
	if !strings.Contains(bodyBundle, "data-timecapsule-neutralized") {
		t.Fatalf("expected script neutralization in bundle: %s", bodyBundle)
	}

	// 2. GET /api/bundle?id=...
	reqAPI, _ := http.NewRequest(http.MethodGet, "/api/bundle?id="+snap.ID, nil)
	recAPI := httptest.NewRecorder()
	handler.ServeHTTP(recAPI, reqAPI)

	if recAPI.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/bundle?id=..., got %d", recAPI.Code)
	}
	var res bundle.BundleResult
	if err := json.NewDecoder(recAPI.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode bundle json: %v", err)
	}
	if res.OriginalURL != targetURL || res.BundledSizeBytes == 0 {
		t.Fatalf("unexpected bundle result: %+v", res)
	}
}
