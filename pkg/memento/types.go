package memento

import (
	"time"
)

// RFC1123Format is the standard HTTP datetime format mandated by RFC 7089 (Section 2.1).
const RFC1123Format = time.RFC1123

// LinkFormatMimeType is the media type for RFC 7089 Section 5 link-format serialization.
const LinkFormatMimeType = "application/link-format"

// Memento represents an immutable historical web capture (URI-M).
type Memento struct {
	URI         string    `json:"uri"`
	Datetime    time.Time `json:"datetime"`
	OriginalURL string    `json:"original_url"`
	MimeType    string    `json:"mime_type,omitempty"`
	StatusCode  int       `json:"status_code,omitempty"`
	Digest      string    `json:"digest,omitempty"`
}

// TimeGateConfig contains routing endpoints and defaults for RFC 7089 resolution.
type TimeGateConfig struct {
	BaseURL     string // e.g. "https://timecapsule.local" or "http://localhost:8080"
	DefaultRel  string
	EnableJSON  bool
}

// TimeMap contains the complete chronological sequence of mementos for a given original resource (URI-R).
type TimeMap struct {
	OriginalURL string     `json:"original_url"`
	TimeGateURI string     `json:"timegate_uri"`
	TimeMapURI  string     `json:"timemap_uri"`
	Mementos    []*Memento `json:"mementos"`
}

// TemporalResolution is the result of resolving an Accept-Datetime request against a TimeGate.
type TemporalResolution struct {
	RequestedTime time.Time `json:"requested_time"`
	Selected      *Memento  `json:"selected"`
	First         *Memento  `json:"first"`
	Prev          *Memento  `json:"prev,omitempty"`
	Next          *Memento  `json:"next,omitempty"`
	Last          *Memento  `json:"last"`
	DeltaSeconds  float64   `json:"delta_seconds"`
}
