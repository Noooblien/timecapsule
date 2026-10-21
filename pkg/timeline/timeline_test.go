package timeline

import (
	"strings"
	"testing"
	"time"

	"github.com/Noooblien/timecapsule/pkg/archive"
)

func TestTimeline_BuildTimelineAndHistograms(t *testing.T) {
	baseURL := "http://localhost:8080"
	targetURL := "https://example.com"
	t1 := time.Date(2024, 1, 10, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	t3 := time.Date(2024, 5, 20, 14, 0, 0, 0, time.UTC)
	t4 := time.Date(2025, 3, 1, 9, 0, 0, 0, time.UTC)

	snaps := []*archive.Snapshot{
		{ID: "s4", URL: targetURL, Timestamp: t4, WaybackTS: "20250301090000", ContentHash: "hash4", SizeBytes: 4000},
		{ID: "s1", URL: targetURL, Timestamp: t1, WaybackTS: "20240110100000", ContentHash: "hash1", SizeBytes: 1000},
		{ID: "s3", URL: targetURL, Timestamp: t3, WaybackTS: "20240520140000", ContentHash: "hash3", SizeBytes: 3000},
		{ID: "s2", URL: targetURL, Timestamp: t2, WaybackTS: "20240115120000", ContentHash: "hash2", SizeBytes: 2000},
		{ID: "other", URL: "https://other.org", Timestamp: t1, WaybackTS: "20240110100000", ContentHash: "other", SizeBytes: 500},
	}

	tl := BuildTimeline(baseURL, targetURL, snaps)
	if tl.TotalCaptures != 4 {
		t.Fatalf("expected 4 captures, got %d", tl.TotalCaptures)
	}

	// Verify chronological sort
	if tl.Captures[0].ID != "s1" || tl.Captures[3].ID != "s4" {
		t.Fatalf("captures not sorted chronologically: %+v", tl.Captures)
	}

	// Verify diff urls
	if tl.Captures[0].DiffURL != "" {
		t.Fatalf("first capture should not have diff URL")
	}
	if !strings.Contains(tl.Captures[1].DiffURL, "/diff/s1/s2") {
		t.Fatalf("expected diff URL /diff/s1/s2, got %s", tl.Captures[1].DiffURL)
	}

	// Verify years aggregation
	if len(tl.Years) != 2 {
		t.Fatalf("expected 2 years, got %d", len(tl.Years))
	}
	if tl.Years[0].Year != 2024 || tl.Years[0].Total != 3 {
		t.Fatalf("unexpected year 2024 stats: %+v", tl.Years[0])
	}
	if tl.Years[0].Months[0] != 2 { // Jan
		t.Fatalf("expected 2 in Jan 2024, got %d", tl.Years[0].Months[0])
	}
	if tl.Years[0].Months[4] != 1 { // May
		t.Fatalf("expected 1 in May 2024, got %d", tl.Years[0].Months[4])
	}
}

func TestTimeline_NavContext(t *testing.T) {
	baseURL := "http://localhost:8080"
	targetURL := "https://example.com"
	t1 := time.Date(2024, 1, 10, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 2, 10, 10, 0, 0, 0, time.UTC)
	t3 := time.Date(2024, 3, 10, 10, 0, 0, 0, time.UTC)

	snaps := []*archive.Snapshot{
		{ID: "s1", URL: targetURL, Timestamp: t1, WaybackTS: "20240110100000", ContentHash: "hash1"},
		{ID: "s2", URL: targetURL, Timestamp: t2, WaybackTS: "20240210100000", ContentHash: "hash2"},
		{ID: "s3", URL: targetURL, Timestamp: t3, WaybackTS: "20240310100000", ContentHash: "hash3"},
	}

	// Middle capture navigation
	nav2 := BuildNavContext(baseURL, snaps, snaps[1])
	if nav2.Position != 2 || nav2.Total != 3 {
		t.Fatalf("expected pos 2 of 3, got %d of %d", nav2.Position, nav2.Total)
	}
	if nav2.Prev == nil || nav2.Prev.ID != "s1" {
		t.Fatalf("expected prev s1, got %+v", nav2.Prev)
	}
	if nav2.Next == nil || nav2.Next.ID != "s3" {
		t.Fatalf("expected next s3, got %+v", nav2.Next)
	}
	if nav2.First == nil || nav2.First.ID != "s1" {
		t.Fatalf("expected first s1, got %+v", nav2.First)
	}
	if nav2.Last == nil || nav2.Last.ID != "s3" {
		t.Fatalf("expected last s3, got %+v", nav2.Last)
	}

	bannerHTML := RenderWaybackBanner(nav2)
	if !strings.Contains(bannerHTML, "Capture 2 of 3") {
		t.Fatalf("banner missing position indicator: %s", bannerHTML)
	}
	if !strings.Contains(bannerHTML, "Diff ⇄") {
		t.Fatalf("banner missing diff link: %s", bannerHTML)
	}

	// First capture navigation
	nav1 := BuildNavContext(baseURL, snaps, snaps[0])
	if nav1.Prev != nil {
		t.Fatalf("first capture should have nil prev, got %+v", nav1.Prev)
	}
	if nav1.Next == nil || nav1.Next.ID != "s2" {
		t.Fatalf("expected next s2, got %+v", nav1.Next)
	}
}

func TestTimeline_RenderingAndDemo(t *testing.T) {
	demo := GenerateDemoTimeline("https://example.org")
	if demo.TotalCaptures != 5 {
		t.Fatalf("expected 5 demo captures, got %d", demo.TotalCaptures)
	}

	ascii := RenderASCIITimeline(demo)
	if !strings.Contains(ascii, "Temporal Archival Timeline") || !strings.Contains(ascii, "Capture Distribution Matrix") {
		t.Fatalf("invalid ASCII output: %s", ascii)
	}

	mermaid := RenderTimelineMermaid(demo)
	if !strings.Contains(mermaid, "timeline") || !strings.Contains(mermaid, "Temporal Archival History") {
		t.Fatalf("invalid Mermaid output: %s", mermaid)
	}

	pageHTML := RenderTimelinePage(demo)
	if !strings.Contains(pageHTML, "Temporal Timeline") || !strings.Contains(pageHTML, "Total Historical Captures") {
		t.Fatalf("invalid HTML output: %s", pageHTML)
	}
}
