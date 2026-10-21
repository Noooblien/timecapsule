package attest

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Noooblien/timecapsule/pkg/archive"
)

// AttestationAuthority manages Ed25519 signing keys and Merkle tree verification.
type AttestationAuthority struct {
	AuthorityID string
	PublicKey   ed25519.PublicKey
	privateKey  ed25519.PrivateKey
}

// NewAuthority generates a fresh Ed25519 keypair for the specified authority ID.
func NewAuthority(authorityID string) (*AttestationAuthority, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ed25519 key: %w", err)
	}
	return &AttestationAuthority{
		AuthorityID: authorityID,
		PublicKey:   pub,
		privateKey:  priv,
	}, nil
}

// NewAuthorityWithKey constructs an authority using an existing Ed25519 private key.
func NewAuthorityWithKey(authorityID string, priv ed25519.PrivateKey) *AttestationAuthority {
	return &AttestationAuthority{
		AuthorityID: authorityID,
		PublicKey:   priv.Public().(ed25519.PublicKey),
		privateKey:  priv,
	}
}

// BuildMerkleTree calculates the 5-component Merkle tree for a captured web snapshot.
func BuildMerkleTree(snap *archive.Snapshot) *MerkleTree {
	leaves := make([]MerkleLeaf, 5)

	// Leaf 0: Canonical HTTP Metadata
	headersCopy := make(map[string]string)
	for k, v := range snap.Headers {
		headersCopy[k] = v
	}
	// Sort header keys for canonical serialization
	type metaCanonical struct {
		URL         string            `json:"url"`
		StatusCode  int               `json:"status_code"`
		ContentType string            `json:"content_type"`
		Headers     map[string]string `json:"headers"`
	}
	metaBytes, _ := json.Marshal(metaCanonical{
		URL:         snap.URL,
		StatusCode:  snap.StatusCode,
		ContentType: snap.ContentType,
		Headers:     headersCopy,
	})
	h0 := sha256Hash(metaBytes)
	leaves[0] = MerkleLeaf{
		Type:        LeafHTTPMetadata,
		Hash:        h0,
		Description: fmt.Sprintf("HTTP %d %s (%d headers)", snap.StatusCode, snap.ContentType, len(snap.Headers)),
		ByteLength:  len(metaBytes),
	}

	// Leaf 1: Raw Verbatim HTML DOM
	htmlBytes := []byte(snap.HTML)
	h1 := sha256Hash(htmlBytes)
	leaves[1] = MerkleLeaf{
		Type:        LeafHTMLDOM,
		Hash:        h1,
		Description: fmt.Sprintf("Verbatim HTML DOM (%d bytes)", len(htmlBytes)),
		ByteLength:  len(htmlBytes),
	}

	// Leaf 2: Reader Markdown Content
	mdBytes := []byte(snap.Markdown)
	h2 := sha256Hash(mdBytes)
	leaves[2] = MerkleLeaf{
		Type:        LeafReaderMarkdown,
		Hash:        h2,
		Description: fmt.Sprintf("Clean Reader Markdown (%d bytes)", len(mdBytes)),
		ByteLength:  len(mdBytes),
	}

	// Leaf 3: Archive Metadata & Fingerprints
	type manifestCanonical struct {
		ContentHash string `json:"content_hash"`
		SizeBytes   int64  `json:"size_bytes"`
		WaybackTS   string `json:"wayback_timestamp"`
	}
	manifestBytes, _ := json.Marshal(manifestCanonical{
		ContentHash: snap.ContentHash,
		SizeBytes:   snap.SizeBytes,
		WaybackTS:   snap.WaybackTS,
	})
	h3 := sha256Hash(manifestBytes)
	leaves[3] = MerkleLeaf{
		Type:        LeafArchiveManifest,
		Hash:        h3,
		Description: fmt.Sprintf("Archive Envelope (Hash: %s, %d bytes)", snap.ContentHash, snap.SizeBytes),
		ByteLength:  len(manifestBytes),
	}

	// Leaf 4: Asset Manifest
	type assetCanonical struct {
		AssetCount int `json:"asset_count"`
	}
	assetBytes, _ := json.Marshal(assetCanonical{AssetCount: snap.AssetCount})
	h4 := sha256Hash(assetBytes)
	leaves[4] = MerkleLeaf{
		Type:        LeafAssetManifest,
		Hash:        h4,
		Description: fmt.Sprintf("Asset Manifest (%d bundled assets)", snap.AssetCount),
		ByteLength:  len(assetBytes),
	}

	// Compute Merkle Root via hierarchical pairwise reduction
	rootHash, intermediates := computeMerkleRoot(leaves)

	return &MerkleTree{
		Leaves:             leaves,
		IntermediateHashes: intermediates,
		RootHash:           rootHash,
	}
}

// SignSnapshot constructs the Merkle tree and issues an Ed25519-signed AttestationReceipt.
func (a *AttestationAuthority) SignSnapshot(snap *archive.Snapshot) (*AttestationReceipt, error) {
	if a.privateKey == nil {
		return nil, fmt.Errorf("authority private key is not configured for signing")
	}

	tree := BuildMerkleTree(snap)
	nonce := generateNonce()
	attestedAt := time.Now().UTC()

	canonicalPayload := fmt.Sprintf("TIMECAPSULE-ATTESTATION:v1:%s:%s:%s:%s",
		snap.ID, tree.RootHash, nonce, attestedAt.Format(time.RFC3339))

	sig := ed25519.Sign(a.privateKey, []byte(canonicalPayload))

	receiptID := fmt.Sprintf("rcpt_%s_%s", snap.ID, nonce[:8])

	return &AttestationReceipt{
		ReceiptID:          receiptID,
		SnapshotID:         snap.ID,
		URL:                snap.URL,
		ArchivedAt:         snap.Timestamp,
		AttestedAt:         attestedAt,
		AuthorityID:        a.AuthorityID,
		AuthorityPublicKey: hex.EncodeToString(a.PublicKey),
		MerkleRoot:         tree.RootHash,
		Signature:          hex.EncodeToString(sig),
		Nonce:              nonce,
		Leaves:             tree.Leaves,
	}, nil
}

// VerifySnapshot checks the cryptographic signature and verifies zero tampering across all Merkle leaves.
func VerifySnapshot(receipt *AttestationReceipt, snap *archive.Snapshot) *VerificationResult {
	start := time.Now()

	result := &VerificationResult{
		ReceiptID:        receipt.ReceiptID,
		SnapshotID:       snap.ID,
		URL:              snap.URL,
		VerifiedAt:       time.Now().UTC(),
		MismatchedLeaves: make([]string, 0),
	}

	// 1. Verify Ed25519 Digital Signature
	pubKeyBytes, err := hex.DecodeString(receipt.AuthorityPublicKey)
	if err != nil || len(pubKeyBytes) != ed25519.PublicKeySize {
		result.Status = StatusSignatureInvalid
		result.Summary = "Corrupted or invalid Ed25519 public key format."
		result.LatencyMs = float64(time.Since(start).Microseconds()) / 1000.0
		return result
	}

	sigBytes, err := hex.DecodeString(receipt.Signature)
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		result.Status = StatusSignatureInvalid
		result.Summary = "Corrupted or invalid Ed25519 signature format."
		result.LatencyMs = float64(time.Since(start).Microseconds()) / 1000.0
		return result
	}

	canonicalPayload := fmt.Sprintf("TIMECAPSULE-ATTESTATION:v1:%s:%s:%s:%s",
		snap.ID, receipt.MerkleRoot, receipt.Nonce, receipt.AttestedAt.Format(time.RFC3339))

	if !ed25519.Verify(ed25519.PublicKey(pubKeyBytes), []byte(canonicalPayload), sigBytes) {
		result.Status = StatusSignatureInvalid
		result.SignatureValid = false
		result.Summary = "Ed25519 signature verification failed. Possible certificate forgery."
		result.LatencyMs = float64(time.Since(start).Microseconds()) / 1000.0
		return result
	}
	result.SignatureValid = true

	// 2. Recompute Merkle Tree from current snapshot
	liveTree := BuildMerkleTree(snap)

	// 3. Inspect individual leaf parity for forensic tamper localization
	leafMap := make(map[MerkleLeafType]string)
	for _, l := range receipt.Leaves {
		leafMap[l.Type] = l.Hash
	}

	for _, liveLeaf := range liveTree.Leaves {
		expectedHash, exists := leafMap[liveLeaf.Type]
		if !exists || expectedHash != liveLeaf.Hash {
			diffMsg := fmt.Sprintf("Leaf [%s] modified! Expected %s, computed %s",
				liveLeaf.Type, truncateHash(expectedHash), truncateHash(liveLeaf.Hash))
			result.MismatchedLeaves = append(result.MismatchedLeaves, diffMsg)
		}
	}

	if liveTree.RootHash != receipt.MerkleRoot || len(result.MismatchedLeaves) > 0 {
		result.Status = StatusTamperDetected
		result.RootMatches = false
		result.Summary = fmt.Sprintf("Tamper detected! Merkle root mismatch across %d component leaf nodes.", len(result.MismatchedLeaves))
	} else {
		result.Status = StatusVerified
		result.RootMatches = true
		result.Summary = "Snapshot is 100% authentic. Ed25519 signature valid and Merkle tree verified."
	}

	result.LatencyMs = float64(time.Since(start).Microseconds()) / 1000.0
	return result
}

func computeMerkleRoot(leaves []MerkleLeaf) (string, []string) {
	currentLevel := make([]string, len(leaves))
	for i, l := range leaves {
		currentLevel[i] = l.Hash
	}

	var intermediates []string

	for len(currentLevel) > 1 {
		// If odd count, duplicate last element
		if len(currentLevel)%2 != 0 {
			currentLevel = append(currentLevel, currentLevel[len(currentLevel)-1])
		}

		nextLevel := make([]string, len(currentLevel)/2)
		for i := 0; i < len(currentLevel); i += 2 {
			combined := currentLevel[i] + currentLevel[i+1]
			h := sha256Hash([]byte(combined))
			nextLevel[i/2] = h
			intermediates = append(intermediates, h)
		}
		currentLevel = nextLevel
	}

	return currentLevel[0], intermediates
}

func sha256Hash(b []byte) string {
	h := sha256.New()
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}

func generateNonce() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func truncateHash(h string) string {
	if len(h) <= 12 {
		return h
	}
	return h[:6] + "..." + h[len(h)-6:]
}
