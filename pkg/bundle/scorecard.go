package bundle

import (
	"fmt"
	"strings"
)

// ANSI terminal colors.
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[1;32m"
	colorYellow = "\033[1;33m"
	colorRed    = "\033[1;31m"
	colorCyan   = "\033[1;36m"
	colorBold   = "\033[1m"
	colorGray   = "\033[0;90m"
)

// RenderBundleScorecard generates an ANSI terminal scorecard summarizing the single-file serialization.
func RenderBundleScorecard(res *BundleResult) string {
	var sb strings.Builder

	expansionRatio := 1.0
	if res.OriginalSizeBytes > 0 {
		expansionRatio = float64(res.BundledSizeBytes) / float64(res.OriginalSizeBytes)
	}

	statusBadge := fmt.Sprintf("%s[100%% HERMETIC - READY]%s", colorGreen, colorReset)
	if res.AssetsSkipped > 0 {
		statusBadge = fmt.Sprintf("%s[PARTIAL - %d SKIPPED]%s", colorYellow, res.AssetsSkipped, colorReset)
	}

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("%s╭────────────────────────────────────────────────────────────────────────────╮%s\n", colorCyan, colorReset))
	sb.WriteString(fmt.Sprintf("%s│%s  📦  %sTimeCapsule Single-File Verbatim DOM Serialization%s                %s│%s\n", colorCyan, colorReset, colorBold, colorReset, colorCyan, colorReset))
	sb.WriteString(fmt.Sprintf("%s├────────────────────────────────────────────────────────────────────────────┤%s\n", colorCyan, colorReset))
	sb.WriteString(fmt.Sprintf("│  %sSource URL:%s       %-56s │\n", colorBold, colorReset, truncate(res.OriginalURL, 56)))
	sb.WriteString(fmt.Sprintf("│  %sPage Title:%s       %-56s │\n", colorBold, colorReset, truncate(res.Title, 56)))
	sb.WriteString(fmt.Sprintf("│  %sOriginal Size:%s    %-28s  %sDuration:%s  %-15s │\n", colorBold, colorReset, formatBytes(res.OriginalSizeBytes), colorBold, colorReset, fmt.Sprintf("%d ms", res.DurationMs)))
	sb.WriteString(fmt.Sprintf("│  %sBundled Size:%s     %-28s  %sExpansion:%s %-15s │\n", colorBold, colorReset, formatBytes(res.BundledSizeBytes), colorBold, colorReset, fmt.Sprintf("%.2fx", expansionRatio)))
	sb.WriteString(fmt.Sprintf("│  %sAssets Inlined:%s   %-28d  %sStatus:%s     %-24s │\n", colorBold, colorReset, res.AssetsInlined, colorBold, colorReset, statusBadge))

	if len(res.Assets) > 0 {
		sb.WriteString(fmt.Sprintf("%s├────────────────────────────────────────────────────────────────────────────┤%s\n", colorCyan, colorReset))
		sb.WriteString(fmt.Sprintf("│  %sInlined Sub-Resources Breakdown (%d assets):%s                             │\n", colorBold, len(res.Assets), colorReset))
		for _, a := range res.Assets {
			icon := "📄"
			if a.AssetType == "image" {
				icon = "🖼️ "
			} else if a.AssetType == "stylesheet" {
				icon = "🎨"
			}
			badge := fmt.Sprintf("%s✓ inlined%s", colorGreen, colorReset)
			if a.Status != "inlined" {
				badge = fmt.Sprintf("%s✗ %s%s", colorRed, truncate(a.ErrorMsg, 12), colorReset)
			}
			sb.WriteString(fmt.Sprintf("│  %s %-10s %-12s %-10s %-32s │\n", icon, a.AssetType, formatBytes(a.SizeBytes), badge, truncate(a.OriginalURL, 32)))
		}
	}

	sb.WriteString(fmt.Sprintf("%s╰────────────────────────────────────────────────────────────────────────────╯%s\n", colorCyan, colorReset))
	return sb.String()
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
