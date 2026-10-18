package tests

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Noooblien/timecapsule/pkg/archive"
	"github.com/Noooblien/timecapsule/pkg/storage"
	"github.com/Noooblien/timecapsule/pkg/watcher"
)

func TestWatcher_EndToEndLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	store, err := storage.NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	currentHTML := "<html><body><h1>Policy v1</h1><p>We respect customer privacy.</p></body></html>"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, currentHTML)
	}))
	defer server.Close()

	archiver := archive.NewArchiver(5 * time.Second)
	engine, err := watcher.NewEngine(store, archiver, tempDir)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	target, err := engine.AddTarget(server.URL, "Privacy Policy", "1m", "", 0.05, nil, []string{"legal"})
	if err != nil {
		t.Fatalf("failed to add target: %v", err)
	}

	ctx := context.Background()

	// Initial poll: establishes baseline
	ev, err := engine.PollTarget(ctx, target.ID)
	if err != nil {
		t.Fatalf("initial poll error: %v", err)
	}
	if ev != nil {
		t.Errorf("expected no drift event on initial baseline")
	}

	history := store.GetHistory(target.URL)
	if len(history) != 1 {
		t.Fatalf("expected 1 snapshot in storage history, got %d", len(history))
	}

	// Second poll: no changes
	ev, err = engine.PollTarget(ctx, target.ID)
	if err != nil || ev != nil {
		t.Fatalf("unexpected drift on identical poll: %v", err)
	}
	if len(store.GetHistory(target.URL)) != 1 {
		t.Errorf("storage history should still be 1 after identical poll")
	}

	// Third poll: content drifts significantly
	currentHTML = "<html><body><h1>Policy v2</h1><p>We monetize customer telemetry with third-party partners.</p></body></html>"
	ev, err = engine.PollTarget(ctx, target.ID)
	if err != nil {
		t.Fatalf("drift poll error: %v", err)
	}
	if ev == nil {
		t.Fatalf("expected drift event to be recorded")
	}

	if ev.Severity != watcher.DriftSubstantial && ev.Severity != watcher.DriftStructural {
		t.Errorf("expected substantial/structural severity, got %s", ev.Severity)
	}

	history = store.GetHistory(target.URL)
	if len(history) != 2 {
		t.Fatalf("expected 2 snapshots in storage history after drift, got %d", len(history))
	}

	events := engine.GetEvents(target.ID)
	if len(events) != 1 {
		t.Fatalf("expected 1 event recorded for target, got %d", len(events))
	}

	scorecard := watcher.RenderWatcherScorecard(engine.ListTargets(), events)
	if !strings.Contains(scorecard, "Privacy Policy") {
		t.Errorf("scorecard missing target name")
	}
}

func TestWatcher_DemoSuite(t *testing.T) {
	// Terminal output
	termOutput, err := watcher.RunWatcherDemo(false, false)
	if err != nil {
		t.Fatalf("RunWatcherDemo failed: %v", err)
	}
	if !strings.Contains(termOutput, "TIMECAPSULE CONTINUOUS DRIFT WATCHER") {
		t.Errorf("demo terminal output missing header")
	}
	if !strings.Contains(termOutput, "Terms of Service") {
		t.Errorf("demo terminal output missing Terms of Service target")
	}

	// Mermaid output
	mermaidOutput, err := watcher.RunWatcherDemo(true, false)
	if err != nil {
		t.Fatalf("RunWatcherDemo mermaid failed: %v", err)
	}
	if !strings.Contains(mermaidOutput, "timeline") {
		t.Errorf("demo mermaid output missing timeline directive")
	}

	// Markdown output
	mdOutput, err := watcher.RunWatcherDemo(false, true)
	if err != nil {
		t.Fatalf("RunWatcherDemo markdown failed: %v", err)
	}
	if !strings.Contains(mdOutput, "# TimeCapsule Continuous Drift Watcher Report") {
		t.Errorf("demo markdown output missing header")
	}
}
