package timeline

import (
	"fmt"
	"sort"
	"time"

	"github.com/Noooblien/timecapsule/pkg/archive"
)

// ReplayPoint represents a single archived capture plotted on a temporal timeline.
type ReplayPoint struct {
	ID          string    `json:"id"`
	WaybackTS   string    `json:"wayback_ts"`
	Timestamp   time.Time `json:"timestamp"`
	Title       string    `json:"title"`
	StatusCode  int       `json:"status_code"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	AssetCount  int       `json:"asset_count"`
	ContentHash string    `json:"content_hash"`
	ReplayURL   string    `json:"replay_url"`
	DiffURL     string    `json:"diff_url,omitempty"`
	VDiffURL    string    `json:"vdiff_url,omitempty"`
	ReaderURL   string    `json:"reader_url"`
	RawURL      string    `json:"raw_url"`
	WARCURL     string    `json:"warc_url"`
	BundleURL   string    `json:"bundle_url"`
	IsCurrent   bool      `json:"is_current,omitempty"`
}

// YearHistogram aggregates capture frequencies by year and month.
type YearHistogram struct {
	Year   int     `json:"year"`
	Total  int     `json:"total"`
	Months [12]int `json:"months"` // 0 = Jan, 11 = Dec
}

// URLTimeline represents the full temporal history for a specific URL.
type URLTimeline struct {
	URL           string          `json:"url"`
	FirstCapture  time.Time       `json:"first_capture"`
	LastCapture   time.Time       `json:"last_capture"`
	TotalCaptures int             `json:"total_captures"`
	Years         []YearHistogram `json:"years"`
	Captures      []ReplayPoint   `json:"captures"`
}

// NavContext provides relative temporal navigation (Prev, Next, First, Last) around an active capture.
type NavContext struct {
	Current  ReplayPoint  `json:"current"`
	First    *ReplayPoint `json:"first,omitempty"`
	Prev     *ReplayPoint `json:"prev,omitempty"`
	Next     *ReplayPoint `json:"next,omitempty"`
	Last     *ReplayPoint `json:"last,omitempty"`
	Total    int          `json:"total"`
	Position int          `json:"position"` // 1-based index
	Timeline *URLTimeline `json:"timeline,omitempty"`
}

// BuildTimeline constructs a URLTimeline from a slice of snapshots for a target URL.
func BuildTimeline(baseURL, targetURL string, snapshots []*archive.Snapshot) *URLTimeline {
	var matches []*archive.Snapshot
	for _, s := range snapshots {
		if s.URL == targetURL {
			matches = append(matches, s)
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Timestamp.Before(matches[j].Timestamp)
	})

	tl := &URLTimeline{
		URL:           targetURL,
		TotalCaptures: len(matches),
	}

	if len(matches) == 0 {
		return tl
	}

	tl.FirstCapture = matches[0].Timestamp
	tl.LastCapture = matches[len(matches)-1].Timestamp

	yearMap := make(map[int]*YearHistogram)

	for i, s := range matches {
		pt := SnapshotToReplayPoint(baseURL, s)
		if i > 0 {
			pt.DiffURL = fmt.Sprintf("%s/diff/%s/%s", baseURL, matches[i-1].ID, s.ID)
			pt.VDiffURL = fmt.Sprintf("%s/vdiff/%s/%s", baseURL, matches[i-1].ID, s.ID)
		}
		tl.Captures = append(tl.Captures, pt)

		y := s.Timestamp.Year()
		m := int(s.Timestamp.Month()) - 1
		if _, ok := yearMap[y]; !ok {
			yearMap[y] = &YearHistogram{Year: y}
		}
		yearMap[y].Total++
		if m >= 0 && m < 12 {
			yearMap[y].Months[m]++
		}
	}

	for _, yh := range yearMap {
		tl.Years = append(tl.Years, *yh)
	}
	sort.Slice(tl.Years, func(i, j int) bool {
		return tl.Years[i].Year < tl.Years[j].Year
	})

	return tl
}

// BuildNavContext computes adjacent navigation points for a given active snapshot.
func BuildNavContext(baseURL string, snapshots []*archive.Snapshot, currentSnap *archive.Snapshot) *NavContext {
	tl := BuildTimeline(baseURL, currentSnap.URL, snapshots)
	ctx := &NavContext{
		Current:  SnapshotToReplayPoint(baseURL, currentSnap),
		Total:    len(tl.Captures),
		Timeline: tl,
	}

	for i, c := range tl.Captures {
		if c.ID == currentSnap.ID || c.WaybackTS == currentSnap.WaybackTS {
			ctx.Position = i + 1
			ctx.Current.IsCurrent = true
			if i > 0 {
				prev := tl.Captures[i-1]
				ctx.Prev = &prev
			}
			if i < len(tl.Captures)-1 {
				next := tl.Captures[i+1]
				ctx.Next = &next
			}
			if len(tl.Captures) > 1 {
				first := tl.Captures[0]
				last := tl.Captures[len(tl.Captures)-1]
				if first.ID != currentSnap.ID {
					ctx.First = &first
				}
				if last.ID != currentSnap.ID {
					ctx.Last = &last
				}
			}
			break
		}
	}

	return ctx
}

// SnapshotToReplayPoint converts an archive.Snapshot to ReplayPoint with precomputed links.
func SnapshotToReplayPoint(baseURL string, s *archive.Snapshot) ReplayPoint {
	return ReplayPoint{
		ID:          s.ID,
		WaybackTS:   s.WaybackTS,
		Timestamp:   s.Timestamp,
		Title:       s.Title,
		StatusCode:  s.StatusCode,
		ContentType: s.ContentType,
		SizeBytes:   s.SizeBytes,
		AssetCount:  s.AssetCount,
		ContentHash: s.ContentHash,
		ReplayURL:   fmt.Sprintf("%s/web/%s/%s", baseURL, s.WaybackTS, s.URL),
		ReaderURL:   fmt.Sprintf("%s/reader/%s", baseURL, s.ID),
		RawURL:      fmt.Sprintf("%s/raw/%s", baseURL, s.ID),
		WARCURL:     fmt.Sprintf("%s/warc/%s", baseURL, s.ID),
		BundleURL:   fmt.Sprintf("%s/bundle/%s", baseURL, s.ID),
	}
}
