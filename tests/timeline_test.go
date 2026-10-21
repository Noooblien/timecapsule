package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Noooblien/timecapsule/pkg/archive"
	"github.com/Noooblien/timecapsule/pkg/server"
	"github.com/Noooblien/timecapsule/pkg/storage"
	"github.com/Noooblien/timecapsule/pkg/timeline"
)

func TestServer_TimelineEndpoints(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "timecapsule-tl-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := storage.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	targetURL := "https://golang.org/doc"
	t1 := time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 6, 15, 15, 30, 0, 0, time.UTC)
	t3 := time.Date(2026, 2, 20, 18, 0, 0, 0, time.UTC)

	snaps := []*archive.Snapshot{
		{
			ID:          "snap_1",
			URL:         targetURL,
			Timestamp:   t1,
			WaybackTS:   archive.FormatWaybackTS(t1),
			Title:       "Go Documentation - Jan 2025",
			StatusCode:  200,
			ContentType: "text/html",
			HTML:        "<html><body><h1>Go 1.24 Docs</h1></body></html>",
			ContentHash: "hash11111111",
			SizeBytes:   12000,
		},
		{
			ID:          "snap_2",
			URL:         targetURL,
			Timestamp:   t2,
			WaybackTS:   archive.FormatWaybackTS(t2),
			Title:       "Go Documentation - Jun 2025",
			StatusCode:  200,
			ContentType: "text/html",
			HTML:        "<html><body><h1>Go 1.25 Release Candidate Docs</h1></body></html>",
			ContentHash: "hash22222222",
			SizeBytes:   13500,
		},
		{
			ID:          "snap_3",
			URL:         targetURL,
			Timestamp:   t3,
			WaybackTS:   archive.FormatWaybackTS(t3),
			Title:       "Go Documentation - Feb 2026",
			StatusCode:  200,
			ContentType: "text/html",
			HTML:        "<html><body><h1>Go 1.26 Stable Docs</h1></body></html>",
			ContentHash: "hash33333333",
			SizeBytes:   15200,
		},
	}

	for _, s := range snaps {
		if err := store.Save(s); err != nil {
			t.Fatalf("failed to save test snapshot: %v", err)
		}
	}

	srv := server.NewServer(store, nil)
	handler := srv.Handler()

	// 1. Test HTML Timeline page: /timeline/{url}
	reqPage, _ := http.NewRequest(http.MethodGet, "/timeline/"+targetURL, nil)
	recPage := httptest.NewRecorder()
	handler.ServeHTTP(recPage, reqPage)

	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 on /timeline/, got %d", recPage.Code)
	}
	bodyPage := recPage.Body.String()
	if !strings.Contains(bodyPage, "Temporal Timeline") {
		t.Fatalf("expected 'Temporal Timeline' in response: %s", bodyPage)
	}
	if !strings.Contains(bodyPage, "3") { // 3 captures
		t.Fatalf("expected 3 captures in response: %s", bodyPage)
	}
	if !strings.Contains(bodyPage, "2025") || !strings.Contains(bodyPage, "2026") {
		t.Fatalf("expected year histogram headers in response: %s", bodyPage)
	}

	// 2. Test JSON API Timeline: /api/timeline/?url=...
	reqAPI, _ := http.NewRequest(http.MethodGet, "/api/timeline/?url="+targetURL, nil)
	recAPI := httptest.NewRecorder()
	handler.ServeHTTP(recAPI, reqAPI)

	if recAPI.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/timeline/, got %d", recAPI.Code)
	}
	var tl timeline.URLTimeline
	if err := json.NewDecoder(recAPI.Body).Decode(&tl); err != nil {
		t.Fatalf("failed to decode timeline json: %v", err)
	}
	if tl.TotalCaptures != 3 {
		t.Fatalf("expected 3 total captures, got %d", tl.TotalCaptures)
	}
	if len(tl.Years) != 2 {
		t.Fatalf("expected 2 years, got %d", len(tl.Years))
	}

	// 3. Test Wayback Replay with Rich Navigation Banner
	// Snap 2 has both prev (Snap 1) and next (Snap 3)
	reqReplay, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/web/%s/%s", snaps[1].WaybackTS, targetURL), nil)
	recReplay := httptest.NewRecorder()
	handler.ServeHTTP(recReplay, reqReplay)

	if recReplay.Code != http.StatusOK {
		t.Fatalf("expected 200 on replay, got %d", recReplay.Code)
	}
	bodyReplay := recReplay.Body.String()
	if !strings.Contains(bodyReplay, "Capture 2 of 3") {
		t.Fatalf("expected 'Capture 2 of 3' in banner, got: %s", bodyReplay)
	}
	if !strings.Contains(bodyReplay, "Prev") || !strings.Contains(bodyReplay, "Next") {
		t.Fatalf("expected Prev/Next buttons in banner, got: %s", bodyReplay)
	}
	if !strings.Contains(bodyReplay, "Diff ⇄") {
		t.Fatalf("expected Diff link in banner for snapshot with predecessor, got: %s", bodyReplay)
	}
}
