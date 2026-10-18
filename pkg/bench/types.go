package bench

import (
	"time"
)

// AuditStatus categorizes the archival fidelity and security state.
type AuditStatus string

const (
	StatusPass         AuditStatus = "PASS"
	StatusDegraded     AuditStatus = "DEGRADED"
	StatusLeakDetected AuditStatus = "LEAK_DETECTED"
)

// LeakSeverity indicates the severity of an un-neutralized or external resource leak.
type LeakSeverity string

const (
	SeverityInfo     LeakSeverity = "INFO"
	SeverityWarn     LeakSeverity = "WARN"
	SeverityCritical LeakSeverity = "CRITICAL"
)

// ResourceLeak captures an un-inlined remote network dependency or tracker in an offline bundle.
type ResourceLeak struct {
	URL         string       `json:"url"`
	ElementType string       `json:"element_type"` // <script>, <img>, <link>, <iframe>, @import
	Severity    LeakSeverity `json:"severity"`
	Description string       `json:"description"`
}

// ISOValidation reports compliance against the ISO 28500 WARC standard.
type ISOValidation struct {
	ValidFormat      bool     `json:"valid_format"`
	HasWarcInfo      bool     `json:"has_warcinfo"`
	HasResponse      bool     `json:"has_response"`
	DigestMatch      bool     `json:"digest_match"`
	RecordCount      int      `json:"record_count"`
	MemberGzipValid  bool     `json:"member_gzip_valid"`
	Errors           []string `json:"errors,omitempty"`
}

// FidelityMetrics captures empirical preservation and compression numbers.
type FidelityMetrics struct {
	TotalDetectedAssets    int     `json:"total_detected_assets"`
	InlinedAssets          int     `json:"inlined_assets"`
	PreservationRatio      float64 `json:"preservation_ratio"`      // Inlined / Detected %
	HermeticScore          float64 `json:"hermetic_score"`          // 0.0 to 100.0%
	OriginalSizeBytes      int64   `json:"original_size_bytes"`
	BundledSizeBytes       int64   `json:"bundled_size_bytes"`
	WarcSizeBytes          int64   `json:"warc_size_bytes"`
	GzipSizeBytes          int64   `json:"gzip_size_bytes"`
	CompressionRatio       float64 `json:"compression_ratio"`       // Raw / Gzip
	SpaceSavingsPct        float64 `json:"space_savings_pct"`       // (1 - Gzip/Raw) * 100
	ThroughputMBps         float64 `json:"throughput_mb_s"`
	SerializationLatencyUs int64   `json:"serialization_latency_us"`
}

// CorpusResult encapsulates the evaluation of a single archived page or corpus.
type CorpusResult struct {
	Name          string          `json:"name"`
	TargetURL     string          `json:"target_url"`
	Status        AuditStatus     `json:"status"`
	FidelityScore float64         `json:"fidelity_score"` // 0.0 - 100.0%
	Metrics       FidelityMetrics `json:"metrics"`
	ISOReport     ISOValidation   `json:"iso_report"`
	Leaks         []ResourceLeak  `json:"leaks"`
	Passed        bool            `json:"passed"`
	Summary       string          `json:"summary"`
}

// BenchmarkSuite aggregates multi-corpus test results.
type BenchmarkSuite struct {
	SuiteName      string         `json:"suite_name"`
	CorpusResults  []CorpusResult `json:"corpus_results"`
	PassedCount    int            `json:"passed_count"`
	TotalCount     int            `json:"total_count"`
	OverallPassed  bool           `json:"overall_passed"`
	MeanFidelity   float64        `json:"mean_fidelity"`
	TotalTimeUs    int64          `json:"total_time_us"`
	EvaluatedAt    time.Time      `json:"evaluated_at"`
}
