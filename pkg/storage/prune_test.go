package storage

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Noooblien/timecapsule/pkg/archive"
)

func TestStorage_PruneAndRetention(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "timecapsule-prune-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	now := time.Now().UTC()
	urlA := "https://news.ycombinator.com"
	urlB := "https://go.dev"

	snaps := []*archive.Snapshot{
		// 1. urlA - oldest, unique hash
		{
			ID:          "snap_a1",
			URL:         urlA,
			Timestamp:   now.Add(-48 * time.Hour),
			WaybackTS:   archive.FormatWaybackTS(now.Add(-48 * time.Hour)),
			Title:       "HN Day 1",
			HTML:        "<html>content 1</html>",
			ContentHash: "hash_1111",
			SizeBytes:   2048,
		},
		// 2. urlA - day 2, duplicate hash with day 3
		{
			ID:          "snap_a2",
			URL:         urlA,
			Timestamp:   now.Add(-24 * time.Hour),
			WaybackTS:   archive.FormatWaybackTS(now.Add(-24 * time.Hour)),
			Title:       "HN Day 2",
			HTML:        "<html>content 2</html>",
			ContentHash: "hash_2222",
			SizeBytes:   4096,
		},
		// 3. urlA - day 2.5, exact duplicate of snap_a2
		{
			ID:          "snap_a3",
			URL:         urlA,
			Timestamp:   now.Add(-12 * time.Hour),
			WaybackTS:   archive.FormatWaybackTS(now.Add(-12 * time.Hour)),
			Title:       "HN Day 2 (Duplicate)",
			HTML:        "<html>content 2</html>",
			ContentHash: "hash_2222",
			SizeBytes:   4096,
		},
		// 4. urlA - latest, unique hash
		{
			ID:          "snap_a4",
			URL:         urlA,
			Timestamp:   now,
			WaybackTS:   archive.FormatWaybackTS(now),
			Title:       "HN Day 3",
			HTML:        "<html>content 3</html>",
			ContentHash: "hash_3333",
			SizeBytes:   8192,
		},
		// 5. urlB - independent URL
		{
			ID:          "snap_b1",
			URL:         urlB,
			Timestamp:   now,
			WaybackTS:   archive.FormatWaybackTS(now),
			Title:       "Go Dev",
			HTML:        "<html>go dev</html>",
			ContentHash: "hash_bbbb",
			SizeBytes:   1024,
		},
	}

	for _, s := range snaps {
		if err := store.Save(s); err != nil {
			t.Fatalf("failed to save snapshot: %v", err)
		}
	}

	// Verify Stats
	stats := store.Stats()
	if stats.TotalSnapshots != 5 {
		t.Fatalf("expected 5 snapshots, got %d", stats.TotalSnapshots)
	}
	if stats.UniqueURLs != 2 {
		t.Fatalf("expected 2 unique urls, got %d", stats.UniqueURLs)
	}

	// 1. DryRun Deduplication
	repDedup, err := store.Prune(PruneOptions{
		Deduplicate: true,
		DryRun:      true,
	})
	if err != nil {
		t.Fatalf("prune failed: %v", err)
	}
	if len(repDedup.Candidates) != 1 || repDedup.Candidates[0].ID != "snap_a3" {
		t.Fatalf("expected duplicate snap_a3 flagged, got: %+v", repDedup.Candidates)
	}
	// Verify dry run didn't delete
	if _, ok := store.Get("snap_a3"); !ok {
		t.Fatalf("dry run must not delete snapshots")
	}

	// 2. MaxAge Pruning
	repAge, err := store.Prune(PruneOptions{
		MaxAge: 36 * time.Hour,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("prune failed: %v", err)
	}
	if len(repAge.Candidates) != 1 || repAge.Candidates[0].ID != "snap_a1" {
		t.Fatalf("expected snap_a1 flagged by age, got: %+v", repAge.Candidates)
	}

	// 3. KeepPerURL Pruning (Keep 2 per URL on urlA)
	repKeep, err := store.Prune(PruneOptions{
		TargetURL:  urlA,
		KeepPerURL: 2,
		DryRun:     false,
	})
	if err != nil {
		t.Fatalf("prune failed: %v", err)
	}
	if repKeep.DeletedCount != 2 {
		t.Fatalf("expected 2 deleted snapshots for urlA with KeepPerURL=2, got %d", repKeep.DeletedCount)
	}

	// Remaining for urlA should be snap_a3 and snap_a4
	historyA := store.GetHistory(urlA)
	if len(historyA) != 2 {
		t.Fatalf("expected 2 remaining snapshots for urlA, got %d", len(historyA))
	}

	// urlB should remain untouched
	historyB := store.GetHistory(urlB)
	if len(historyB) != 1 {
		t.Fatalf("expected 1 snapshot for urlB, got %d", len(historyB))
	}

	// Test scorecard output
	scorecard := RenderPruneScorecard(repKeep)
	if !strings.Contains(scorecard, "TimeCapsule Storage Pruning") {
		t.Fatalf("unexpected scorecard: %s", scorecard)
	}
}
