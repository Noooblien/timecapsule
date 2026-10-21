package memento

import (
	"fmt"
	"strings"
	"time"
)

// RenderScorecard formats a temporal resolution result for terminal output.
func RenderScorecard(res *TemporalResolution, originalURL, linkHeader string) string {
	sb := &strings.Builder{}
	sb.WriteString("\n")
	sb.WriteString("========================================================================================================\n")
	sb.WriteString("  TIMECAPSULE // RFC 7089 MEMENTO TEMPORAL RESOLUTION SCORECARD\n")
	sb.WriteString("========================================================================================================\n")
	sb.WriteString(fmt.Sprintf("  Original Resource (URI-R):  %s\n", originalURL))
	sb.WriteString(fmt.Sprintf("  Requested Target Time:      %s\n", res.RequestedTime.Format(RFC1123Format)))
	sb.WriteString(fmt.Sprintf("  Selected Memento (URI-M):   %s\n", res.Selected.URI))
	sb.WriteString(fmt.Sprintf("  Memento Timestamp:          %s\n", res.Selected.Datetime.Format(RFC1123Format)))
	sb.WriteString(fmt.Sprintf("  Temporal Delta:             %.1f seconds (approx %.1f hours)\n", res.DeltaSeconds, res.DeltaSeconds/3600.0))
	sb.WriteString("--------------------------------------------------------------------------------------------------------\n")
	sb.WriteString("  RFC 7089 LINK RELATIONSHIPS:\n")
	for _, line := range strings.Split(linkHeader, ",\n") {
		sb.WriteString(fmt.Sprintf("    %s\n", strings.TrimSpace(line)))
	}
	sb.WriteString("========================================================================================================\n\n")
	return sb.String()
}

// RenderMarkdownReport produces an executive Markdown summary for documentation and CI reports.
func RenderMarkdownReport(res *TemporalResolution, originalURL, linkHeader string) string {
	prevURI := "*(None - Earliest Capture)*"
	if res.Prev != nil {
		prevURI = fmt.Sprintf("[%s](%s) (%s)", res.Prev.Datetime.Format(time.RFC3339), res.Prev.URI, res.Prev.Datetime.Format(RFC1123Format))
	}
	nextURI := "*(None - Latest Capture)*"
	if res.Next != nil {
		nextURI = fmt.Sprintf("[%s](%s) (%s)", res.Next.Datetime.Format(time.RFC3339), res.Next.URI, res.Next.Datetime.Format(RFC1123Format))
	}

	return fmt.Sprintf(`### ⏳ RFC 7089 Memento Temporal Resolution Report

| Parameter | Value |
| :--- | :--- |
| **Original Resource (URI-R)** | `+"`%s`"+` |
| **Requested Target Time** | **%s** |
| **Selected Memento (URI-M)** | [%s](%s) |
| **Memento Timestamp** | **%s** |
| **Temporal Delta** | **%.1f s (%.2f hours)** |
| **Previous Memento** | %s |
| **Next Memento** | %s |
| **First Available Capture**| %s |
| **Latest Available Capture**| %s |

#### 🔗 RFC 7089 Link Headers
`+"```http"+`
Link: %s
`+"```"+`
`, originalURL, res.RequestedTime.Format(RFC1123Format), res.Selected.URI, res.Selected.URI,
		res.Selected.Datetime.Format(RFC1123Format), res.DeltaSeconds, res.DeltaSeconds/3600.0,
		prevURI, nextURI, res.First.Datetime.Format(RFC1123Format), res.Last.Datetime.Format(RFC1123Format),
		linkHeader)
}

// RenderMermaidDiagram generates an RFC 7089 temporal flow diagram.
func RenderMermaidDiagram(res *TemporalResolution, originalURL string) string {
	sb := &strings.Builder{}
	sb.WriteString("```mermaid\nsequenceDiagram\n")
	sb.WriteString("    autonumber\n")
	sb.WriteString("    participant Client as User Agent (Browser / Crawler)\n")
	sb.WriteString("    participant Gate as TimeGate (/timegate/...)\n")
	sb.WriteString("    participant Memento as Selected Memento (/web/...)\n")
	sb.WriteString("    participant Map as TimeMap (/timemap/...)\n\n")

	sb.WriteString(fmt.Sprintf("    Client->>Gate: GET /timegate/%s\n", originalURL))
	sb.WriteString(fmt.Sprintf("    Note over Client,Gate: Accept-Datetime: %s\n", res.RequestedTime.Format(RFC1123Format)))
	sb.WriteString("    Gate->>Gate: Binary Search Closest Chronological Neighbor\n")
	sb.WriteString("    Gate-->>Client: HTTP 302 Found\n")
	sb.WriteString(fmt.Sprintf("    Note over Gate,Client: Location: %s<br/>Vary: Accept-Datetime<br/>Link: rel=\"original\", rel=\"memento\", rel=\"timemap\"\n", res.Selected.URI))
	sb.WriteString(fmt.Sprintf("    Client->>Memento: GET %s\n", res.Selected.URI))
	sb.WriteString("    Memento-->>Client: HTTP 200 OK (Historical Snapshot Body)\n")
	sb.WriteString(fmt.Sprintf("    Client->>Map: GET /timemap/link/%s\n", originalURL))
	sb.WriteString("    Map-->>Client: HTTP 200 OK (application/link-format)\n")
	sb.WriteString("```\n")
	return sb.String()
}
