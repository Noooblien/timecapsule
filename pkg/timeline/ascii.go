package timeline

import (
	"fmt"
	"strings"
	"time"

	"github.com/Noooblien/timecapsule/pkg/archive"
)

// RenderASCIITimeline generates a terminal-friendly chronological timeline diagram.
func RenderASCIITimeline(tl *URLTimeline) string {
	if tl == nil || len(tl.Captures) == 0 {
		return "No captures on timeline."
	}

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("\033[1;36m⏳ Temporal Archival Timeline for %s\033[0m\n", tl.URL))
	sb.WriteString(fmt.Sprintf("Total Captures: \033[1;32m%d\033[0m | Range: %s -> %s\n",
		tl.TotalCaptures,
		tl.FirstCapture.Format("2006-01-02 15:04"),
		tl.LastCapture.Format("2006-01-02 15:04")))
	sb.WriteString(strings.Repeat("=", 85) + "\n\n")

	// 1. Year / Month distribution matrix
	sb.WriteString("\033[1;33m[ Capture Distribution Matrix ]\033[0m\n")
	sb.WriteString("Year   | Jan Feb Mar Apr May Jun Jul Aug Sep Oct Nov Dec | Total\n")
	sb.WriteString("-------+-------------------------------------------------+------\n")
	for _, y := range tl.Years {
		var mParts []string
		for _, cnt := range y.Months {
			if cnt == 0 {
				mParts = append(mParts, "  -")
			} else {
				mParts = append(mParts, fmt.Sprintf("%3d", cnt))
			}
		}
		sb.WriteString(fmt.Sprintf("%4d   |%s | \033[1;32m%4d\033[0m\n", y.Year, strings.Join(mParts, ""), y.Total))
	}
	sb.WriteString("\n")

	// 2. Chronological sequence
	sb.WriteString("\033[1;33m[ Historical Capture Sequence ]\033[0m\n")
	for i, c := range tl.Captures {
		isFirst := (i == 0)
		isLast := (i == len(tl.Captures)-1)
		
		marker := "|--"
		if isLast {
			marker = "`--"
		}

		tag := ""
		if isFirst && isLast {
			tag = " \033[1;35m[ONLY]\033[0m"
		} else if isFirst {
			tag = " \033[1;34m[EARLIEST]\033[0m"
		} else if isLast {
			tag = " \033[1;32m[LATEST]\033[0m"
		}

		hashShort := c.ContentHash
		if len(hashShort) > 8 {
			hashShort = hashShort[:8]
		}

		diffNote := ""
		if c.DiffURL != "" {
			diffNote = " \033[36m(diff available)\033[0m"
		}

		sb.WriteString(fmt.Sprintf("%s \033[1;37m#%-2d\033[0m [%s] Status: %d | Size: %6d B | SHA: %s%s%s\n",
			marker, i+1, c.Timestamp.Format("2006-01-02 15:04:05 MST"),
			c.StatusCode, c.SizeBytes, hashShort, tag, diffNote))
	}
	sb.WriteString("\n")
	return sb.String()
}

// RenderTimelineMermaid generates a Mermaid timeline / sequence chart diagram.
func RenderTimelineMermaid(tl *URLTimeline) string {
	if tl == nil || len(tl.Captures) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("```mermaid\n")
	sb.WriteString("timeline\n")
	sb.WriteString(fmt.Sprintf("    title Temporal Archival History: %s\n", tl.URL))

	for _, y := range tl.Years {
		sb.WriteString(fmt.Sprintf("    %d : %d captures preserved\n", y.Year, y.Total))
	}

	for i, c := range tl.Captures {
		dt := c.Timestamp.Format("2006-01-02 15:04")
		label := fmt.Sprintf("Snap #%d (%s, %d bytes)", i+1, c.WaybackTS, c.SizeBytes)
		sb.WriteString(fmt.Sprintf("    %s : %s\n", dt, label))
	}

	sb.WriteString("```\n")
	return sb.String()
}

// GenerateDemoTimeline constructs a rich multi-year mock timeline for demo/CLI evaluation.
func GenerateDemoTimeline(targetURL string) *URLTimeline {
	now := time.Now().UTC()
	baseSnaps := []*archive.Snapshot{
		{
			ID:          "tc_demo_20240115",
			URL:         targetURL,
			Timestamp:   now.AddDate(-2, -3, 0),
			WaybackTS:   archive.FormatWaybackTS(now.AddDate(-2, -3, 0)),
			Title:       "Example Domain - 2024 Launch",
			StatusCode:  200,
			ContentType: "text/html; charset=utf-8",
			SizeBytes:   45120,
			AssetCount:  12,
			ContentHash: "a1b2c3d4e5f67890",
		},
		{
			ID:          "tc_demo_20240820",
			URL:         targetURL,
			Timestamp:   now.AddDate(-2, 4, 5),
			WaybackTS:   archive.FormatWaybackTS(now.AddDate(-2, 4, 5)),
			Title:       "Example Domain - Summer Revamp",
			StatusCode:  200,
			ContentType: "text/html; charset=utf-8",
			SizeBytes:   58430,
			AssetCount:  18,
			ContentHash: "b2c3d4e5f6a17890",
		},
		{
			ID:          "tc_demo_20250310",
			URL:         targetURL,
			Timestamp:   now.AddDate(-1, -2, 0),
			WaybackTS:   archive.FormatWaybackTS(now.AddDate(-1, -2, 0)),
			Title:       "Example Domain - v2 Modern UI",
			StatusCode:  200,
			ContentType: "text/html; charset=utf-8",
			SizeBytes:   71200,
			AssetCount:  24,
			ContentHash: "c3d4e5f6a1b27890",
		},
		{
			ID:          "tc_demo_20251105",
			URL:         targetURL,
			Timestamp:   now.AddDate(-1, 6, 0),
			WaybackTS:   archive.FormatWaybackTS(now.AddDate(-1, 6, 0)),
			Title:       "Example Domain - Autumn Update",
			StatusCode:  200,
			ContentType: "text/html; charset=utf-8",
			SizeBytes:   79400,
			AssetCount:  27,
			ContentHash: "d4e5f6a1b2c37890",
		},
		{
			ID:          "tc_demo_20261018",
			URL:         targetURL,
			Timestamp:   now,
			WaybackTS:   archive.FormatWaybackTS(now),
			Title:       "Example Domain - Current Hermetic Release",
			StatusCode:  200,
			ContentType: "text/html; charset=utf-8",
			SizeBytes:   94120,
			AssetCount:  31,
			ContentHash: "e5f6a1b2c3d47890",
		},
	}

	return BuildTimeline("http://localhost:8080", targetURL, baseSnaps)
}
