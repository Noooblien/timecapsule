package archive

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// Snapshot represents a preserved point-in-time web page capture.
type Snapshot struct {
	ID          string            `json:"id"`
	URL         string            `json:"url"`
	Timestamp   time.Time         `json:"timestamp"`
	WaybackTS   string            `json:"wayback_timestamp"` // Format: YYYYMMDDHHMMSS
	Title       string            `json:"title"`
	StatusCode  int               `json:"status_code"`
	ContentType string            `json:"content_type"`
	Headers     map[string]string `json:"headers"`
	HTML        string            `json:"html"`
	Markdown    string            `json:"markdown"`
	ContentHash string            `json:"content_hash"`
	SizeBytes   int64             `json:"size_bytes"`
	AssetCount  int               `json:"asset_count"`
}

// ComputeHash calculates the SHA-256 fingerprint of the HTML content.
func ComputeHash(content []byte) string {
	h := sha256.New()
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

// FormatWaybackTS formats a timestamp into standard Wayback Machine format: YYYYMMDDhhmmss.
func FormatWaybackTS(t time.Time) string {
	return t.UTC().Format("20060102150405")
}
