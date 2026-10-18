package watcher

import (
	"time"
)

// TargetStatus represents the runtime health state of a watched target.
type TargetStatus string

const (
	StatusActive        TargetStatus = "active"
	StatusPaused        TargetStatus = "paused"
	StatusDriftDetected TargetStatus = "drift_detected"
	StatusError         TargetStatus = "error"
	StatusUnreachable   TargetStatus = "unreachable"
)

// DriftSeverity classifies the magnitude of detected website mutations.
type DriftSeverity string

const (
	DriftNone        DriftSeverity = "NONE"
	DriftCosmetic    DriftSeverity = "COSMETIC"
	DriftSubstantial DriftSeverity = "SUBSTANTIAL"
	DriftStructural  DriftSeverity = "STRUCTURAL"
	DriftOutage      DriftSeverity = "OUTAGE"
)

// WatchTarget defines a website configured for continuous temporal monitoring.
type WatchTarget struct {
	ID               string        `json:"id"`
	URL              string        `json:"url"`
	Name             string        `json:"name"`
	IntervalStr      string        `json:"interval"`
	IntervalDuration time.Duration `json:"-"`
	LastCheck        time.Time     `json:"last_check,omitempty"`
	LastChange       time.Time     `json:"last_change,omitempty"`
	LastSnapshotID   string        `json:"last_snapshot_id,omitempty"`
	BaselineHash     string        `json:"baseline_hash,omitempty"`
	LastHash         string        `json:"last_hash,omitempty"`
	CheckCount       int           `json:"check_count"`
	ChangeCount      int           `json:"change_count"`
	Status           TargetStatus  `json:"status"`
	DriftThreshold   float64       `json:"drift_threshold"` // Min change delta to trigger snapshot (e.g. 0.05 = 5%)
	IgnoreSelectors  []string      `json:"ignore_selectors,omitempty"`
	WebhookURL       string        `json:"webhook_url,omitempty"`
	Tags             []string      `json:"tags,omitempty"`
	CreatedAt        time.Time     `json:"created_at"`
}

// ChangeEvent represents an immutable record of a detected change between captures.
type ChangeEvent struct {
	ID             string        `json:"id"`
	TargetID       string        `json:"target_id"`
	TargetName     string        `json:"target_name"`
	TargetURL      string        `json:"target_url"`
	Timestamp      time.Time     `json:"timestamp"`
	OldSnapshotID  string        `json:"old_snapshot_id"`
	NewSnapshotID  string        `json:"new_snapshot_id"`
	OldHash        string        `json:"old_hash"`
	NewHash        string        `json:"new_hash"`
	Severity       DriftSeverity `json:"severity"`
	DeltaRatio     float64       `json:"delta_ratio"` // 0.00 to 1.00
	Summary        string        `json:"summary"`
	LinesAdded     int           `json:"lines_added"`
	LinesDeleted   int           `json:"lines_deleted"`
	WebhookStatus  string        `json:"webhook_status,omitempty"`
}

// WebhookPayload defines the standard JSON alert sent to external endpoints.
type WebhookPayload struct {
	Event         string        `json:"event"`
	TargetID      string        `json:"target_id"`
	TargetName    string        `json:"target_name"`
	TargetURL     string        `json:"target_url"`
	Timestamp     time.Time     `json:"timestamp"`
	Severity      DriftSeverity `json:"severity"`
	DeltaRatio    float64       `json:"delta_ratio"`
	Summary       string        `json:"summary"`
	OldSnapshotID string        `json:"old_snapshot_id"`
	NewSnapshotID string        `json:"new_snapshot_id"`
	DiffURL       string        `json:"diff_url"`
}
