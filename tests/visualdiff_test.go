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
	"github.com/Noooblien/timecapsule/pkg/server"
	"github.com/Noooblien/timecapsule/pkg/storage"
	"github.com/Noooblien/timecapsule/pkg/visualdiff"
)

func TestVisualDiff_EndToEndDemoSuite(t *testing.T) {
	outANSI, err := visualdiff.RunDemo(false, false)
	if err != nil {
		t.Fatalf("RunDemo ANSI failed: %v", err)
	}
	if !strings.Contains(outANSI, "TIMECAPSULE TEMPORAL VISUAL & DOM DIFF SCORECARD") {
		t.Fatalf("ANSI demo output missing header")
	}

	outMermaid, err := visualdiff.RunDemo(true, false)
	if err != nil {
		t.Fatalf("RunDemo Mermaid failed: %v", err)
	}
	if !strings.Contains(outMermaid, "flowchart TD") {
		t.Fatalf("Mermaid demo output missing flowchart")
	}

	outMarkdown, err := visualdiff.RunDemo(false, true)
	if err != nil {
		t.Fatalf("RunDemo Markdown failed: %v", err)
	}
	if !strings.Contains(outMarkdown, "TimeCapsule Visual & DOM Diff Report") {
		t.Fatalf("Markdown demo output missing report title")
	}
}

func TestVisualDiff_StoreIntegration(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "timecapsule_vdiff_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := storage.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	html1 := "<html><body><header><h1>App</h1></header><main><p>Welcome v1</p></main></body></html>"
	html2 := "<html><body><header><h1>App</h1></header><main><p>Welcome v2</p><div class=\"banner\"><p>New!</p></div></main></body></html>"

	snap1 := &archive.Snapshot{
		ID:          "snap_test_1",
		URL:         "https://example.com/app",
		Timestamp:   time.Now().Add(-time.Hour),
		WaybackTS:   archive.FormatWaybackTS(time.Now().Add(-time.Hour)),
		Title:       "App",
		HTML:        html1,
		ContentHash: archive.ComputeHash([]byte(html1)),
		SizeBytes:   int64(len(html1)),
	}
	snap2 := &archive.Snapshot{
		ID:          "snap_test_2",
		URL:         "https://example.com/app",
		Timestamp:   time.Now(),
		WaybackTS:   archive.FormatWaybackTS(time.Now()),
		Title:       "App",
		HTML:        html2,
		ContentHash: archive.ComputeHash([]byte(html2)),
		SizeBytes:   int64(len(html2)),
	}

	if err := store.Save(snap1); err != nil {
		t.Fatalf("failed to save snap1: %v", err)
	}
	if err := store.Save(snap2); err != nil {
		t.Fatalf("failed to save snap2: %v", err)
	}

	engine := visualdiff.NewEngine(visualdiff.DefaultVisualDiffConfig())
	res := engine.CompareSnapshots(snap1, snap2)

	if res.AddedNodes == 0 {
		t.Errorf("expected added nodes for banner div")
	}
	if res.ModifiedNodes == 0 {
		t.Errorf("expected text modification for welcome paragraph")
	}
}

func TestVisualDiff_ServerEndpoints(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "timecapsule_vdiff_srv_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := storage.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	snap1 := &archive.Snapshot{
		ID:          "snap_vdiff_srv1",
		URL:         "https://service.com",
		Timestamp:   time.Now().Add(-time.Hour),
		WaybackTS:   archive.FormatWaybackTS(time.Now().Add(-time.Hour)),
		Title:       "Service",
		HTML:        "<html><body><h1>V1</h1></body></html>",
		ContentHash: "hash1111",
		SizeBytes:   50,
	}
	snap2 := &archive.Snapshot{
		ID:          "snap_vdiff_srv2",
		URL:         "https://service.com",
		Timestamp:   time.Now(),
		WaybackTS:   archive.FormatWaybackTS(time.Now()),
		Title:       "Service",
		HTML:        "<html><body><h1>V2</h1><p>Added</p></body></html>",
		ContentHash: "hash2222",
		SizeBytes:   65,
	}

	_ = store.Save(snap1)
	_ = store.Save(snap2)

	srv := server.NewServer(store, archive.NewArchiver(5*time.Second))

	// Test JSON API /api/diff/visual
	req := httptest.NewRequest(http.MethodGet, "/api/diff/visual?old=snap_vdiff_srv1&new=snap_vdiff_srv2", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from /api/diff/visual, got %d", w.Code)
	}

	var jsonRes visualdiff.VisualDiffResult
	if err := json.NewDecoder(w.Body).Decode(&jsonRes); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
	if jsonRes.AddedNodes == 0 {
		t.Errorf("expected added nodes in json response")
	}

	// Test HTML Route /vdiff/snap_vdiff_srv1/snap_vdiff_srv2
	reqHTML := httptest.NewRequest(http.MethodGet, "/vdiff/snap_vdiff_srv1/snap_vdiff_srv2", nil)
	wHTML := httptest.NewRecorder()
	srv.ServeHTTP(wHTML, reqHTML)

	if wHTML.Code != http.StatusOK {
		t.Fatalf("expected 200 from /vdiff/, got %d", wHTML.Code)
	}
	if !strings.Contains(wHTML.Body.String(), "Temporal Visual & DOM Tree Diff") {
		t.Errorf("expected HTML diff output to contain page title")
	}
}
