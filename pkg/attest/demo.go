package attest

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/Noooblien/timecapsule/pkg/archive"
)

// RunDemoSuite executes 4 production scenarios demonstrating cryptographic provenance and tamper detection.
func RunDemoSuite() (AttestationSummary, error) {
	authority, err := NewAuthority("timecapsule-notary-us-east")
	if err != nil {
		return AttestationSummary{}, fmt.Errorf("failed to initialize notary authority: %w", err)
	}

	var results []VerificationResult

	// -------------------------------------------------------------------------
	// Scenario 1: Authentic Legal Web Archive (100% Verified)
	// -------------------------------------------------------------------------
	snap1 := &archive.Snapshot{
		ID:          "tc_20261021_legal_01",
		URL:         "https://compliance.enterprise.com/terms-of-service",
		Timestamp:   time.Now().Add(-2 * time.Hour),
		WaybackTS:   "20261021133000",
		Title:       "Enterprise Cloud Terms of Service",
		StatusCode:  200,
		ContentType: "text/html; charset=utf-8",
		Headers: map[string]string{
			"Server":        "cloudflare",
			"Cache-Control": "public, max-age=3600",
		},
		HTML:        "<!DOCTYPE html><html><body><h1>Terms of Service</h1><p>Enterprise SLA guarantee is 99.99%.</p></body></html>",
		Markdown:    "# Terms of Service\n\nEnterprise SLA guarantee is 99.99%.",
		ContentHash: "a1b2c3d4e5f67890123456789abcdef0123456789abcdef0123456789abcdef0",
		SizeBytes:   4096,
		AssetCount:  2,
	}

	receipt1, err := authority.SignSnapshot(snap1)
	if err != nil {
		return AttestationSummary{}, err
	}
	res1 := VerifySnapshot(receipt1, snap1)
	results = append(results, *res1)

	// -------------------------------------------------------------------------
	// Scenario 2: Stealth HTML DOM Tampering Attack
	// -------------------------------------------------------------------------
	snap2 := &archive.Snapshot{
		ID:          "tc_20261021_pricing_02",
		URL:         "https://saas.enterprise.com/pricing-contract",
		Timestamp:   time.Now().Add(-4 * time.Hour),
		WaybackTS:   "20261021113000",
		Title:       "Enterprise Tier Pricing Agreement",
		StatusCode:  200,
		ContentType: "text/html; charset=utf-8",
		Headers: map[string]string{
			"Server": "nginx/1.24",
		},
		HTML:        "<!DOCTYPE html><html><body><div id='price'>Annual license fee: $500,000 USD.</div></body></html>",
		Markdown:    "Annual license fee: $500,000 USD.",
		ContentHash: "b2c3d4e5f67890123456789abcdef0123456789abcdef0123456789abcdef01",
		SizeBytes:   2048,
		AssetCount:  1,
	}

	receipt2, err := authority.SignSnapshot(snap2)
	if err != nil {
		return AttestationSummary{}, err
	}

	// Adversary tampers with the archived HTML: "$500,000" -> "$50,000"
	snap2Tampered := *snap2
	snap2Tampered.HTML = "<!DOCTYPE html><html><body><div id='price'>Annual license fee: $50,000 USD.</div></body></html>"

	res2 := VerifySnapshot(receipt2, &snap2Tampered)
	results = append(results, *res2)

	// -------------------------------------------------------------------------
	// Scenario 3: Header & Status Code Manipulation
	// -------------------------------------------------------------------------
	snap3 := &archive.Snapshot{
		ID:          "tc_20261021_endpoint_03",
		URL:         "https://api.partner.com/v1/auth/keys",
		Timestamp:   time.Now().Add(-6 * time.Hour),
		WaybackTS:   "20261021093000",
		Title:       "OAuth Key Distribution Endpoint",
		StatusCode:  404,
		ContentType: "application/json",
		Headers: map[string]string{
			"X-RateLimit-Remaining": "0",
		},
		HTML:        `{"error": "Endpoint decommissioned"}`,
		Markdown:    "Endpoint decommissioned",
		ContentHash: "c3d4e5f67890123456789abcdef0123456789abcdef0123456789abcdef012",
		SizeBytes:   512,
		AssetCount:  0,
	}

	receipt3, err := authority.SignSnapshot(snap3)
	if err != nil {
		return AttestationSummary{}, err
	}

	// Adversary forges status code from 404 -> 200
	snap3Tampered := *snap3
	snap3Tampered.StatusCode = 200

	res3 := VerifySnapshot(receipt3, &snap3Tampered)
	results = append(results, *res3)

	// -------------------------------------------------------------------------
	// Scenario 4: Forged Attestation Signature
	// -------------------------------------------------------------------------
	snap4 := &archive.Snapshot{
		ID:          "tc_20261021_forged_04",
		URL:         "https://unverified.com/leaked-memo",
		Timestamp:   time.Now().Add(-8 * time.Hour),
		WaybackTS:   "20261021073000",
		Title:       "Internal Memo",
		StatusCode:  200,
		ContentType: "text/html",
		Headers:     map[string]string{},
		HTML:        "<p>Confidential memo</p>",
		Markdown:    "Confidential memo",
		ContentHash: "d4e5f67890123456789abcdef0123456789abcdef0123456789abcdef0123",
		SizeBytes:   1024,
		AssetCount:  0,
	}

	// Sign with a rogue alien keypair
	_, roguePriv, _ := ed25519.GenerateKey(rand.Reader)
	rogueAuth := NewAuthorityWithKey("rogue-forger", roguePriv)
	forgedReceipt, err := rogueAuth.SignSnapshot(snap4)
	if err != nil {
		return AttestationSummary{}, err
	}

	// Claim to be signed by official authority public key
	forgedReceipt.AuthorityPublicKey = hex.EncodeToString(authority.PublicKey)

	res4 := VerifySnapshot(forgedReceipt, snap4)
	results = append(results, *res4)

	// Summarize
	summary := AttestationSummary{
		TotalEvaluated: len(results),
		Results:        results,
	}
	for _, r := range results {
		switch r.Status {
		case StatusVerified:
			summary.VerifiedCount++
		case StatusTamperDetected:
			summary.TamperCount++
		case StatusSignatureInvalid:
			summary.InvalidSignatureCount++
		}
	}

	return summary, nil
}
