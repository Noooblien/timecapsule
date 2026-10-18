package bench

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ANSI color codes.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[1;31m"
	colorGreen  = "\033[1;32m"
	colorYellow = "\033[1;33m"
	colorCyan   = "\033[1;36m"
	colorBold   = "\033[1m"
)

// RenderCorpusScorecard produces an ANSI dashboard for an individual corpus audit.
func RenderCorpusScorecard(res *CorpusResult) string {
	var sb strings.Builder

	statusBadge := fmt.Sprintf("%s[PASS / HERMETIC]%s", colorGreen, colorReset)
	if res.Status == StatusLeakDetected {
		statusBadge = fmt.Sprintf("%s[LEAK BREACH]%s", colorRed, colorReset)
	} else if res.Status == StatusDegraded {
		statusBadge = fmt.Sprintf("%s[DEGRADED]%s", colorYellow, colorReset)
	}

	scoreColor := colorGreen
	if res.FidelityScore < 75.0 {
		scoreColor = colorRed
	} else if res.FidelityScore < 85.0 {
		scoreColor = colorYellow
	}

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("%s╭────────────────────────────────────────────────────────────────────────────╮%s\n", colorCyan, colorReset))
	sb.WriteString(fmt.Sprintf("%s│%s  ⏳  %sTimeCapsule Archival Fidelity & Security Audit Scorecard%s          %s│%s\n", colorCyan, colorReset, colorBold, colorReset, colorCyan, colorReset))
	sb.WriteString(fmt.Sprintf("%s├────────────────────────────────────────────────────────────────────────────┤%s\n", colorCyan, colorReset))
	sb.WriteString(fmt.Sprintf("│  %sCorpus:%s           %-28s  %sStatus:%s       %-22s │\n", colorBold, colorReset, truncate(res.Name, 28), colorBold, colorReset, statusBadge))
	sb.WriteString(fmt.Sprintf("│  %sTarget URL:%s       %-28s  %sLatency:%s      %-12s │\n", colorBold, colorReset, truncate(res.TargetURL, 28), colorBold, colorReset, fmt.Sprintf("%d µs", res.Metrics.SerializationLatencyUs)))
	sb.WriteString(fmt.Sprintf("│  %sFidelity Score:%s   %s%-6.1f / 100.0%s                 %sHermetic Score:%s%-12s │\n", colorBold, colorReset, scoreColor, res.FidelityScore, colorReset, colorBold, colorReset, fmt.Sprintf("%.1f%%", res.Metrics.HermeticScore)))
	sb.WriteString(fmt.Sprintf("│  %sAsset Retention:%s  %-28s  %sISO 28500:%s    %-12s │\n", colorBold, colorReset, fmt.Sprintf("%d/%d (%.1f%%)", res.Metrics.InlinedAssets, res.Metrics.TotalDetectedAssets, res.Metrics.PreservationRatio), colorBold, colorReset, boolStatus(res.ISOReport.ValidFormat)))
	sb.WriteString(fmt.Sprintf("│  %sOriginal Size:%s    %-28s  %sBundled Size:%s %-12s │\n", colorBold, colorReset, fmt.Sprintf("%d bytes", res.Metrics.OriginalSizeBytes), colorBold, colorReset, fmt.Sprintf("%d bytes", res.Metrics.BundledSizeBytes)))
	sb.WriteString(fmt.Sprintf("│  %sWARC GZIP Size:%s   %-28s  %sSpace Savings:%s %-12s │\n", colorBold, colorReset, fmt.Sprintf("%d bytes (%.2fx)", res.Metrics.GzipSizeBytes, res.Metrics.CompressionRatio), colorBold, colorReset, fmt.Sprintf("%.1f%%", res.Metrics.SpaceSavingsPct)))
	sb.WriteString(fmt.Sprintf("│  %sThroughput:%s       %-28s  %sWARC Records:%s %-12s │\n", colorBold, colorReset, fmt.Sprintf("%.1f MB/s", res.Metrics.ThroughputMBps), colorBold, colorReset, fmt.Sprintf("%d records", res.ISOReport.RecordCount)))
	sb.WriteString(fmt.Sprintf("│  %sSummary:%s          %-56s │\n", colorBold, colorReset, truncate(res.Summary, 56)))

	if len(res.Leaks) > 0 {
		sb.WriteString(fmt.Sprintf("%s├────────────────────────────────────────────────────────────────────────────┤%s\n", colorCyan, colorReset))
		sb.WriteString(fmt.Sprintf("│  %sDetected Network Leaks & Privacy Vulnerabilities (%d found):%s            │\n", colorRed, len(res.Leaks), colorReset))
		for _, l := range res.Leaks {
			sevColor := colorYellow
			if l.Severity == SeverityCritical {
				sevColor = colorRed
			}
			sb.WriteString(fmt.Sprintf("│  %s[%-8s]%s %-10s %-48s │\n", sevColor, l.Severity, colorReset, l.ElementType, truncate(l.URL, 48)))
		}
	}

	sb.WriteString(fmt.Sprintf("%s╰────────────────────────────────────────────────────────────────────────────╯%s\n", colorCyan, colorReset))
	return sb.String()
}

// RenderSuiteScorecard produces an aggregated ANSI summary across all test corpuses.
func RenderSuiteScorecard(suite *BenchmarkSuite) string {
	var sb strings.Builder

	statusBadge := fmt.Sprintf("%s[SUITE PASS]%s", colorGreen, colorReset)
	if !suite.OverallPassed {
		statusBadge = fmt.Sprintf("%s[SUITE FAIL]%s", colorRed, colorReset)
	}

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("%s========================================================================================%s\n", colorCyan, colorReset))
	sb.WriteString(fmt.Sprintf("   %sTIMECAPSULE ARCHIVAL FIDELITY & ISO 28500 BENCHMARK SUITE%s\n", colorBold, colorReset))
	sb.WriteString(fmt.Sprintf("%s========================================================================================%s\n", colorCyan, colorReset))

	for i, r := range suite.CorpusResults {
		fmt.Fprintf(&sb, "\n%s[Corpus %d/%d]%s %s%s%s: %s\n",
			colorYellow, i+1, len(suite.CorpusResults), colorReset, colorBold, r.Name, colorReset, r.TargetURL)
		sb.WriteString(RenderCorpusScorecard(&r))
	}

	fmt.Fprintf(&sb, "\n%s✓ Benchmark Complete.%s Status: %s | Pass Rate: %d/%d (Mean Fidelity: %.1f%%)\n\n",
		colorGreen, colorReset, statusBadge, suite.PassedCount, suite.TotalCount, suite.MeanFidelity)
	return sb.String()
}

// RenderMarkdown creates a formatted Markdown report table for pull requests and CI/CD.
func RenderMarkdown(suite *BenchmarkSuite) string {
	var sb strings.Builder

	statusBadge := "✅ PASS"
	if !suite.OverallPassed {
		statusBadge = "❌ FAIL"
	}

	sb.WriteString("### ⏳ TimeCapsule Archival Fidelity & ISO 28500 Benchmark\n\n")
	sb.WriteString(fmt.Sprintf("**Status:** %s | **Passed:** `%d/%d` | **Mean Fidelity:** `%.1f / 100.0` | **Time:** `%d µs`\n\n",
		statusBadge, suite.PassedCount, suite.TotalCount, suite.MeanFidelity, suite.TotalTimeUs))

	sb.WriteString("| Corpus | Hermetic Score | Asset Retention | ISO 28500 WARC | Space Savings | Throughput | Verdict |\n")
	sb.WriteString("| :--- | :--- | :--- | :--- | :--- | :--- | :--- |\n")

	for _, r := range suite.CorpusResults {
		isoStatus := "✅ Valid"
		if !r.ISOReport.ValidFormat {
			isoStatus = "❌ Invalid"
		}
		verdict := "✅ PASS"
		if r.Status == StatusLeakDetected {
			verdict = "❌ LEAK"
		} else if r.Status == StatusDegraded {
			verdict = "⚠️ DEGRADED"
		}

		sb.WriteString(fmt.Sprintf("| **%s** | `%.1f%%` | `%d/%d (%.1f%%)` | %s | `%.1f%% (%.1fx)` | `%.1f MB/s` | %s |\n",
			r.Name, r.Metrics.HermeticScore, r.Metrics.InlinedAssets, r.Metrics.TotalDetectedAssets, r.Metrics.PreservationRatio,
			isoStatus, r.Metrics.SpaceSavingsPct, r.Metrics.CompressionRatio, r.Metrics.ThroughputMBps, verdict))
	}
	sb.WriteString("\n")
	return sb.String()
}

// RenderJSON outputs structured JSON representation of the benchmark suite.
func RenderJSON(suite *BenchmarkSuite) (string, error) {
	bytes, err := json.MarshalIndent(suite, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func boolStatus(b bool) string {
	if b {
		return "VALID"
	}
	return "INVALID"
}
