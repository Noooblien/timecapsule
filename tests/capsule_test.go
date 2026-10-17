package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Noooblien/timecapsule/pkg/archive"
	"github.com/Noooblien/timecapsule/pkg/server"
	"github.com/Noooblien/timecapsule/pkg/storage"
)

const sampleHTML = `<!DOCTYPE html>
<html>
<head>
  <title>DeepMind Gemini 3 Announcement</title>
  <link rel="stylesheet" href="/assets/style.css">
</head>
<body>
  <h1>Welcome to Gemini 3</h1>
  <p>Gemini 3 represents a fundamental leap in reasoning, autonomous capability, and multimodal synthesis.</p>
  <p>The model architecture introduces native temporal causality and zero-shot tool integration.</p>
  <img src="/images/banner.png" alt="Gemini">
  <script src="/js/app.js"></script>
</body>
</html>`

func TestArchive_Extraction(t *testing.T) {
	title := archive.ExtractTitle(sampleHTML, "fallback")
	if title != "DeepMind Gemini 3 Announcement" {
		t.Fatalf("expected title 'DeepMind Gemini 3 Announcement', got '%s'", title)
	}

	md := archive.ExtractMarkdown(sampleHTML, title)
	if !strings.Contains(md, "# DeepMind Gemini 3 Announcement") {
		t.Fatalf("missing title in markdown: %s", md)
	}
	if !strings.Contains(md, "Gemini 3 represents a fundamental leap") {
		t.Fatalf("missing paragraph content in markdown: %s", md)
	}

	assets := archive.ExtractAssets(sampleHTML)
	if len(assets) < 3 {
		t.Fatalf("expected at least 3 assets (css, img, js), got %d: %v", len(assets), assets)
	}
}

func TestArchive_WARCGeneration(t *testing.T) {
	now := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	snap := &archive.Snapshot{
		ID:          "tc_20261018000000_deadbeef",
		URL:         "https://deepmind.google/gemini",
		Timestamp:   now,
		WaybackTS:   archive.FormatWaybackTS(now),
		Title:       "Gemini 3",
		StatusCode:  200,
		ContentType: "text/html",
		HTML:        sampleHTML,
	}

	warc := archive.GenerateWARC(snap)
	warcStr := string(warc)

	if !strings.Contains(warcStr, "WARC/1.0") {
		t.Fatalf("missing WARC version header")
	}
	if !strings.Contains(warcStr, "WARC-Target-URI: https://deepmind.google/gemini") {
		t.Fatalf("missing WARC target URI")
	}
	if !strings.Contains(warcStr, "HTTP/1.1 200 OK") {
		t.Fatalf("missing HTTP response payload")
	}
}

func TestStorage_LifecycleAndSearch(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "timecapsule-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := storage.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	now := time.Now().UTC()
	snap := &archive.Snapshot{
		ID:          "tc_test_1",
		URL:         "https://example.com/blog/ai",
		Timestamp:   now,
		WaybackTS:   archive.FormatWaybackTS(now),
		Title:       "Autonomous AI Systems",
		StatusCode:  200,
		ContentType: "text/html",
		HTML:        sampleHTML,
		Markdown:    "# Autonomous AI Systems\n\nDeep learning models are evolving.",
		ContentHash: archive.ComputeHash([]byte(sampleHTML)),
		SizeBytes:   int64(len(sampleHTML)),
	}

	// 1. Save
	if err := store.Save(snap); err != nil {
		t.Fatalf("failed to save snapshot: %v", err)
	}

	// Verify on-disk files
	snapDir := filepath.Join(tmpDir, "capsules", snap.ID)
	if _, err := os.Stat(filepath.Join(snapDir, "index.html")); err != nil {
		t.Fatalf("index.html not created")
	}
	if _, err := os.Stat(filepath.Join(snapDir, "archive.warc")); err != nil {
		t.Fatalf("archive.warc not created")
	}
	if _, err := os.Stat(filepath.Join(snapDir, "content.md")); err != nil {
		t.Fatalf("content.md not created")
	}

	// 2. Get
	retrieved, ok := store.Get(snap.ID)
	if !ok || retrieved.Title != "Autonomous AI Systems" {
		t.Fatalf("failed to retrieve snapshot by ID")
	}

	// 3. Search
	results := store.Search("Autonomous")
	if len(results) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(results))
	}

	resultsEmpty := store.Search("NonExistentTerm12345")
	if len(resultsEmpty) != 0 {
		t.Fatalf("expected 0 results, got %d", len(resultsEmpty))
	}

	// 4. Time-travel lookup
	foundSnap, ok := store.FindByWayback("https://example.com/blog/ai", snap.WaybackTS)
	if !ok || foundSnap.ID != snap.ID {
		t.Fatalf("failed to find snapshot via Wayback timestamp")
	}
}

func TestServer_WaybackReplayAndAPIs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "timecapsule-server-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := storage.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	now := time.Now().UTC()
	snap := &archive.Snapshot{
		ID:          "tc_srv_test",
		URL:         "https://news.ycombinator.com",
		Timestamp:   now,
		WaybackTS:   archive.FormatWaybackTS(now),
		Title:       "Hacker News",
		StatusCode:  200,
		ContentType: "text/html",
		HTML:        sampleHTML,
		Markdown:    "# Hacker News\n\nTop stories today.",
		ContentHash: "feedbeef1234",
	}

	_ = store.Save(snap)

	archiver := archive.NewArchiver(5 * time.Second)
	srv := server.NewServer(store, archiver)
	handler := srv.Handler()

	// 1. Dashboard
	recIndex := httptest.NewRecorder()
	reqIndex, _ := http.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(recIndex, reqIndex)
	if recIndex.Code != http.StatusOK {
		t.Fatalf("expected 200 on dashboard, got %d", recIndex.Code)
	}
	if !strings.Contains(recIndex.Body.String(), "TimeCapsule") {
		t.Fatalf("dashboard missing TimeCapsule header")
	}

	// 2. Wayback Replay
	replayURL := "/web/" + snap.WaybackTS + "/https://news.ycombinator.com"
	recReplay := httptest.NewRecorder()
	reqReplay, _ := http.NewRequest(http.MethodGet, replayURL, nil)
	handler.ServeHTTP(recReplay, reqReplay)
	if recReplay.Code != http.StatusOK {
		t.Fatalf("expected 200 on wayback replay, got %d", recReplay.Code)
	}
	bodyStr := recReplay.Body.String()
	if !strings.Contains(bodyStr, "TimeCapsule Archive") {
		t.Fatalf("replay missing archive banner")
	}
	if !strings.Contains(bodyStr, "Welcome to Gemini 3") {
		t.Fatalf("replay missing original HTML content")
	}

	// 3. API snapshots list
	recAPI := httptest.NewRecorder()
	reqAPI, _ := http.NewRequest(http.MethodGet, "/api/snapshots", nil)
	handler.ServeHTTP(recAPI, reqAPI)
	if recAPI.Code != http.StatusOK {
		t.Fatalf("expected 200 on API, got %d", recAPI.Code)
	}
	if !strings.Contains(recAPI.Body.String(), "Hacker News") {
		t.Fatalf("API missing snapshot data")
	}

	// 4. Markdown View
	recMD := httptest.NewRecorder()
	reqMD, _ := http.NewRequest(http.MethodGet, "/markdown/"+snap.ID, nil)
	handler.ServeHTTP(recMD, reqMD)
	if recMD.Code != http.StatusOK {
		t.Fatalf("expected 200 on markdown, got %d", recMD.Code)
	}
	if !strings.Contains(recMD.Body.String(), "Top stories today.") {
		t.Fatalf("markdown endpoint returned unexpected content")
	}
}

func TestArchiver_LiveMockServer(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleHTML))
	}))
	defer mockServer.Close()

	archiver := archive.NewArchiver(5 * time.Second)
	snap, err := archiver.Capture(context.Background(), mockServer.URL)
	if err != nil {
		t.Fatalf("archiver capture failed: %v", err)
	}

	if snap.Title != "DeepMind Gemini 3 Announcement" {
		t.Fatalf("unexpected title: %s", snap.Title)
	}
	if snap.StatusCode != 200 {
		t.Fatalf("unexpected status code: %d", snap.StatusCode)
	}
	if snap.SizeBytes == 0 {
		t.Fatalf("unexpected size: %d", snap.SizeBytes)
	}
}
