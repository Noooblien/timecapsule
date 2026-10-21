package attest

import (
	"fmt"
	"strings"
	"time"
)

// ANSI color escapes for terminal scorecard rendering.
const (
	colorReset   = "\033[0m"
	colorBold    = "\033[1m"
	colorRed     = "\033[1;31m"
	colorGreen   = "\033[1;32m"
	colorYellow  = "\033[1;33m"
	colorBlue    = "\033[1;34m"
	colorMagenta = "\033[1;35m"
	colorCyan    = "\033[1;36m"
	colorGray    = "\033[0;90m"
)

// RenderAttestationScorecard formats verification results into a high-visibility terminal scorecard.
func RenderAttestationScorecard(summary AttestationSummary) string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("%s┌────────────────────────────────────────────────────────────────────────┐%s\n", colorCyan, colorReset))
	sb.WriteString(fmt.Sprintf("%s│ %s%-70s%s%s │%s\n", colorCyan, colorBold, "  TIMECAPSULE CRYPTOGRAPHIC PROVENANCE & MERKLE ATTESTATION", colorReset, colorCyan, colorReset))
	sb.WriteString(fmt.Sprintf("%s├────────────────────────────────────────────────────────────────────────┤%s\n", colorCyan, colorReset))
	sb.WriteString(fmt.Sprintf("%s│%s Total Snapshots Evaluated:     %-38d %s│%s\n", colorCyan, colorReset, summary.TotalEvaluated, colorCyan, colorReset))
	sb.WriteString(fmt.Sprintf("%s│%s Verified Authentic (100%%):    %s%-38d%s %s│%s\n", colorCyan, colorReset, colorGreen, summary.VerifiedCount, colorReset, colorCyan, colorReset))
	sb.WriteString(fmt.Sprintf("%s│%s Tampering Breaches Detected:   %s%-38d%s %s│%s\n", colorCyan, colorReset, colorRed, summary.TamperCount, colorReset, colorCyan, colorReset))
	sb.WriteString(fmt.Sprintf("%s│%s Forged / Invalid Signatures:   %s%-38d%s %s│%s\n", colorCyan, colorReset, colorYellow, summary.InvalidSignatureCount, colorReset, colorCyan, colorReset))
	sb.WriteString(fmt.Sprintf("%s└────────────────────────────────────────────────────────────────────────┘%s\n\n", colorCyan, colorReset))

	// Breakdown table
	sb.WriteString(fmt.Sprintf("%s%s%-16s %-24s %-16s %-10s %-8s%s\n",
		colorBold, colorBlue, "SNAPSHOT ID", "TARGET URL", "STATUS", "SIG VALID", "LATENCY", colorReset))
	sb.WriteString(fmt.Sprintf("%s%s%s\n", colorGray, strings.Repeat("─", 80), colorReset))

	for _, r := range summary.Results {
		stColor := colorGreen
		switch r.Status {
		case StatusTamperDetected:
			stColor = colorRed
		case StatusSignatureInvalid:
			stColor = colorYellow
		case StatusUnattested:
			stColor = colorGray
		}

		sigStr := "FAIL"
		if r.SignatureValid {
			sigStr = "PASS"
		}

		sb.WriteString(fmt.Sprintf("%-16s %-24s %s%-16s%s %-10s %-8.2fms\n",
			truncate(r.SnapshotID, 15),
			truncate(r.URL, 23),
			stColor, r.Status, colorReset,
			sigStr,
			r.LatencyMs,
		))

		if len(r.MismatchedLeaves) > 0 {
			for _, diff := range r.MismatchedLeaves {
				sb.WriteString(fmt.Sprintf("  %s└─ ❌ %s%s\n", colorRed, diff, colorReset))
			}
		}
	}
	sb.WriteString("\n")

	return sb.String()
}

// RenderMerkleTreeMermaid generates a Mermaid flowchart of the Merkle Tree hierarchy with tamper highlighting.
func RenderMerkleTreeMermaid(receipt *AttestationReceipt, result *VerificationResult) string {
	var sb strings.Builder

	sb.WriteString("```mermaid\n")
	sb.WriteString("flowchart TD\n")

	rootColor := "fill:#d4edda,stroke:#28a745,stroke-width:2px"
	if result != nil && result.Status == StatusTamperDetected {
		rootColor = "fill:#f8d7da,stroke:#dc3545,stroke-width:2px"
	}

	rootLabel := fmt.Sprintf("Merkle Root<br/><code>%s</code>", truncateHash(receipt.MerkleRoot))
	sb.WriteString(fmt.Sprintf("    Root[\"%s\"]:::rootStyle\n", rootLabel))
	sb.WriteString(fmt.Sprintf("    classDef rootStyle %s;\n\n", rootColor))

	// Intermediate Nodes
	sb.WriteString("    Root --> H01[\"Node H01<br/>Metadata + DOM\"]\n")
	sb.WriteString("    Root --> H23[\"Node H23<br/>Markdown + Manifest\"]\n\n")

	// Leaves
	for i, leaf := range receipt.Leaves {
		leafLabel := fmt.Sprintf("%s<br/><code>%s</code>", leaf.Type, truncateHash(leaf.Hash))
		leafID := fmt.Sprintf("L%d", i)

		isTampered := false
		if result != nil {
			for _, m := range result.MismatchedLeaves {
				if strings.Contains(m, string(leaf.Type)) {
					isTampered = true
					break
				}
			}
		}

		leafStyle := "fill:#e2e3e5,stroke:#383d41"
		if isTampered {
			leafStyle = "fill:#f8d7da,stroke:#dc3545,stroke-width:3px"
		} else if result != nil && result.Status == StatusVerified {
			leafStyle = "fill:#d4edda,stroke:#28a745"
		}

		sb.WriteString(fmt.Sprintf("    %s[\"%s\"]:::%sStyle\n", leafID, leafLabel, leafID))
		sb.WriteString(fmt.Sprintf("    classDef %sStyle %s;\n", leafID, leafStyle))

		if i < 2 {
			sb.WriteString(fmt.Sprintf("    H01 --> %s\n", leafID))
		} else {
			sb.WriteString(fmt.Sprintf("    H23 --> %s\n", leafID))
		}
	}

	sb.WriteString("```\n")
	return sb.String()
}

// RenderAttestationMarkdown generates a GitHub Flavored Markdown compliance report.
func RenderAttestationMarkdown(summary AttestationSummary) string {
	var sb strings.Builder

	sb.WriteString("# TimeCapsule Cryptographic Provenance & Chain of Custody Report\n\n")
	sb.WriteString(fmt.Sprintf("*Generated at: %s*\n\n", time.Now().UTC().Format(time.RFC3339)))

	sb.WriteString("## Executive Summary\n\n")
	sb.WriteString("| Metric | Value |\n")
	sb.WriteString("| :--- | :--- |\n")
	sb.WriteString(fmt.Sprintf("| **Total Snapshots Evaluated** | `%d` |\n", summary.TotalEvaluated))
	sb.WriteString(fmt.Sprintf("| **Verified Authentic (Zero Tampering)** | `%d` |\n", summary.VerifiedCount))
	sb.WriteString(fmt.Sprintf("| **Tampering Breaches Detected** | `%d` |\n", summary.TamperCount))
	sb.WriteString(fmt.Sprintf("| **Invalid / Forged Signatures** | `%d` |\n", summary.InvalidSignatureCount))
	sb.WriteString(fmt.Sprintf("| **Integrity Verification Rate** | `%.1f%%` |\n\n",
		float64(summary.VerifiedCount)/float64(summary.TotalEvaluated)*100))

	sb.WriteString("## Detailed Snapshot Verification Breakdown\n\n")
	sb.WriteString("| Snapshot ID | Target URL | Status | Signature | Merkle Root | Latency | Tamper Details |\n")
	sb.WriteString("| :--- | :--- | :---: | :---: | :---: | :---: | :--- |\n")

	for _, r := range summary.Results {
		sigBadge := "❌ INVALID"
		if r.SignatureValid {
			sigBadge = "✅ VALID"
		}
		rootBadge := "❌ MISMATCH"
		if r.RootMatches {
			rootBadge = "✅ MATCH"
		}
		tamperDetail := "None (Clean)"
		if len(r.MismatchedLeaves) > 0 {
			tamperDetail = fmt.Sprintf("`%s`", strings.Join(r.MismatchedLeaves, "`; `"))
		}

		sb.WriteString(fmt.Sprintf("| `%s` | `%s` | **%s** | %s | %s | `%.2f ms` | %s |\n",
			r.SnapshotID,
			r.URL,
			r.Status,
			sigBadge,
			rootBadge,
			r.LatencyMs,
			tamperDetail,
		))
	}

	return sb.String()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 3 {
		return s[:maxLen]
	}
	return s[:maxLen-2] + ".."
}
