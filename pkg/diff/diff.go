package diff

import (
	"fmt"
	"html"
	"strings"

	"github.com/Noooblien/timecapsule/pkg/archive"
)

// DiffLineType indicates if a line was added, removed, or unchanged.
type DiffLineType int

const (
	LineUnchanged DiffLineType = 0
	LineAdded     DiffLineType = 1
	LineRemoved   DiffLineType = 2
)

// DiffLine represents a single line in a unified diff.
type DiffLine struct {
	Type    DiffLineType `json:"type"`
	Content string       `json:"content"`
}

// DiffResult encapsulates comparison metrics and line deltas between two snapshots.
type DiffResult struct {
	OldID          string     `json:"old_id"`
	NewID          string     `json:"new_id"`
	URL            string     `json:"url"`
	OldTimestamp   string     `json:"old_timestamp"`
	NewTimestamp   string     `json:"new_timestamp"`
	Identical      bool       `json:"identical"`
	Similarity     float64    `json:"similarity"` // 0.0 to 1.0
	AddedCount     int        `json:"added_count"`
	RemovedCount   int        `json:"removed_count"`
	UnchangedCount int        `json:"unchanged_count"`
	Lines          []DiffLine `json:"lines"`
}

// CompareText computes line-by-line diff between two arbitrary text strings.
func CompareText(oldName, newName, oldText, newText string) *DiffResult {
	res := &DiffResult{
		OldID: oldName,
		NewID: newName,
		Lines: make([]DiffLine, 0),
	}

	if oldText == newText {
		res.Identical = true
		res.Similarity = 1.0
		return res
	}

	oldLines := strings.Split(oldText, "\n")
	newLines := strings.Split(newText, "\n")

	diffLines := computeLCSDiff(oldLines, newLines)
	res.Lines = diffLines

	for _, dl := range diffLines {
		switch dl.Type {
		case LineAdded:
			res.AddedCount++
		case LineRemoved:
			res.RemovedCount++
		case LineUnchanged:
			res.UnchangedCount++
		}
	}

	total := res.AddedCount + res.RemovedCount + res.UnchangedCount
	if total > 0 {
		res.Similarity = float64(res.UnchangedCount) / float64(total)
	}

	res.Identical = (res.AddedCount == 0 && res.RemovedCount == 0)
	return res
}

// CompareSnapshots computes line-by-line unified diff between old and new snapshots.
func CompareSnapshots(oldSnap, newSnap *archive.Snapshot) *DiffResult {
	if oldSnap.ContentHash == newSnap.ContentHash {
		return &DiffResult{
			OldID:          oldSnap.ID,
			NewID:          newSnap.ID,
			URL:            newSnap.URL,
			OldTimestamp:   oldSnap.Timestamp.Format("2006-01-02 15:04:05 UTC"),
			NewTimestamp:   newSnap.Timestamp.Format("2006-01-02 15:04:05 UTC"),
			Identical:      true,
			Similarity:     1.0,
			UnchangedCount: len(strings.Split(oldSnap.Markdown, "\n")),
			Lines:          nil,
		}
	}

	res := CompareText(oldSnap.ID, newSnap.ID, oldSnap.Markdown, newSnap.Markdown)
	res.URL = newSnap.URL
	res.OldTimestamp = oldSnap.Timestamp.Format("2006-01-02 15:04:05 UTC")
	res.NewTimestamp = newSnap.Timestamp.Format("2006-01-02 15:04:05 UTC")
	return res
}

// computeLCSDiff runs Longest Common Subsequence algorithm on string slices.
func computeLCSDiff(a, b []string) []DiffLine {
	n := len(a)
	m := len(b)

	// dp table
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}

	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack to extract diff
	var rev []DiffLine
	i, j := n, m
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && a[i-1] == b[j-1] {
			rev = append(rev, DiffLine{Type: LineUnchanged, Content: a[i-1]})
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			rev = append(rev, DiffLine{Type: LineAdded, Content: b[j-1]})
			j--
		} else if i > 0 && (j == 0 || dp[i][j-1] < dp[i-1][j]) {
			rev = append(rev, DiffLine{Type: LineRemoved, Content: a[i-1]})
			i--
		}
	}

	// Reverse rev
	res := make([]DiffLine, len(rev))
	for k := range rev {
		res[k] = rev[len(rev)-1-k]
	}
	return res
}

// FormatANSI produces terminal colored unified diff (+ green, - red).
func (dr *DiffResult) FormatANSI() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("--- %s (%s)\n", dr.OldID, dr.OldTimestamp))
	sb.WriteString(fmt.Sprintf("+++ %s (%s)\n", dr.NewID, dr.NewTimestamp))
	sb.WriteString(fmt.Sprintf("Similarity: %.1f%% (+%d / -%d)\n\n", dr.Similarity*100, dr.AddedCount, dr.RemovedCount))

	if dr.Identical {
		sb.WriteString("  [Content is identical]\n")
		return sb.String()
	}

	for _, l := range dr.Lines {
		switch l.Type {
		case LineAdded:
			sb.WriteString(fmt.Sprintf("\033[32m+ %s\033[0m\n", l.Content))
		case LineRemoved:
			sb.WriteString(fmt.Sprintf("\033[31m- %s\033[0m\n", l.Content))
		case LineUnchanged:
			sb.WriteString(fmt.Sprintf("  %s\n", l.Content))
		}
	}
	return sb.String()
}

// FormatHTML produces styled HTML with highlight badges for side-by-side or unified web viewing.
func (dr *DiffResult) FormatHTML() string {
	if dr.Identical {
		return `<div style="text-align:center;padding:30px;color:#8b949e;font-size:14px;">Both snapshots have identical content. No changes detected.</div>`
	}

	var sb strings.Builder
	sb.WriteString("<div style=\"font-family:monospace;font-size:13px;line-height:1.6;background:#0d1117;border:1px solid #30363d;border-radius:6px;overflow-x:auto;\">")
	for _, l := range dr.Lines {
		escaped := html.EscapeString(l.Content)
		if escaped == "" {
			escaped = "&nbsp;"
		}
		switch l.Type {
		case LineAdded:
			sb.WriteString(fmt.Sprintf("<div style=\"background:#2ea04326;color:#7ee787;padding:3px 12px;border-left:4px solid #2ea043;white-space:pre-wrap;\">+ %s</div>", escaped))
		case LineRemoved:
			sb.WriteString(fmt.Sprintf("<div style=\"background:#da363326;color:#f85149;padding:3px 12px;border-left:4px solid #da3633;white-space:pre-wrap;\">- %s</div>", escaped))
		case LineUnchanged:
			sb.WriteString(fmt.Sprintf("<div style=\"color:#8b949e;padding:3px 12px;white-space:pre-wrap;\">&nbsp;&nbsp;%s</div>", escaped))
		}
	}
	sb.WriteString("</div>")
	return sb.String()
}
