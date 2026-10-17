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
	"github.com/Noooblien/timecapsule/pkg/diff"
	"github.com/Noooblien/timecapsule/pkg/server"
	"github.com/Noooblien/timecapsule/pkg/storage"
)

func TestDiff_LCSAndFormatting(t *testing.T) {
	textA := "Line 1\nLine 2\nLine 3"
	textB := "Line 1\nLine 2 modified\nLine 3\nLine 4 added"

	res := diff.CompareText("old", "new", textA, textB)

	if res.Identical {
		t.Fatalf("expected diff to be non-identical")
	}
	if res.AddedCount != 2 {
		t.Fatalf("expected 2 added lines, got %d", res.AddedCount)
	}
	if res.RemovedCount != 1 {
		t.Fatalf("expected 1 removed line, got %d", res.RemovedCount)
	}
	if res.Similarity <= 0 || res.Similarity >= 1.0 {
		t.Fatalf("expected similarity between 0 and 1, got %f", res.Similarity)
	}

	ansi := res.FormatANSI()
	if !strings.Contains(ansi, "\033[32m+") || !strings.Contains(ansi, "\033[31m-") {
		t.Fatalf("ANSI formatting missing color escape codes")
	}

	htmlDiff := res.FormatHTML()
	if !strings.Contains(htmlDiff, "+ Line 4 added") || !strings.Contains(htmlDiff, "- Line 2") {
		t.Fatalf("HTML formatting missing added/removed lines")
	}

	// Test identical texts
	sameRes := diff.CompareText("old", "new", textA, textA)
	if !sameRes.Identical {
		t.Fatalf("expected identical flag to be true")
	}
	if sameRes.Similarity != 1.0 {
		t.Fatalf("expected similarity to be 1.0, got %f", sameRes.Similarity)
	}
}

func TestDiff_SnapshotComparison(t *testing.T) {
	t1 := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 10, 18, 1, 0, 0, 0, time.UTC)

	snap1 := &archive.Snapshot{
		ID:          "snap_1",
		URL:         "https://news.ycombinator.com",
		Timestamp:   t1,
		ContentHash: "hash111",
		Markdown:    "# Hacker News\n1. Project A (100 points)\n2. Project B (80 points)",
	}

	snap2 := &archive.Snapshot{
		ID:          "snap_2",
		URL:         "https://news.ycombinator.com",
		Timestamp:   t2,
		ContentHash: "hash222",
		Markdown:    "# Hacker News\n1. Project A (150 points)\n2. Project B (80 points)\n3. Project C (50 points)",
	}

	res := diff.CompareSnapshots(snap1, snap2)
	if res.Identical {
		t.Fatalf("expected diff between snap1 and snap2")
	}
	if res.AddedCount != 2 { // "1. Project A (150 points)" and "3. Project C (50 points)"
		t.Fatalf("expected 2 added lines, got %d", res.AddedCount)
	}
	if res.RemovedCount != 1 { // "1. Project A (100 points)"
		t.Fatalf("expected 1 removed line, got %d", res.RemovedCount)
	}

	// Compare with itself
	resIdentical := diff.CompareSnapshots(snap1, snap1)
	if !resIdentical.Identical {
		t.Fatalf("expected self-comparison to be identical")
	}
}

func TestDiff_ServerEndpoints(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "timecapsule-diff-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := storage.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	t1 := time.Now().UTC().Add(-1 * time.Hour)
	t2 := time.Now().UTC()

	snap1 := &archive.Snapshot{
		ID:          "snap_100",
		URL:         "https://example.com",
		Timestamp:   t1,
		WaybackTS:   archive.FormatWaybackTS(t1),
		Title:       "Example V1",
		Markdown:    "# Title V1\nWelcome to our page.",
		ContentHash: "hash100",
	}
	snap2 := &archive.Snapshot{
		ID:          "snap_200",
		URL:         "https://example.com",
		Timestamp:   t2,
		WaybackTS:   archive.FormatWaybackTS(t2),
		Title:       "Example V2",
		Markdown:    "# Title V2\nWelcome to our page.\nNew features announced!",
		ContentHash: "hash200",
	}

	_ = store.Save(snap1)
	_ = store.Save(snap2)

	// Verify GetHistory
	history := store.GetHistory("https://example.com")
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(history))
	}
	if history[0].ID != "snap_100" || history[1].ID != "snap_200" {
		t.Fatalf("expected history to be sorted chronologically")
	}

	archiver := archive.NewArchiver(5 * time.Second)
	srv := server.NewServer(store, archiver)
	handler := srv.Handler()

	// 1. Web Diff endpoint: /diff/{id1}/{id2}
	recDiff := httptest.NewRecorder()
	reqDiff, _ := http.NewRequest(http.MethodGet, "/diff/snap_100/snap_200", nil)
	handler.ServeHTTP(recDiff, reqDiff)
	if recDiff.Code != http.StatusOK {
		t.Fatalf("expected 200 on /diff/, got %d", recDiff.Code)
	}
	body := recDiff.Body.String()
	if !strings.Contains(body, "Snapshot Comparison") {
		t.Fatalf("expected header 'Snapshot Comparison' in body")
	}
	if !strings.Contains(body, "New features announced!") {
		t.Fatalf("expected diff content in body")
	}

	// 2. API Diff endpoint: /api/diff?old=snap_100&new=snap_200
	recAPI := httptest.NewRecorder()
	reqAPI, _ := http.NewRequest(http.MethodGet, "/api/diff?old=snap_100&new=snap_200", nil)
	handler.ServeHTTP(recAPI, reqAPI)
	if recAPI.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/diff, got %d", recAPI.Code)
	}

	var apiRes diff.DiffResult
	if err := json.NewDecoder(recAPI.Body).Decode(&apiRes); err != nil {
		t.Fatalf("failed to decode API diff JSON: %v", err)
	}
	if apiRes.AddedCount == 0 {
		t.Fatalf("expected added lines in API diff result")
	}

	// 3. Check dashboard renders "Compare with Prev" button
	recDash := httptest.NewRecorder()
	reqDash, _ := http.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(recDash, reqDash)
	if !strings.Contains(recDash.Body.String(), "Compare with Prev ⇄") {
		t.Fatalf("dashboard missing 'Compare with Prev ⇄' link")
	}
}
