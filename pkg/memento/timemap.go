package memento

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// NewTimeMap constructs a sorted TimeMap for the target resource.
func NewTimeMap(originalURL, baseURL string, items []*Memento) *TimeMap {
	cleanOriginal := strings.TrimSpace(originalURL)
	cleanBase := strings.TrimRight(baseURL, "/")

	tm := &TimeMap{
		OriginalURL: cleanOriginal,
		TimeGateURI: fmt.Sprintf("%s/timegate/%s", cleanBase, cleanOriginal),
		TimeMapURI:  fmt.Sprintf("%s/timemap/link/%s", cleanBase, cleanOriginal),
		Mementos:    make([]*Memento, 0, len(items)),
	}

	for _, item := range items {
		if item != nil && !item.Datetime.IsZero() {
			tm.Mementos = append(tm.Mementos, item)
		}
	}

	sort.Slice(tm.Mementos, func(i, j int) bool {
		return tm.Mementos[i].Datetime.Before(tm.Mementos[j].Datetime)
	})

	return tm
}

// FormatLinkFormat renders the TimeMap in the canonical RFC 7089 Section 5 (RFC 5988 application/link-format).
func (tm *TimeMap) FormatLinkFormat() string {
	var lines []string

	// Original resource
	lines = append(lines, fmt.Sprintf("<%s>; rel=\"original\"", tm.OriginalURL))

	// Self timemap anchor
	lines = append(lines, fmt.Sprintf("<%s>; rel=\"self\"; type=\"%s\"", tm.TimeMapURI, LinkFormatMimeType))

	// TimeGate
	lines = append(lines, fmt.Sprintf("<%s>; rel=\"timegate\"", tm.TimeGateURI))

	n := len(tm.Mementos)
	for i, m := range tm.Mementos {
		rel := "memento"
		if n == 1 {
			rel = "first last memento"
		} else if i == 0 {
			rel = "first memento"
		} else if i == n-1 {
			rel = "last memento"
		}

		line := fmt.Sprintf("<%s>; rel=\"%s\"; datetime=\"%s\"",
			m.URI, rel, m.Datetime.Format(RFC1123Format))

		lines = append(lines, line)
	}

	return strings.Join(lines, ",\n") + "\n"
}

// JSONTimeMap represents standard structured JSON export for modern API clients.
type JSONTimeMap struct {
	OriginalURL string           `json:"original_uri"`
	TimeGateURI string           `json:"timegate_uri"`
	TimeMapURI  string           `json:"timemap_uri"`
	Mementos    []JSONMementoRef `json:"mementos"`
}

// JSONMementoRef is a single snapshot item inside the JSON timemap.
type JSONMementoRef struct {
	Datetime string `json:"datetime"`
	URI      string `json:"uri"`
	Rel      string `json:"rel,omitempty"`
}

// FormatJSON serializes the TimeMap into a JSON structure.
func (tm *TimeMap) FormatJSON() ([]byte, error) {
	n := len(tm.Mementos)
	refs := make([]JSONMementoRef, 0, n)

	for i, m := range tm.Mementos {
		rel := "memento"
		if n == 1 {
			rel = "first last memento"
		} else if i == 0 {
			rel = "first memento"
		} else if i == n-1 {
			rel = "last memento"
		}

		refs = append(refs, JSONMementoRef{
			Datetime: m.Datetime.Format(RFC1123Format),
			URI:      m.URI,
			Rel:      rel,
		})
	}

	out := JSONTimeMap{
		OriginalURL: tm.OriginalURL,
		TimeGateURI: tm.TimeGateURI,
		TimeMapURI:  tm.TimeMapURI,
		Mementos:    refs,
	}

	return json.MarshalIndent(out, "", "  ")
}
