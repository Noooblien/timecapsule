package watcher

import (
	"fmt"
	"strings"
	"time"
)

// ANSI color codes for terminal rendering.
const (
	colorReset   = "\033[0m"
	colorBold    = "\033[1m"
	colorDim     = "\033[2m"
	colorRed     = "\033[31m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorMagenta = "\033[35m"
	colorCyan    = "\033[36m"
	colorWhite   = "\033[37m"
)

// RenderWatcherScorecard formats an ANSI telemetry dashboard for all watched targets and events.
func RenderWatcherScorecard(targets []*WatchTarget, events []ChangeEvent) string {
	var sb strings.Builder

	sb.WriteString("\n" + colorBold + colorCyan + "==========================================================================================" + colorReset + "\n")
	sb.WriteString(colorBold + "  TIMECAPSULE CONTINUOUS DRIFT WATCHER & TEMPORAL MONITOR" + colorReset + "\n")
	sb.WriteString(colorDim + fmt.Sprintf("  Active Targets: %d  |  Total Checks: %d  |  Total Drift Events: %d",
		len(targets), totalChecks(targets), len(events)) + colorReset + "\n")
	sb.WriteString(colorBold + colorCyan + "==========================================================================================" + colorReset + "\n\n")

	// Target Inventory Table
	sb.WriteString(colorBold + "--- Configured Watch Targets ---" + colorReset + "\n")
	sb.WriteString(fmt.Sprintf("  %-22s %-32s %-10s %-8s %-8s %-12s %s\n",
		"Target Name", "Monitored URL", "Interval", "Checks", "Changes", "Status", "Last Change"))
	sb.WriteString("  ----------------------------------------------------------------------------------------------------\n")

	if len(targets) == 0 {
		sb.WriteString("  (No watch targets configured)\n")
	} else {
		for _, t := range targets {
			statusBadge := formatStatus(t.Status)
			lastChg := "Never"
			if !t.LastChange.IsZero() {
				lastChg = timeAgo(t.LastChange)
			}
			displayURL := t.URL
			if len(displayURL) > 30 {
				displayURL = displayURL[:27] + "..."
			}
			displayName := t.Name
			if len(displayName) > 20 {
				displayName = displayName[:17] + "..."
			}

			sb.WriteString(fmt.Sprintf("  %-22s %-32s %-10s %-8d %-8d %-20s %s\n",
				displayName, displayURL, t.IntervalStr, t.CheckCount, t.ChangeCount, statusBadge, lastChg))
		}
	}
	sb.WriteString("\n")

	// Recent Change Events
	sb.WriteString(colorBold + "--- Recent Drift & Mutation Events ---" + colorReset + "\n")
	if len(events) == 0 {
		sb.WriteString("  (No content drift detected yet - all monitored pages pristine)\n")
	} else {
		sb.WriteString(fmt.Sprintf("  %-19s %-18s %-14s %-10s %-12s %s\n",
			"Timestamp", "Target", "Severity", "Delta %", "Diff (+/-)", "Summary"))
		sb.WriteString("  ----------------------------------------------------------------------------------------------------\n")

		// Render up to 10 latest events in reverse order
		maxEv := 10
		if len(events) < maxEv {
			maxEv = len(events)
		}
		for i := len(events) - 1; i >= len(events)-maxEv; i-- {
			ev := events[i]
			sevBadge := formatSeverity(ev.Severity)
			diffStat := fmt.Sprintf("+%d/-%d", ev.LinesAdded, ev.LinesDeleted)
			deltaPct := fmt.Sprintf("%.1f%%", ev.DeltaRatio*100)

			sb.WriteString(fmt.Sprintf("  %-19s %-18s %-22s %-10s %-12s %s\n",
				ev.Timestamp.Format("2006-01-02 15:04:05"),
				truncate(ev.TargetName, 17),
				sevBadge,
				deltaPct,
				diffStat,
				truncate(ev.Summary, 35),
			))
		}
	}
	sb.WriteString("\n")

	return sb.String()
}

// RenderMermaidTimeline generates a Mermaid timeline of detected drift events.
func RenderMermaidTimeline(events []ChangeEvent) string {
	var sb strings.Builder
	sb.WriteString("```mermaid\ntimeline\n")
	sb.WriteString("    title Website Temporal Mutation History\n")

	if len(events) == 0 {
		sb.WriteString("    Pristine Baseline : Zero content drift recorded\n")
	} else {
		for _, ev := range events {
			ts := ev.Timestamp.Format("15:04:05")
			cleanSummary := strings.ReplaceAll(ev.Summary, ":", "-")
			sb.WriteString(fmt.Sprintf("    %s : %s [%s] : %s\n", ts, ev.TargetName, ev.Severity, cleanSummary))
		}
	}

	sb.WriteString("```\n")
	return sb.String()
}

// RenderMarkdownReport generates a GitHub Flavored Markdown summary report.
func RenderMarkdownReport(targets []*WatchTarget, events []ChangeEvent) string {
	var sb strings.Builder

	sb.WriteString("# TimeCapsule Continuous Drift Watcher Report\n\n")
	sb.WriteString(fmt.Sprintf("- **Active Targets:** `%d`\n", len(targets)))
	sb.WriteString(fmt.Sprintf("- **Total Checks Completed:** `%d`\n", totalChecks(targets)))
	sb.WriteString(fmt.Sprintf("- **Total Drift Events Detected:** `%d`\n\n", len(events)))

	sb.WriteString("## Monitored Targets\n\n")
	sb.WriteString("| Target Name | URL | Interval | Total Checks | Drift Count | Status | Last Drift |\n")
	sb.WriteString("| :--- | :--- | :--- | :---: | :---: | :---: | :--- |\n")

	for _, t := range targets {
		lastChg := "Never"
		if !t.LastChange.IsZero() {
			lastChg = t.LastChange.Format("2006-01-02 15:04:05 UTC")
		}
		sb.WriteString(fmt.Sprintf("| **%s** | [%s](%s) | `%s` | `%d` | `%d` | `%s` | %s |\n",
			t.Name, t.URL, t.URL, t.IntervalStr, t.CheckCount, t.ChangeCount, t.Status, lastChg))
	}
	sb.WriteString("\n")

	if len(events) > 0 {
		sb.WriteString("## Mutation Event Audit Log\n\n")
		sb.WriteString("| Timestamp | Target | Severity | Delta % | Diff Lines | Change Summary |\n")
		sb.WriteString("| :--- | :--- | :---: | :---: | :---: | :--- |\n")
		for _, ev := range events {
			sb.WriteString(fmt.Sprintf("| `%s` | **%s** | `%s` | `%.1f%%` | `+%d / -%d` | %s |\n",
				ev.Timestamp.Format("2006-01-02 15:04:05"), ev.TargetName, ev.Severity, ev.DeltaRatio*100, ev.LinesAdded, ev.LinesDeleted, ev.Summary))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Temporal Change Timeline\n\n")
	sb.WriteString(RenderMermaidTimeline(events))

	return sb.String()
}

func totalChecks(targets []*WatchTarget) int {
	sum := 0
	for _, t := range targets {
		sum += t.CheckCount
	}
	return sum
}

func formatStatus(s TargetStatus) string {
	switch s {
	case StatusActive:
		return colorGreen + "● ACTIVE" + colorReset
	case StatusDriftDetected:
		return colorYellow + "⚡ DRIFT" + colorReset
	case StatusError, StatusUnreachable:
		return colorRed + "✖ FAILED" + colorReset
	case StatusPaused:
		return colorDim + "⏸ PAUSED" + colorReset
	default:
		return string(s)
	}
}

func formatSeverity(s DriftSeverity) string {
	switch s {
	case DriftCosmetic:
		return colorBlue + "[COSMETIC]" + colorReset
	case DriftSubstantial:
		return colorYellow + colorBold + "[SUBSTANTIAL]" + colorReset
	case DriftStructural:
		return colorMagenta + colorBold + "[STRUCTURAL]" + colorReset
	case DriftOutage:
		return colorRed + colorBold + "[OUTAGE]" + colorReset
	default:
		return colorGreen + "[NONE]" + colorReset
	}
}

func timeAgo(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
