package bundle

import (
	"time"
)

// BundleConfig controls asset discovery, size boundaries, and script handling.
type BundleConfig struct {
	InlineImages       bool          `json:"inline_images"`
	InlineStyles       bool          `json:"inline_styles"`
	InlineScripts      bool          `json:"inline_scripts"`
	NeutralizeScripts  bool          `json:"neutralize_scripts"` // Convert <script> to safe data blocks
	MaxAssetBytes      int64         `json:"max_asset_bytes"`    // Max size of a single inlined asset (default 5MB)
	Timeout            time.Duration `json:"timeout"`
	UserAgent          string        `json:"user_agent"`
}

// DefaultBundleConfig returns standard production settings for single-file archiving.
func DefaultBundleConfig() BundleConfig {
	return BundleConfig{
		InlineImages:      true,
		InlineStyles:      true,
		InlineScripts:     false,
		NeutralizeScripts: true,
		MaxAssetBytes:     5 * 1024 * 1024, // 5 MB
		Timeout:           15 * time.Second,
		UserAgent:         "Mozilla/5.0 (compatible; TimeCapsule/1.2; +https://github.com/Noooblien/timecapsule)",
	}
}

// InlinedAsset tracks a sub-resource bundled into the single-file DOM.
type InlinedAsset struct {
	OriginalURL string `json:"original_url"`
	ResolvedURL string `json:"resolved_url"`
	AssetType   string `json:"asset_type"` // "image", "stylesheet", "script", "font"
	MimeType    string `json:"mime_type"`
	SizeBytes   int64  `json:"size_bytes"`
	Status      string `json:"status"` // "inlined", "skipped", "failed"
	ErrorMsg    string `json:"error_msg,omitempty"`
}

// BundleResult encapsulates the output of a single-file DOM serialization.
type BundleResult struct {
	OriginalURL       string         `json:"original_url"`
	Title             string         `json:"title"`
	OriginalSizeBytes int64          `json:"original_size_bytes"`
	BundledSizeBytes  int64          `json:"bundled_size_bytes"`
	BundledHTML       string         `json:"bundled_html"`
	AssetsInlined     int            `json:"assets_inlined"`
	AssetsSkipped     int            `json:"assets_skipped"`
	Assets            []InlinedAsset `json:"assets"`
	DurationMs        int64          `json:"duration_ms"`
	CreatedAt         time.Time      `json:"created_at"`
}
