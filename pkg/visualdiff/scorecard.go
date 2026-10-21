package visualdiff

import (
	"fmt"
	"strings"
)

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiBlue   = "\033[34m"
	ansiCyan   = "\033[36m"
	ansiGray   = "\033[90m"
)

// RenderVisualScorecard returns a colorized ANSI terminal scorecard.
func RenderVisualScorecard(res *VisualDiffResult) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("%s%s=== TIMECAPSULE TEMPORAL VISUAL & DOM DIFF SCORECARD ===%s\n", ansiBold, ansiCyan, ansiReset))
	b.WriteString(fmt.Sprintf("URL:              %s%s%s\n", ansiBold, res.URL, ansiReset))
	b.WriteString(fmt.Sprintf("Baseline (Old):   %s (%s)\n", res.OldSnapshotID, res.OldTimestamp.Format("2006-01-02 15:04:05 UTC")))
	b.WriteString(fmt.Sprintf("Target (New):     %s (%s)\n", res.NewSnapshotID, res.NewTimestamp.Format("2006-01-02 15:04:05 UTC")))
	b.WriteString(fmt.Sprintf("Analysis Time:    %d ms\n", res.DurationMs))

	sevColor := ansiGreen
	switch res.Severity {
	case SeverityBreaking:
		sevColor = ansiRed
	case SeveritySubstantial:
		sevColor = ansiYellow
	case SeverityMinor:
		sevColor = ansiBlue
	}

	b.WriteString(fmt.Sprintf("Visual Drift:     %s%.1f%%%s | Severity: %s[%s]%s\n", sevColor, res.VisualDriftScore, ansiReset, sevColor, res.Severity, ansiReset))
	b.WriteString(fmt.Sprintf("DOM Similarity:   %s%.1f%%%s (%d old nodes ↔ %d new nodes)\n", ansiCyan, res.StructuralSimilarity*100, ansiReset, res.TotalNodesOld, res.TotalNodesNew))
	b.WriteString(fmt.Sprintf("Mutations:        %s+%d added%s | %s-%d removed%s | %s~%d modified%s\n", ansiGreen, res.AddedNodes, ansiReset, ansiRed, res.RemovedNodes, ansiReset, ansiYellow, res.ModifiedNodes, ansiReset))

	var blocksStr []string
	for _, blk := range res.ImpactedBlocks {
		blocksStr = append(blocksStr, string(blk))
	}
	if len(blocksStr) == 0 {
		blocksStr = append(blocksStr, "NONE")
	}
	b.WriteString(fmt.Sprintf("Impacted Blocks:  [%s]\n", strings.Join(blocksStr, ", ")))
	b.WriteString("──────────────────────────────────────────────────────────────────────────────────\n")

	if len(res.Mutations) == 0 {
		b.WriteString(fmt.Sprintf("%s✔ IDENTICAL DOM HIERARCHY: No visual or structural mutations detected.%s\n", ansiGreen, ansiReset))
	} else {
		b.WriteString(fmt.Sprintf("%sHIERARCHICAL MUTATIONS (%d):%s\n\n", ansiBold, len(res.Mutations), ansiReset))
		for i, m := range res.Mutations {
			badge := ansiBlue + "[MODIFIED]" + ansiReset
			switch m.Type {
			case MutationNodeAdded:
				badge = ansiGreen + "[+ADDED]  " + ansiReset
			case MutationNodeRemoved:
				badge = ansiRed + "[-REMOVED]" + ansiReset
			case MutationTextModified:
				badge = ansiYellow + "[~TEXT]   " + ansiReset
			case MutationAttrModified:
				badge = ansiCyan + "[~ATTR]   " + ansiReset
			}

			b.WriteString(fmt.Sprintf("[%d] %s <%s> in %s%s%s\n", i+1, badge, m.Tag, ansiBold, m.SemanticBlock, ansiReset))
			b.WriteString(fmt.Sprintf("    DOM Path: %s%s%s\n", ansiGray, m.Path, ansiReset))
			b.WriteString(fmt.Sprintf("    Detail:   %s\n", m.Explanation))
			if m.OldValue != "" && m.NewValue != "" {
				b.WriteString(fmt.Sprintf("    Delta:    %s%s%s -> %s%s%s\n", ansiRed, m.OldValue, ansiReset, ansiGreen, m.NewValue, ansiReset))
			}
			b.WriteString("\n")
		}
	}
	b.WriteString("──────────────────────────────────────────────────────────────────────────────────\n")

	return b.String()
}

// RenderMermaidDOMTree produces a Mermaid diagram of the impacted DOM structure.
func RenderMermaidDOMTree(res *VisualDiffResult) string {
	var b strings.Builder
	b.WriteString("```mermaid\n")
	b.WriteString("flowchart TD\n")
	b.WriteString(fmt.Sprintf("    Root[\"Document: %s\"] --> Diff[\"Visual Drift: %.1f%% (%s)\"]\n", res.URL, res.VisualDriftScore, res.Severity))

	for i, m := range res.Mutations {
		if i >= 10 { // limit diagram size for readability
			b.WriteString(fmt.Sprintf("    Diff -.-> Extra[\"... and %d more mutations\"]\n", len(res.Mutations)-10))
			break
		}
		nodeID := fmt.Sprintf("M%d", i)
		label := fmt.Sprintf("%s: <%s> in %s", m.Type, m.Tag, m.SemanticBlock)
		switch m.Type {
		case MutationNodeAdded:
			b.WriteString(fmt.Sprintf("    Diff --> %s[\"🟢 %s\"]\n", nodeID, label))
		case MutationNodeRemoved:
			b.WriteString(fmt.Sprintf("    Diff --> %s[\"🔴 %s\"]\n", nodeID, label))
		default:
			b.WriteString(fmt.Sprintf("    Diff --> %s[\"🟡 %s\"]\n", nodeID, label))
		}
	}

	b.WriteString("```\n")
	return b.String()
}

// RenderMarkdownReport generates a GitHub Flavored Markdown summary report.
func RenderMarkdownReport(res *VisualDiffResult) string {
	var b strings.Builder
	b.WriteString("## 👁️ TimeCapsule Visual & DOM Diff Report\n\n")
	b.WriteString(fmt.Sprintf("- **Target URL:** `%s`\n", res.URL))
	b.WriteString(fmt.Sprintf("- **Baseline Snapshot:** `%s` (%s)\n", res.OldSnapshotID, res.OldTimestamp.Format("2006-01-02 15:04:05 UTC")))
	b.WriteString(fmt.Sprintf("- **Target Snapshot:** `%s` (%s)\n", res.NewSnapshotID, res.NewTimestamp.Format("2006-01-02 15:04:05 UTC")))
	b.WriteString(fmt.Sprintf("- **Visual Drift Score:** `%.1f%%` (**%s**)\n", res.VisualDriftScore, res.Severity))
	b.WriteString(fmt.Sprintf("- **DOM Similarity:** `%.1f%%` (%d old nodes vs %d new nodes)\n", res.StructuralSimilarity*100, res.TotalNodesOld, res.TotalNodesNew))
	b.WriteString(fmt.Sprintf("- **Net Mutations:** `+%d` added, `-%d` removed, `~%d` modified\n\n", res.AddedNodes, res.RemovedNodes, res.ModifiedNodes))

	b.WriteString("### Detected DOM Mutations\n\n")
	b.WriteString("| # | Type | Tag | Semantic Block | DOM Path | Summary |\n")
	b.WriteString("| :-: | :--- | :--- | :--- | :--- | :--- |\n")
	for i, m := range res.Mutations {
		b.WriteString(fmt.Sprintf("| %d | `%s` | `<%s>` | `%s` | `%s` | %s |\n",
			i+1, m.Type, m.Tag, m.SemanticBlock, m.Path, m.Explanation))
	}

	return b.String()
}
