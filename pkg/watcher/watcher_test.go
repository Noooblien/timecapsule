package watcher

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Noooblien/timecapsule/pkg/archive"
	"github.com/Noooblien/timecapsule/pkg/storage"
)

func TestSanitization_NoiseAndNonces(t *testing.T) {
	rawHTML := `<!DOCTYPE html><html><head><title>Test</title>
	<script>var x = 123;</script>
	<style>.hide { display: none; }</style>
	</head><body>
	<!-- secret internal comment -->
	<h1>Document Title</h1>
	<p>Published at 2026-10-18T14:50:00Z by Admin.</p>
	<input type="hidden" name="csrf_token" value="nonce_abc123xyz" />
	<div class="ad-banner"><p>Buy something now!</p></div>
	<p>Meaningful stable content.</p>
	</body></html>`

	clean := SanitizeContent(rawHTML, []string{"ad-banner"})

	if strings.Contains(clean, "secret internal comment") {
		t.Errorf("expected HTML comment to be stripped")
	}
	if strings.Contains(clean, "var x = 123") {
		t.Errorf("expected script to be stripped")
	}
	if strings.Contains(clean, "Buy something now") {
		t.Errorf("expected ad-banner to be stripped via ignoreSelectors")
	}
	if strings.Contains(clean, "nonce_abc123xyz") {
		t.Errorf("expected csrf_token nonce to be normalized")
	}
	if !strings.Contains(clean, "Meaningful stable content") {
		t.Errorf("expected stable content to be preserved")
	}
}

func TestTokenSimilarity(t *testing.T) {
	s1 := "The quick brown fox jumps over the lazy dog"
	s2 := "The quick brown fox jumps over the lazy dog"
	if sim := ComputeTokenSimilarity(s1, s2); sim != 1.0 {
		t.Errorf("expected similarity 1.0 for identical strings, got %f", sim)
	}

	s3 := "The quick brown fox leaps over the lazy dog"
	if sim := ComputeTokenSimilarity(s1, s3); sim < 0.80 || sim >= 1.0 {
		t.Errorf("expected similarity ~0.89 for single word change, got %f", sim)
	}

	s4 := "Completely unrelated text with zero overlap"
	if sim := ComputeTokenSimilarity(s1, s4); sim > 0.20 {
		t.Errorf("expected low similarity for disjoint strings, got %f", sim)
	}
}

func TestClassifyDrift(t *testing.T) {
	oldT := "Welcome to our platform. All users must adhere to terms."
	newTIdentical := "Welcome to our platform. All users must adhere to terms."

	sev, delta, _ := ClassifyDrift(oldT, newTIdentical, 200, 0.05)
	if sev != DriftNone || delta != 0.0 {
		t.Errorf("expected DriftNone, got %s, delta: %f", sev, delta)
	}

	newTCosmetic := "Welcome to our platform. All users must adhere to terms!"
	sev, _, _ = ClassifyDrift(oldT, newTCosmetic, 200, 0.00)
	if sev != DriftCosmetic {
		t.Errorf("expected DriftCosmetic, got %s", sev)
	}

	newTSubstantial := "Welcome to our platform. All users must submit arbitration claims in private proceedings."
	sev, _, _ = ClassifyDrift(oldT, newTSubstantial, 200, 0.05)
	if sev != DriftSubstantial {
		t.Errorf("expected DriftSubstantial, got %s", sev)
	}

	sev, _, _ = ClassifyDrift(oldT, "", 503, 0.05)
	if sev != DriftOutage {
		t.Errorf("expected DriftOutage for HTTP 503, got %s", sev)
	}
}

func TestEngine_TargetLifecycleAndPersistence(t *testing.T) {
	tempDir := t.TempDir()
	store, _ := storage.NewStore(tempDir)
	archiver := archive.NewArchiver(5 * time.Second)

	engine, err := NewEngine(store, archiver, tempDir)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	// Add targets
	t1, err := engine.AddTarget("https://example.com/terms", "Terms", "10m", "https://hook.example.com", 0.05, []string{"ads"}, []string{"legal"})
	if err != nil {
		t.Fatalf("failed to add target: %v", err)
	}

	if t1.ID == "" || t1.Status != StatusActive {
		t.Errorf("invalid target created: %+v", t1)
	}

	targets := engine.ListTargets()
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}

	// Reload engine from file to verify persistence
	engine2, err := NewEngine(store, archiver, tempDir)
	if err != nil {
		t.Fatalf("failed to reload engine: %v", err)
	}

	tReloaded, ok := engine2.GetTarget(t1.ID)
	if !ok {
		t.Fatalf("target %s not found in reloaded engine", t1.ID)
	}
	if tReloaded.Name != "Terms" || tReloaded.IntervalStr != "10m" {
		t.Errorf("unexpected reloaded target fields: %+v", tReloaded)
	}

	// Remove target
	if err := engine2.RemoveTarget(t1.ID); err != nil {
		t.Fatalf("failed to remove target: %v", err)
	}
	if len(engine2.ListTargets()) != 0 {
		t.Errorf("expected 0 targets after removal")
	}
}

func TestEngine_DriftDetectionAndWebhook(t *testing.T) {
	tempDir := t.TempDir()
	store, _ := storage.NewStore(tempDir)
	archiver := archive.NewArchiver(5 * time.Second)

	var webhookCount int32
	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&webhookCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	var pageContent string
	contentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, pageContent)
	}))
	defer contentServer.Close()

	engine, err := NewEngine(store, archiver, tempDir)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	target, err := engine.AddTarget(contentServer.URL, "Test Target", "1m", webhookServer.URL, 0.05, nil, nil)
	if err != nil {
		t.Fatalf("failed to add target: %v", err)
	}

	ctx := context.Background()

	// 1. Initial capture (baseline establishment)
	pageContent = "<html><body><h1>Version 1.0</h1><p>Initial pristine state.</p></body></html>"
	ev, err := engine.PollTarget(ctx, target.ID)
	if err != nil {
		t.Fatalf("initial poll failed: %v", err)
	}
	if ev != nil {
		t.Errorf("expected nil ChangeEvent on initial baseline capture, got %+v", ev)
	}
	if target.CheckCount != 1 || target.ChangeCount != 0 {
		t.Errorf("expected checkCount=1, changeCount=0, got %d, %d", target.CheckCount, target.ChangeCount)
	}

	// 2. Poll with unchanged content (no drift)
	ev, err = engine.PollTarget(ctx, target.ID)
	if err != nil {
		t.Fatalf("second poll failed: %v", err)
	}
	if ev != nil {
		t.Errorf("expected nil ChangeEvent on identical poll, got %+v", ev)
	}

	// 3. Poll with mutated content (drift detected!)
	pageContent = "<html><body><h1>Version 2.0</h1><p>Major update with breaking changes across all API endpoints.</p></body></html>"
	ev, err = engine.PollTarget(ctx, target.ID)
	if err != nil {
		t.Fatalf("drift poll failed: %v", err)
	}
	if ev == nil {
		t.Fatalf("expected non-nil ChangeEvent when content drifted")
	}

	if ev.Severity != DriftSubstantial && ev.Severity != DriftStructural {
		t.Errorf("expected Substantial/Structural severity, got %s", ev.Severity)
	}
	if ev.LinesAdded == 0 && ev.LinesDeleted == 0 {
		t.Errorf("expected lines added/deleted in diff, got added: %d, deleted: %d", ev.LinesAdded, ev.LinesDeleted)
	}

	if atomic.LoadInt32(&webhookCount) != 1 {
		t.Errorf("expected webhook to be invoked once, got %d", atomic.LoadInt32(&webhookCount))
	}

	if target.ChangeCount != 1 {
		t.Errorf("expected target.ChangeCount=1, got %d", target.ChangeCount)
	}
}

func TestScorecardAndReports(t *testing.T) {
	targets := []*WatchTarget{
		{
			ID:          "wt_1",
			Name:        "ToS",
			URL:         "https://example.com/terms",
			IntervalStr: "5m",
			Status:      StatusActive,
			CheckCount:  12,
			ChangeCount: 2,
			LastChange:  time.Now().Add(-15 * time.Minute),
		},
	}

	events := []ChangeEvent{
		{
			ID:           "chg_1",
			TargetName:   "ToS",
			TargetURL:    "https://example.com/terms",
			Timestamp:    time.Now().Add(-15 * time.Minute),
			Severity:     DriftSubstantial,
			DeltaRatio:   0.18,
			Summary:      "Substantial editorial update",
			LinesAdded:   4,
			LinesDeleted: 2,
		},
	}

	scorecard := RenderWatcherScorecard(targets, events)
	if !strings.Contains(scorecard, "TIMECAPSULE CONTINUOUS DRIFT WATCHER") {
		t.Errorf("scorecard missing header")
	}
	if !strings.Contains(scorecard, "ACTIVE") {
		t.Errorf("scorecard missing ACTIVE status badge")
	}

	md := RenderMarkdownReport(targets, events)
	if !strings.Contains(md, "# TimeCapsule Continuous Drift Watcher Report") {
		t.Errorf("markdown report missing header")
	}

	timeline := RenderMermaidTimeline(events)
	if !strings.Contains(timeline, "timeline") {
		t.Errorf("timeline missing mermaid directive")
	}
}
