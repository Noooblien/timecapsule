package warc

import (
	"time"
)

// RecordType defines standard ISO 28500 WARC record types.
type RecordType string

const (
	RecordTypeWarcinfo RecordType = "warcinfo"
	RecordTypeRequest  RecordType = "request"
	RecordTypeResponse RecordType = "response"
	RecordTypeMetadata RecordType = "metadata"
	RecordTypeRevisit  RecordType = "revisit"
	RecordTypeResource RecordType = "resource"
)

const (
	ProfileIdenticalPayloadDigest = "http://netpreserve.org/warc/1.0/revisit/identical-payload-digest"
	ProfileServerNotModified      = "http://netpreserve.org/warc/1.0/revisit/server-not-modified"
)

// Record represents an in-memory ISO 28500 WARC record.
type Record struct {
	Version        string            `json:"version"`          // e.g. "WARC/1.0"
	Type           RecordType        `json:"type"`             // warcinfo, request, response, metadata
	RecordID       string            `json:"record_id"`        // <urn:uuid:...>
	Date           time.Time         `json:"date"`             // ISO 8601 UTC
	TargetURI      string            `json:"target_uri"`       // Canonical URL
	ContentType    string            `json:"content_type"`     // application/http, text/plain, etc.
	IPAddress      string            `json:"ip_address,omitempty"`
	WarcinfoID     string            `json:"warcinfo_id,omitempty"`
	PayloadDigest  string            `json:"payload_digest,omitempty"` // sha256:...
	BlockDigest    string            `json:"block_digest,omitempty"`
	ConcurrentTo        []string          `json:"concurrent_to,omitempty"`
	Profile             string            `json:"profile,omitempty"`              // e.g. "http://netpreserve.org/warc/1.0/revisit/identical-payload-digest"
	RefersToTargetURI   string            `json:"refers_to_target_uri,omitempty"` // For revisit records
	RefersToDate        time.Time         `json:"refers_to_date,omitempty"`       // For revisit records
	RefersToRecordID    string            `json:"refers_to_record_id,omitempty"`  // <urn:uuid:...> of base record
	Headers             map[string]string `json:"headers"`          // All raw WARC headers
	RawBlock       []byte            `json:"-"`                // Unparsed block bytes

	// Protocol-level HTTP payload fields (populated for request/response records)
	HTTPStatusCode int               `json:"http_status_code,omitempty"`
	HTTPStatusLine string            `json:"http_status_line,omitempty"`
	HTTPHeaders    map[string]string `json:"http_headers,omitempty"`
	HTTPPayload    []byte            `json:"-"`
}

// CDXJEntry represents a standard Wayback Capture Index (CDXJ 1.1) record.
type CDXJEntry struct {
	SURT        string `json:"surt"`                   // Sort-friendly URI Technique key
	Timestamp   string `json:"timestamp"`              // YYYYMMDDhhmmss
	OriginalURL string `json:"url"`                    // Original target URL
	MimeType    string `json:"mime"`                   // Response MIME type
	StatusCode  int    `json:"status"`                 // HTTP status code
	Digest      string `json:"digest"`                 // Content SHA-256 or SHA-1 digest
	Length      int64  `json:"length"`                 // Byte length of record in WARC
	Offset      int64  `json:"offset"`                 // Byte offset of record in WARC
	Filename    string `json:"filename,omitempty"`     // WARC archive filename
}
