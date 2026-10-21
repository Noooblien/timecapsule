package storage

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Noooblien/timecapsule/pkg/archive"
)

// PruneOptions configures retention and cleanup criteria for snapshots.
type PruneOptions struct {
	MaxAge        time.Duration // Maximum age of snapshots to keep (0 = no age limit)
	KeepPerURL    int           // Maximum number of snapshots to retain per URL (0 = unlimited)
	TargetURL     string        // If set, prune only snapshots for this specific URL
	Deduplicate   bool          // Remove consecutive identical snapshots (same content hash)
	DryRun        bool          // Compute pruning candidates without deleting them
}

// PruneCandidate identifies a snapshot flagged for deletion and the reason.
type PruneCandidate struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	WaybackTS string    `json:"wayback_ts"`
	Timestamp time.Time `json:"timestamp"`
	SizeBytes int64     `json:"size_bytes"`
	Reason    string    `json:"reason"`
}

// PruneReport summarizes the outcome of a pruning operation.
type PruneReport struct {
	Options       PruneOptions     `json:"options"`
	Evaluated     int              `json:"evaluated"`
	Candidates    []PruneCandidate `json:"candidates"`
	DeletedCount  int              `json:"deleted_count"`
	ReclaimedBytes int64           `json:"reclaimed_bytes"`
	DryRun        bool             `json:"dry_run"`
	Duration      time.Duration    `json:"duration"`
}

// Prune evaluates stored snapshots against the retention policy, cleans up disk space,
// and updates the storage catalog.
func (s *Store) Prune(opts PruneOptions) (*PruneReport, error) {
	start := time.Now()
	allSnaps := s.List() // sorted descending by timestamp

	report := &PruneReport{
		Options:   opts,
		Evaluated: len(allSnaps),
		DryRun:    opts.DryRun,
	}

	now := time.Now().UTC()
	cutoff := time.Time{}
	if opts.MaxAge > 0 {
		cutoff = now.Add(-opts.MaxAge)
	}

	// Group snapshots by normalized URL
	grouped := make(map[string][]*archive.Snapshot)
	targetNorm := ""
	if opts.TargetURL != "" {
		targetNorm = normalizeURL(opts.TargetURL)
	}

	for _, snap := range allSnaps {
		norm := normalizeURL(snap.URL)
		if targetNorm != "" && norm != targetNorm {
			continue
		}
		grouped[norm] = append(grouped[norm], snap)
	}

	toDeleteMap := make(map[string]string) // ID -> Reason

	for _, list := range grouped {
		// Sort chronologically ascending for deduplication & retention analysis
		sort.Slice(list, func(i, j int) bool {
			return list[i].Timestamp.Before(list[j].Timestamp)
		})

		// 1. Deduplication: remove consecutive snapshots with matching content hashes
		if opts.Deduplicate && len(list) > 1 {
			for i := 1; i < len(list); i++ {
				curr := list[i]
				prev := list[i-1]
				if curr.ContentHash == prev.ContentHash {
					reason := fmt.Sprintf("Duplicate content hash (%s) identical to previous snapshot (%s)",
						curr.ContentHash[:8], prev.ID)
					toDeleteMap[curr.ID] = reason
				}
			}
		}

		// 2. MaxAge expiration
		if opts.MaxAge > 0 {
			for _, snap := range list {
				if snap.Timestamp.Before(cutoff) {
					if _, already := toDeleteMap[snap.ID]; !already {
						toDeleteMap[snap.ID] = fmt.Sprintf("Exceeded maximum age of %v (captured %s)",
							opts.MaxAge, snap.Timestamp.Format("2006-01-02 15:04"))
					}
				}
			}
		}

		// 3. KeepPerURL: retain only the latest N snapshots per URL
		if opts.KeepPerURL > 0 && len(list) > opts.KeepPerURL {
			// Remaining active snapshots after deduplication/age flags
			excess := len(list) - opts.KeepPerURL
			for i := 0; i < excess; i++ {
				snap := list[i]
				if _, already := toDeleteMap[snap.ID]; !already {
					toDeleteMap[snap.ID] = fmt.Sprintf("Exceeded URL retention limit of %d snapshots (rank %d of %d)",
						opts.KeepPerURL, i+1, len(list))
				}
			}
		}
	}

	// Build candidate list
	for id, reason := range toDeleteMap {
		snap, ok := s.Get(id)
		if !ok {
			continue
		}
		report.Candidates = append(report.Candidates, PruneCandidate{
			ID:        snap.ID,
			URL:       snap.URL,
			WaybackTS: snap.WaybackTS,
			Timestamp: snap.Timestamp,
			SizeBytes: snap.SizeBytes,
			Reason:    reason,
		})
		report.ReclaimedBytes += snap.SizeBytes
	}

	// Sort candidates chronologically
	sort.Slice(report.Candidates, func(i, j int) bool {
		return report.Candidates[i].Timestamp.Before(report.Candidates[j].Timestamp)
	})

	// Execute deletion if not DryRun
	if !opts.DryRun {
		for _, c := range report.Candidates {
			if err := s.Delete(c.ID); err != nil {
				return report, fmt.Errorf("failed to prune snapshot '%s': %w", c.ID, err)
			}
			report.DeletedCount++
		}
	}

	report.Duration = time.Since(start)
	return report, nil
}

// RenderPruneScorecard formats a PruneReport as a terminal scorecard.
func RenderPruneScorecard(report *PruneReport) string {
	var sb strings.Builder
	sb.WriteString("\n")
	modeStr := "\033[1;32m[EXECUTE MODE]\033[0m"
	if report.DryRun {
		modeStr = "\033[1;33m[DRY-RUN SIMULATION]\033[0m"
	}
	sb.WriteString(fmt.Sprintf("🧹 \033[1;36mTimeCapsule Storage Pruning & Retention Report\033[0m %s\n", modeStr))
	sb.WriteString(strings.Repeat("═", 85) + "\n")
	sb.WriteString(fmt.Sprintf("Evaluated Snapshots: \033[1;37m%d\033[0m\n", report.Evaluated))
	sb.WriteString(fmt.Sprintf("Pruned / Flagged:    \033[1;31m%d\033[0m\n", len(report.Candidates)))
	sb.WriteString(fmt.Sprintf("Reclaimed Space:     \033[1;32m%.2f KB\033[0m (%d bytes)\n",
		float64(report.ReclaimedBytes)/1024.0, report.ReclaimedBytes))
	sb.WriteString(fmt.Sprintf("Audit Duration:      %v\n\n", report.Duration))

	if len(report.Candidates) > 0 {
		sb.WriteString("\033[1;33m[ Pruning Candidates Breakdown ]\033[0m\n")
		for i, c := range report.Candidates {
			sb.WriteString(fmt.Sprintf("  #%-2d [%s] %s (%s, %.1f KB)\n      \033[36mReason:\033[0m %s\n",
				i+1, c.WaybackTS, c.URL, c.ID, float64(c.SizeBytes)/1024.0, c.Reason))
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("\033[1;32m✓ Storage is optimal. No snapshots met pruning criteria.\033[0m\n\n")
	}

	return sb.String()
}
