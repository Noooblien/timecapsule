package attest

import (
	"crypto/ed25519"
	"strings"
	"testing"
	"time"

	"github.com/Noooblien/timecapsule/pkg/archive"
)

func newSampleSnapshot() *archive.Snapshot {
	return &archive.Snapshot{
		ID:          "snap-test-01",
		URL:         "https://example.com/test",
		Timestamp:   time.Now(),
		WaybackTS:   "20261021120000",
		Title:       "Sample Test Page",
		StatusCode:  200,
		ContentType: "text/html",
		Headers:     map[string]string{"Server": "caddy"},
		HTML:        "<html><body><h1>Hello World</h1></body></html>",
		Markdown:    "# Hello World",
		ContentHash: "hash123",
		SizeBytes:   1024,
		AssetCount:  1,
	}
}

func TestAuthorityKeyGeneration(t *testing.T) {
	auth, err := NewAuthority("test-notary")
	if err != nil {
		t.Fatalf("Failed to generate authority: %v", err)
	}
	if len(auth.PublicKey) != ed25519.PublicKeySize {
		t.Fatalf("Expected public key size %d, got %d", ed25519.PublicKeySize, len(auth.PublicKey))
	}
	if len(auth.privateKey) != ed25519.PrivateKeySize {
		t.Fatalf("Expected private key size %d, got %d", ed25519.PrivateKeySize, len(auth.privateKey))
	}
}

func TestBuildMerkleTree(t *testing.T) {
	snap := newSampleSnapshot()
	tree := BuildMerkleTree(snap)

	if len(tree.Leaves) != 5 {
		t.Fatalf("Expected 5 Merkle leaves, got %d", len(tree.Leaves))
	}
	if tree.RootHash == "" {
		t.Fatalf("Expected non-empty Merkle root hash")
	}
	if len(tree.IntermediateHashes) == 0 {
		t.Fatalf("Expected intermediate hashes in Merkle tree")
	}
}

func TestSignAndVerify_Authentic(t *testing.T) {
	auth, _ := NewAuthority("test-notary")
	snap := newSampleSnapshot()

	receipt, err := auth.SignSnapshot(snap)
	if err != nil {
		t.Fatalf("Failed to sign snapshot: %v", err)
	}

	res := VerifySnapshot(receipt, snap)
	if res.Status != StatusVerified {
		t.Fatalf("Expected StatusVerified for authentic snapshot, got %s (%s)", res.Status, res.Summary)
	}
	if !res.SignatureValid {
		t.Fatalf("Expected SignatureValid == true")
	}
	if !res.RootMatches {
		t.Fatalf("Expected RootMatches == true")
	}
	if len(res.MismatchedLeaves) > 0 {
		t.Fatalf("Expected zero mismatched leaves for authentic snapshot, got %v", res.MismatchedLeaves)
	}
}

func TestTamperDetection_HTML(t *testing.T) {
	auth, _ := NewAuthority("test-notary")
	snap := newSampleSnapshot()

	receipt, _ := auth.SignSnapshot(snap)

	// Tamper HTML DOM
	tamperedSnap := *snap
	tamperedSnap.HTML = "<html><body><h1>Malicious Injected Content</h1></body></html>"

	res := VerifySnapshot(receipt, &tamperedSnap)
	if res.Status != StatusTamperDetected {
		t.Fatalf("Expected StatusTamperDetected for tampered HTML, got %s", res.Status)
	}
	if res.RootMatches {
		t.Fatalf("RootMatches should be false on tampered HTML")
	}

	foundLeaf := false
	for _, m := range res.MismatchedLeaves {
		if strings.Contains(m, string(LeafHTMLDOM)) {
			foundLeaf = true
			break
		}
	}
	if !foundLeaf {
		t.Fatalf("Expected LeafHTMLDOM to be identified in mismatched leaves: %v", res.MismatchedLeaves)
	}
}

func TestTamperDetection_Metadata(t *testing.T) {
	auth, _ := NewAuthority("test-notary")
	snap := newSampleSnapshot()

	receipt, _ := auth.SignSnapshot(snap)

	// Tamper status code
	tamperedSnap := *snap
	tamperedSnap.StatusCode = 403

	res := VerifySnapshot(receipt, &tamperedSnap)
	if res.Status != StatusTamperDetected {
		t.Fatalf("Expected StatusTamperDetected for tampered metadata, got %s", res.Status)
	}

	foundLeaf := false
	for _, m := range res.MismatchedLeaves {
		if strings.Contains(m, string(LeafHTTPMetadata)) {
			foundLeaf = true
			break
		}
	}
	if !foundLeaf {
		t.Fatalf("Expected LeafHTTPMetadata to be identified in mismatched leaves: %v", res.MismatchedLeaves)
	}
}

func TestTamperDetection_Markdown(t *testing.T) {
	auth, _ := NewAuthority("test-notary")
	snap := newSampleSnapshot()

	receipt, _ := auth.SignSnapshot(snap)

	// Tamper markdown
	tamperedSnap := *snap
	tamperedSnap.Markdown = "# Altered Text"

	res := VerifySnapshot(receipt, &tamperedSnap)
	if res.Status != StatusTamperDetected {
		t.Fatalf("Expected StatusTamperDetected for tampered markdown, got %s", res.Status)
	}

	foundLeaf := false
	for _, m := range res.MismatchedLeaves {
		if strings.Contains(m, string(LeafReaderMarkdown)) {
			foundLeaf = true
			break
		}
	}
	if !foundLeaf {
		t.Fatalf("Expected LeafReaderMarkdown in mismatched leaves: %v", res.MismatchedLeaves)
	}
}

func TestSignatureInvalid_ForgedKey(t *testing.T) {
	auth, _ := NewAuthority("test-notary")
	snap := newSampleSnapshot()

	receipt, _ := auth.SignSnapshot(snap)

	// Corrupt signature
	sigRunes := []rune(receipt.Signature)
	if sigRunes[0] == 'a' {
		sigRunes[0] = 'b'
	} else {
		sigRunes[0] = 'a'
	}
	receipt.Signature = string(sigRunes)

	res := VerifySnapshot(receipt, snap)
	if res.Status != StatusSignatureInvalid {
		t.Fatalf("Expected StatusSignatureInvalid for forged signature, got %s", res.Status)
	}
	if res.SignatureValid {
		t.Fatalf("SignatureValid should be false")
	}
}

func TestDemoSuiteExecution(t *testing.T) {
	summary, err := RunDemoSuite()
	if err != nil {
		t.Fatalf("RunDemoSuite failed: %v", err)
	}
	if summary.TotalEvaluated != 4 {
		t.Fatalf("Expected 4 evaluated snapshots, got %d", summary.TotalEvaluated)
	}
	if summary.VerifiedCount != 1 {
		t.Errorf("Expected 1 verified authentic snapshot, got %d", summary.VerifiedCount)
	}
	if summary.TamperCount != 2 {
		t.Errorf("Expected 2 tampering breaches, got %d", summary.TamperCount)
	}
	if summary.InvalidSignatureCount != 1 {
		t.Errorf("Expected 1 invalid signature, got %d", summary.InvalidSignatureCount)
	}

	scorecard := RenderAttestationScorecard(summary)
	if !strings.Contains(scorecard, "TIMECAPSULE CRYPTOGRAPHIC PROVENANCE") {
		t.Errorf("Scorecard missing expected title")
	}

	mermaid := RenderMerkleTreeMermaid(&AttestationReceipt{
		MerkleRoot: "abcdef123456",
		Leaves: []MerkleLeaf{
			{Type: LeafHTMLDOM, Hash: "111"},
			{Type: LeafHTTPMetadata, Hash: "222"},
		},
	}, &summary.Results[1])
	if !strings.Contains(mermaid, "flowchart TD") {
		t.Errorf("Mermaid diagram missing flowchart TD")
	}

	md := RenderAttestationMarkdown(summary)
	if !strings.Contains(md, "Executive Summary") {
		t.Errorf("Markdown report missing Executive Summary")
	}
}
