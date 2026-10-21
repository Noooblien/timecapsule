package memento

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Supported HTTP date parsing layouts.
var dateLayouts = []string{
	time.RFC1123,
	time.RFC1123Z,
	time.RFC850,
	time.ANSIC,
	"2006-01-02T15:04:05Z07:00",
	"20060102150405", // Wayback 14-digit compact format
}

// ParseHTTPDate parses an HTTP datetime string according to RFC 7231 / RFC 7089.
func ParseHTTPDate(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("empty datetime string")
	}

	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse datetime '%s': unsupported layout", raw)
}

// TimeGate manages temporal negotiation across historical mementos.
type TimeGate struct {
	OriginalURL string
	BaseURL     string
	mementos    []*Memento
}

// NewTimeGate constructs a TimeGate with chronological mementos.
func NewTimeGate(originalURL, baseURL string, items []*Memento) *TimeGate {
	tg := &TimeGate{
		OriginalURL: strings.TrimSpace(originalURL),
		BaseURL:     strings.TrimRight(baseURL, "/"),
		mementos:    make([]*Memento, 0, len(items)),
	}

	for _, item := range items {
		if item != nil && !item.Datetime.IsZero() {
			tg.mementos = append(tg.mementos, item)
		}
	}

	// Sort chronologically ascending
	sort.Slice(tg.mementos, func(i, j int) bool {
		return tg.mementos[i].Datetime.Before(tg.mementos[j].Datetime)
	})

	return tg
}

// Count returns the number of registered mementos.
func (tg *TimeGate) Count() int {
	return len(tg.mementos)
}

// Resolve negotiates the closest memento to the target requested time (RFC 7089 Section 2.2).
func (tg *TimeGate) Resolve(targetTime time.Time) (*TemporalResolution, error) {
	if len(tg.mementos) == 0 {
		return nil, fmt.Errorf("no mementos available for %s", tg.OriginalURL)
	}

	targetTime = targetTime.UTC()
	first := tg.mementos[0]
	last := tg.mementos[len(tg.mementos)-1]

	// Exact / Boundary cases
	if !targetTime.After(first.Datetime) {
		res := &TemporalResolution{
			RequestedTime: targetTime,
			Selected:      first,
			First:         first,
			Last:          last,
			DeltaSeconds:  math.Abs(first.Datetime.Sub(targetTime).Seconds()),
		}
		if len(tg.mementos) > 1 {
			res.Next = tg.mementos[1]
		}
		return res, nil
	}

	if !targetTime.Before(last.Datetime) {
		res := &TemporalResolution{
			RequestedTime: targetTime,
			Selected:      last,
			First:         first,
			Last:          last,
			DeltaSeconds:  math.Abs(last.Datetime.Sub(targetTime).Seconds()),
		}
		if len(tg.mementos) > 1 {
			res.Prev = tg.mementos[len(tg.mementos)-2]
		}
		return res, nil
	}

	// Binary search for closest temporal neighbor
	idx := sort.Search(len(tg.mementos), func(i int) bool {
		return !tg.mementos[i].Datetime.Before(targetTime)
	})

	// Compare candidate at idx and idx-1
	selectedIdx := idx
	if idx > 0 {
		diffCurr := targetTime.Sub(tg.mementos[idx-1].Datetime)
		diffNext := tg.mementos[idx].Datetime.Sub(targetTime)
		if diffCurr <= diffNext {
			selectedIdx = idx - 1
		}
	}

	selected := tg.mementos[selectedIdx]
	res := &TemporalResolution{
		RequestedTime: targetTime,
		Selected:      selected,
		First:         first,
		Last:          last,
		DeltaSeconds:  math.Abs(selected.Datetime.Sub(targetTime).Seconds()),
	}

	if selectedIdx > 0 {
		res.Prev = tg.mementos[selectedIdx-1]
	}
	if selectedIdx < len(tg.mementos)-1 {
		res.Next = tg.mementos[selectedIdx+1]
	}

	return res, nil
}

// BuildLinkHeader formats RFC 7089 Section 2.1 compliant Link header relationships.
func (tg *TimeGate) BuildLinkHeader(resolution *TemporalResolution) string {
	timeGateURI := fmt.Sprintf("%s/timegate/%s", tg.BaseURL, tg.OriginalURL)
	timeMapURI := fmt.Sprintf("%s/timemap/link/%s", tg.BaseURL, tg.OriginalURL)

	var links []string
	links = append(links, fmt.Sprintf("<%s>; rel=\"original\"", tg.OriginalURL))
	links = append(links, fmt.Sprintf("<%s>; rel=\"timegate\"", timeGateURI))
	links = append(links, fmt.Sprintf("<%s>; rel=\"timemap\"; type=\"%s\"", timeMapURI, LinkFormatMimeType))

	if resolution == nil || resolution.Selected == nil {
		return strings.Join(links, ", ")
	}

	sel := resolution.Selected
	first := resolution.First
	last := resolution.Last

	// Selected memento relation
	selRel := "memento"
	if sel.URI == first.URI && sel.URI == last.URI {
		selRel = "first last memento"
	} else if sel.URI == first.URI {
		selRel = "first memento"
	} else if sel.URI == last.URI {
		selRel = "last memento"
	}

	links = append(links, fmt.Sprintf("<%s>; rel=\"%s\"; datetime=\"%s\"",
		sel.URI, selRel, sel.Datetime.Format(RFC1123Format)))

	if resolution.Prev != nil {
		rel := "prev memento"
		if resolution.Prev.URI == first.URI {
			rel = "first prev memento"
		}
		links = append(links, fmt.Sprintf("<%s>; rel=\"%s\"; datetime=\"%s\"",
			resolution.Prev.URI, rel, resolution.Prev.Datetime.Format(RFC1123Format)))
	}

	if resolution.Next != nil {
		rel := "next memento"
		if resolution.Next.URI == last.URI {
			rel = "next last memento"
		}
		links = append(links, fmt.Sprintf("<%s>; rel=\"%s\"; datetime=\"%s\"",
			resolution.Next.URI, rel, resolution.Next.Datetime.Format(RFC1123Format)))
	}

	// Always guarantee first and last are listed
	if resolution.Prev == nil && sel.URI != first.URI {
		links = append(links, fmt.Sprintf("<%s>; rel=\"first memento\"; datetime=\"%s\"",
			first.URI, first.Datetime.Format(RFC1123Format)))
	}
	if resolution.Next == nil && sel.URI != last.URI {
		links = append(links, fmt.Sprintf("<%s>; rel=\"last memento\"; datetime=\"%s\"",
			last.URI, last.Datetime.Format(RFC1123Format)))
	}

	return strings.Join(links, ",\n ")
}
