package attest

import "time"

// AttestationStatus denotes the outcome of cryptographic verification on an archived snapshot.
type AttestationStatus string

const (
	StatusVerified         AttestationStatus = "VERIFIED"          // Cryptographically authentic; zero tampering
	StatusTamperDetected   AttestationStatus = "TAMPER_DETECTED"   // Merkle root or leaf hash mismatch detected
	StatusSignatureInvalid AttestationStatus = "SIGNATURE_INVALID" // Ed25519 signature verification failed
	StatusUnattested       AttestationStatus = "UNATTESTED"        // Snapshot lacks cryptographic receipt
)

// MerkleLeafType identifies the specific component of the preserved web document.
type MerkleLeafType string

const (
	LeafHTTPMetadata    MerkleLeafType = "HTTP_METADATA"    // URL, StatusCode, ContentType, Canonical Headers
	LeafHTMLDOM         MerkleLeafType = "HTML_VERBATIM_DOM" // Raw captured HTML DOM
	LeafReaderMarkdown  MerkleLeafType = "READER_MARKDOWN"  // Distraction-free extracted markdown
	LeafArchiveManifest MerkleLeafType = "ARCHIVE_MANIFEST" // ContentHash, SizeBytes, WaybackTS
	LeafAssetManifest   MerkleLeafType = "ASSET_MANIFEST"   // Sub-resource count & assets
)

// MerkleLeaf represents a single authenticated leaf node in the document Merkle tree.
type MerkleLeaf struct {
	Type        MerkleLeafType `json:"type"`
	Hash        string         `json:"hash"`
	Description string         `json:"description"`
	ByteLength  int            `json:"byte_length"`
}

// MerkleTree contains the hierarchical structure and root hash of a preserved snapshot.
type MerkleTree struct {
	Leaves             []MerkleLeaf `json:"leaves"`
	IntermediateHashes []string     `json:"intermediate_hashes"`
	RootHash           string       `json:"root_hash"`
}

// AttestationReceipt is the tamper-evident digital certificate binding a snapshot to an authority.
type AttestationReceipt struct {
	ReceiptID          string       `json:"receipt_id"`
	SnapshotID         string       `json:"snapshot_id"`
	URL                string       `json:"url"`
	ArchivedAt         time.Time    `json:"archived_at"`
	AttestedAt         time.Time    `json:"attested_at"`
	AuthorityID        string       `json:"authority_id"`
	AuthorityPublicKey string       `json:"authority_public_key"` // Hex-encoded Ed25519 public key
	MerkleRoot         string       `json:"merkle_root"`          // SHA-256 Merkle root hash
	Signature          string       `json:"signature"`            // Hex-encoded Ed25519 signature
	Nonce              string       `json:"nonce"`
	Leaves             []MerkleLeaf `json:"leaves"`
}

// VerificationResult details the cryptographic verification status and forensic leaf localization.
type VerificationResult struct {
	ReceiptID        string            `json:"receipt_id"`
	SnapshotID       string            `json:"snapshot_id"`
	URL              string            `json:"url"`
	Status           AttestationStatus `json:"status"`
	SignatureValid   bool              `json:"signature_valid"`
	RootMatches      bool              `json:"root_matches"`
	MismatchedLeaves []string          `json:"mismatched_leaves,omitempty"`
	VerifiedAt       time.Time         `json:"verified_at"`
	LatencyMs        float64           `json:"latency_ms"`
	Summary          string            `json:"summary"`
}

// AttestationSummary aggregates results from multiple snapshot verifications.
type AttestationSummary struct {
	TotalEvaluated        int                  `json:"total_evaluated"`
	VerifiedCount         int                  `json:"verified_count"`
	TamperCount           int                  `json:"tamper_count"`
	InvalidSignatureCount int                  `json:"invalid_signature_count"`
	Results               []VerificationResult `json:"results"`
}
