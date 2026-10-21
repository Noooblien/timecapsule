package tests

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Noooblien/timecapsule/pkg/archive"
	"github.com/Noooblien/timecapsule/pkg/attest"
	"github.com/Noooblien/timecapsule/pkg/storage"
)

func TestAttest_EndToEndDemoSuite(t *testing.T) {
	summary, err := attest.RunDemoSuite()
	if err != nil {
		t.Fatalf("RunDemoSuite failed: %v", err)
	}

	if summary.TotalEvaluated != 4 {
		t.Fatalf("Expected 4 evaluated snapshots, got %d", summary.TotalEvaluated)
	}

	if summary.VerifiedCount != 1 {
		t.Errorf("Expected 1 authentic verified snapshot, got %d", summary.VerifiedCount)
	}

	if summary.TamperCount != 2 {
		t.Errorf("Expected 2 tampering breaches, got %d", summary.TamperCount)
	}

	if summary.InvalidSignatureCount != 1 {
		t.Errorf("Expected 1 invalid signature, got %d", summary.InvalidSignatureCount)
	}

	scorecard := attest.RenderAttestationScorecard(summary)
	if !strings.Contains(scorecard, "TIMECAPSULE CRYPTOGRAPHIC PROVENANCE") {
		t.Errorf("scorecard missing header")
	}

	md := attest.RenderAttestationMarkdown(summary)
	if !strings.Contains(md, "Executive Summary") {
		t.Errorf("markdown missing Executive Summary")
	}
}

func TestAttest_StorageIntegration(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "timecapsule-attest-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := storage.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	snap := &archive.Snapshot{
		ID:          "tc_storage_snap_01",
		URL:         "https://governance.org/constitution",
		Timestamp:   time.Now(),
		WaybackTS:   "20261021140000",
		Title:       "Governance Constitution",
		StatusCode:  200,
		ContentType: "text/html",
		Headers:     map[string]string{"ETag": "w/12345"},
		HTML:        "<html><body><p>Article 1: Immutable preservation.</p></body></html>",
		Markdown:    "Article 1: Immutable preservation.",
		ContentHash: "hash-const-01",
		SizeBytes:   1024,
		AssetCount:  0,
	}

	if err := store.Save(snap); err != nil {
		t.Fatalf("failed to save snapshot: %v", err)
	}

	// Sign with authority
	auth, err := attest.NewAuthority("notary-test-vault")
	if err != nil {
		t.Fatalf("failed to initialize authority: %v", err)
	}

	receipt, err := auth.SignSnapshot(snap)
	if err != nil {
		t.Fatalf("failed to sign snapshot: %v", err)
	}

	// 1. Initial verification against clean stored snapshot
	loadedSnap, exists := store.Get(snap.ID)
	if !exists {
		t.Fatalf("failed to reload snapshot: not found")
	}

	resClean := attest.VerifySnapshot(receipt, loadedSnap)
	if resClean.Status != attest.StatusVerified {
		t.Fatalf("expected StatusVerified on stored snapshot, got %s", resClean.Status)
	}

	// 2. Tamper stored snapshot in memory
	loadedSnap.HTML = "<html><body><p>Article 1: Altered illegally.</p></body></html>"
	resTampered := attest.VerifySnapshot(receipt, loadedSnap)
	if resTampered.Status != attest.StatusTamperDetected {
		t.Fatalf("expected StatusTamperDetected after HTML alteration, got %s", resTampered.Status)
	}
	if len(resTampered.MismatchedLeaves) == 0 {
		t.Fatalf("expected mismatched leaves list to be non-empty")
	}
}
