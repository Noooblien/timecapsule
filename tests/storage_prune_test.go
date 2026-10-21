package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Noooblien/timecapsule/pkg/archive"
	"github.com/Noooblien/timecapsule/pkg/server"
	"github.com/Noooblien/timecapsule/pkg/storage"
)

func TestServer_StorageStatsAndPruneEndpoints(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "timecapsule-server-prune-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := storage.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	targetURL := "https://example.com/api-test"
	now := time.Now().UTC()

	snaps := []*archive.Snapshot{
		{
			ID:          "srv_snap_1",
			URL:         targetURL,
			Timestamp:   now.Add(-2 * time.Hour),
			WaybackTS:   archive.FormatWaybackTS(now.Add(-2 * time.Hour)),
			Title:       "API Test v1",
			HTML:        "<html>same content</html>",
			ContentHash: "hash_ident",
			SizeBytes:   5000,
		},
		{
			ID:          "srv_snap_2",
			URL:         targetURL,
			Timestamp:   now.Add(-1 * time.Hour),
			WaybackTS:   archive.FormatWaybackTS(now.Add(-1 * time.Hour)),
			Title:       "API Test v2 (Dup)",
			HTML:        "<html>same content</html>",
			ContentHash: "hash_ident",
			SizeBytes:   5000,
		},
	}

	for _, s := range snaps {
		if err := store.Save(s); err != nil {
			t.Fatalf("failed to save snapshot: %v", err)
		}
	}

	srv := server.NewServer(store, nil)
	handler := srv.Handler()

	// 1. GET /api/storage/stats
	reqStats, _ := http.NewRequest(http.MethodGet, "/api/storage/stats", nil)
	recStats := httptest.NewRecorder()
	handler.ServeHTTP(recStats, reqStats)

	if recStats.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/storage/stats, got %d", recStats.Code)
	}
	var stats storage.StorageStats
	if err := json.NewDecoder(recStats.Body).Decode(&stats); err != nil {
		t.Fatalf("failed to decode stats json: %v", err)
	}
	if stats.TotalSnapshots != 2 || stats.TotalBytes != 10000 {
		t.Fatalf("unexpected stats: %+v", stats)
	}

	// 2. POST /api/storage/prune?dedup=true&dry_run=true
	reqPruneDry, _ := http.NewRequest(http.MethodPost, "/api/storage/prune?dedup=true&dry_run=true", nil)
	recPruneDry := httptest.NewRecorder()
	handler.ServeHTTP(recPruneDry, reqPruneDry)

	if recPruneDry.Code != http.StatusOK {
		t.Fatalf("expected 200 on dry run prune, got %d", recPruneDry.Code)
	}
	var repDry storage.PruneReport
	if err := json.NewDecoder(recPruneDry.Body).Decode(&repDry); err != nil {
		t.Fatalf("failed to decode prune report: %v", err)
	}
	if len(repDry.Candidates) != 1 || repDry.DeletedCount != 0 {
		t.Fatalf("expected 1 candidate and 0 deletions in dry run: %+v", repDry)
	}

	// 3. POST /api/storage/prune?dedup=true (execute delete)
	reqPruneExec, _ := http.NewRequest(http.MethodPost, "/api/storage/prune?dedup=true", nil)
	recPruneExec := httptest.NewRecorder()
	handler.ServeHTTP(recPruneExec, reqPruneExec)

	if recPruneExec.Code != http.StatusOK {
		t.Fatalf("expected 200 on execute prune, got %d", recPruneExec.Code)
	}
	var repExec storage.PruneReport
	if err := json.NewDecoder(recPruneExec.Body).Decode(&repExec); err != nil {
		t.Fatalf("failed to decode prune report: %v", err)
	}
	if repExec.DeletedCount != 1 {
		t.Fatalf("expected 1 deleted snapshot, got %d", repExec.DeletedCount)
	}

	// Verify only 1 remaining in store
	if len(store.List()) != 1 {
		t.Fatalf("expected 1 remaining snapshot, got %d", len(store.List()))
	}
}
