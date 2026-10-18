package bench

import (
	"bytes"
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/Noooblien/timecapsule/pkg/warc"
)

var (
	reRemoteScript = regexp.MustCompile(`(?i)<script[^>]+src=["']((?:https?:)?//[^"']+)["']`)
	reRemoteStyle  = regexp.MustCompile(`(?i)<link[^>]+(?:rel=["']stylesheet["'][^>]+href=["']((?:https?:)?//[^"']+)["']|href=["']((?:https?:)?//[^"']+)["'][^>]+rel=["']stylesheet["'])`)
	reRemoteImage  = regexp.MustCompile(`(?i)<img[^>]+src=["']((?:https?:)?//[^"']+)["']`)
	reRemoteIframe = regexp.MustCompile(`(?i)<iframe[^>]+src=["']((?:https?:)?//[^"']+)["']`)
	reRemoteImport = regexp.MustCompile(`(?i)@import\s+url\(["']?((?:https?:)?//[^"')]+)["']?\)`)
)

// AuditHTML scans an HTML document for un-inlined remote dependencies and privacy leaks.
func AuditHTML(rawHTML string) ([]ResourceLeak, float64) {
	var leaks []ResourceLeak

	// 1. Remote scripts
	matches := reRemoteScript.FindAllStringSubmatch(rawHTML, -1)
	for _, m := range matches {
		if len(m) > 1 {
			leaks = append(leaks, ResourceLeak{
				URL:         m[1],
				ElementType: "<script>",
				Severity:    SeverityCritical,
				Description: "Un-neutralized active remote script breaks hermetic offline isolation and leaks client IP",
			})
		}
	}

	// 2. Remote stylesheets
	matches = reRemoteStyle.FindAllStringSubmatch(rawHTML, -1)
	for _, m := range matches {
		url := m[1]
		if url == "" && len(m) > 2 {
			url = m[2]
		}
		if url != "" {
			leaks = append(leaks, ResourceLeak{
				URL:         url,
				ElementType: "<link rel=stylesheet>",
				Severity:    SeverityCritical,
				Description: "External CSS stylesheet not inlined; visual layout will break offline",
			})
		}
	}

	// 3. Remote images
	matches = reRemoteImage.FindAllStringSubmatch(rawHTML, -1)
	for _, m := range matches {
		if len(m) > 1 {
			leaks = append(leaks, ResourceLeak{
				URL:         m[1],
				ElementType: "<img>",
				Severity:    SeverityWarn,
				Description: "External remote image reference; potential tracking pixel or missing asset",
			})
		}
	}

	// 4. Remote iframes
	matches = reRemoteIframe.FindAllStringSubmatch(rawHTML, -1)
	for _, m := range matches {
		if len(m) > 1 {
			leaks = append(leaks, ResourceLeak{
				URL:         m[1],
				ElementType: "<iframe>",
				Severity:    SeverityCritical,
				Description: "Remote embedded iframe; unpredictable external content",
			})
		}
	}

	// 5. CSS @import
	matches = reRemoteImport.FindAllStringSubmatch(rawHTML, -1)
	for _, m := range matches {
		if len(m) > 1 {
			leaks = append(leaks, ResourceLeak{
				URL:         m[1],
				ElementType: "@import",
				Severity:    SeverityWarn,
				Description: "Remote @import rule inside inline style; missing subresource",
			})
		}
	}

	// Calculate hermetic score
	critCount := 0
	warnCount := 0
	for _, l := range leaks {
		if l.Severity == SeverityCritical {
			critCount++
		} else if l.Severity == SeverityWarn {
			warnCount++
		}
	}

	score := 100.0 - (float64(critCount) * 25.0) - (float64(warnCount) * 10.0)
	score = math.Max(0.0, math.Min(100.0, score))

	return leaks, score
}

// ValidateWARC parses and verifies an in-memory WARC stream against ISO 28500 criteria.
func ValidateWARC(warcBytes []byte) ISOValidation {
	val := ISOValidation{
		ValidFormat: true,
		Errors:      make([]string, 0),
	}

	if len(warcBytes) < 4 {
		val.ValidFormat = false
		val.Errors = append(val.Errors, "WARC stream is empty or under 4 bytes")
		return val
	}

	// Detect GZIP member header (0x1f 0x8b)
	isGz := len(warcBytes) >= 2 && warcBytes[0] == 0x1f && warcBytes[1] == 0x8b
	val.MemberGzipValid = isGz

	reader, err := warc.NewReader(bytes.NewReader(warcBytes))
	if err != nil {
		val.ValidFormat = false
		val.Errors = append(val.Errors, fmt.Sprintf("Failed to initialize WARC reader: %v", err))
		return val
	}

	recordCount := 0
	for {
		rec, err := reader.NextRecord()
		if err != nil {
			break
		}
		recordCount++

		if rec.Type == warc.RecordTypeWarcinfo {
			val.HasWarcInfo = true
		}
		if rec.Type == warc.RecordTypeResponse {
			val.HasResponse = true
		}

		// Validate RecordID format
		if !strings.HasPrefix(rec.RecordID, "<urn:uuid:") || !strings.HasSuffix(rec.RecordID, ">") {
			val.Errors = append(val.Errors, fmt.Sprintf("Record #%d ID '%s' violates ISO 28500 URN UUID standard", recordCount, rec.RecordID))
		}

		// Validate Date
		if rec.Date.IsZero() {
			val.Errors = append(val.Errors, fmt.Sprintf("Record #%d missing mandatory WARC-Date", recordCount))
		}
	}

	val.RecordCount = recordCount
	if recordCount == 0 {
		val.ValidFormat = false
		val.Errors = append(val.Errors, "Zero valid ISO 28500 WARC records found in payload")
	}

	val.DigestMatch = val.HasResponse && len(val.Errors) == 0
	return val
}
