package tests_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Noooblien/timecapsule/pkg/archive"
	"github.com/Noooblien/timecapsule/pkg/server"
	"github.com/Noooblien/timecapsule/pkg/storage"
	"github.com/Noooblien/timecapsule/pkg/warc"
)

func TestWARC_MultiRecordSuiteAndGzip(t *testing.T) {
	now := time.Now().UTC()
	snap := &archive.Snapshot{
		ID:          "tc_20261018000400_a3f19e8c",
		URL:         "https://news.ycombinator.com",
		Timestamp:   now,
		WaybackTS:   "20261018000400",
		Title:       "Hacker News",
		StatusCode:  200,
		ContentType: "text/html; charset=utf-8",
		Headers: map[string]string{
			"Server":       "nginx",
			"Content-Type": "text/html; charset=utf-8",
		},
		HTML:        "<html><head><title>Hacker News</title></head><body><h1>Hacker News</h1><p>Top stories today</p></body></html>",
		Markdown:    "# Hacker News\n\nTop stories today",
		ContentHash: "a3f19e8c9284f291048bcefae091823746198273645192837465192837465192",
		SizeBytes:   112,
		AssetCount:  0,
	}

	// 1. Test Plain Uncompressed WARC
	var plainBuf bytes.Buffer
	writer := warc.NewWriter(&plainBuf, false)
	cdxEntries, err := writer.WriteSnapshotArchive(snap, "archive.warc")
	if err != nil {
		t.Fatalf("failed to write snapshot archive: %v", err)
	}

	if len(cdxEntries) != 1 {
		t.Fatalf("expected 1 CDX entry for response record, got %d", len(cdxEntries))
	}
	if !strings.HasPrefix(cdxEntries[0].SURT, "com,ycombinator,news)/") {
		t.Errorf("unexpected SURT in CDX entry: %s", cdxEntries[0].SURT)
	}

	reader, err := warc.NewReader(&plainBuf)
	if err != nil {
		t.Fatalf("failed to create warc reader: %v", err)
	}

	// Record 0: Warcinfo
	rec0, err := reader.NextRecord()
	if err != nil {
		t.Fatalf("expected warcinfo record, got error: %v", err)
	}
	if rec0.Type != warc.RecordTypeWarcinfo {
		t.Errorf("expected type warcinfo, got %s", rec0.Type)
	}
	if !strings.Contains(string(rec0.RawBlock), "TimeCapsule/1.2.0") {
		t.Errorf("expected TimeCapsule software identifier in warcinfo block")
	}

	// Record 1: Request
	rec1, err := reader.NextRecord()
	if err != nil {
		t.Fatalf("expected request record, got error: %v", err)
	}
	if rec1.Type != warc.RecordTypeRequest {
		t.Errorf("expected type request, got %s", rec1.Type)
	}
	if rec1.TargetURI != snap.URL {
		t.Errorf("expected target URI %s, got %s", snap.URL, rec1.TargetURI)
	}

	// Record 2: Response
	rec2, err := reader.NextRecord()
	if err != nil {
		t.Fatalf("expected response record, got error: %v", err)
	}
	if rec2.Type != warc.RecordTypeResponse {
		t.Errorf("expected type response, got %s", rec2.Type)
	}
	if rec2.HTTPStatusCode != 200 {
		t.Errorf("expected status code 200, got %d", rec2.HTTPStatusCode)
	}
	if string(rec2.HTTPPayload) != snap.HTML {
		t.Errorf("HTML payload mismatch in response record")
	}

	// Record 3: Metadata
	rec3, err := reader.NextRecord()
	if err != nil {
		t.Fatalf("expected metadata record, got error: %v", err)
	}
	if rec3.Type != warc.RecordTypeMetadata {
		t.Errorf("expected type metadata, got %s", rec3.Type)
	}

	// 2. Test GZIP Compressed WARC (ISO 28500 Annex D)
	var gzBuf bytes.Buffer
	gzWriter := warc.NewWriter(&gzBuf, true)
	_, err = gzWriter.WriteSnapshotArchive(snap, "archive.warc.gz")
	if err != nil {
		t.Fatalf("failed to write gzipped snapshot archive: %v", err)
	}

	gzReader, err := warc.NewReader(&gzBuf)
	if err != nil {
		t.Fatalf("failed to create reader for gzipped warc: %v", err)
	}

	// Read first record from gzipped stream
	gzRec0, err := gzReader.NextRecord()
	if err != nil {
		t.Fatalf("failed to read from gzipped warc: %v", err)
	}
	if gzRec0.Type != warc.RecordTypeWarcinfo {
		t.Errorf("expected warcinfo from gzipped warc, got %s", gzRec0.Type)
	}
}

func TestWARC_ImportToSnapshot(t *testing.T) {
	now := time.Now().UTC()
	html := `<!DOCTYPE html><html><head><title>Test Blog Post</title></head><body><h1>Hello World</h1><p>Archived content paragraph.</p></body></html>`

	rec := &warc.Record{
		Type:           warc.RecordTypeResponse,
		Date:           now,
		TargetURI:      "https://blog.example.org/posts/hello",
		HTTPStatusCode: 200,
		HTTPHeaders: map[string]string{
			"Content-Type": "text/html; charset=utf-8",
		},
		HTTPPayload: []byte(html),
	}

	snap, err := warc.ImportToSnapshot(rec)
	if err != nil {
		t.Fatalf("failed to import warc record to snapshot: %v", err)
	}

	if snap.Title != "Test Blog Post" {
		t.Errorf("expected title 'Test Blog Post', got '%s'", snap.Title)
	}
	if snap.URL != "https://blog.example.org/posts/hello" {
		t.Errorf("expected URL 'https://blog.example.org/posts/hello', got '%s'", snap.URL)
	}
	if !strings.Contains(snap.Markdown, "# Hello World") {
		t.Errorf("expected markdown conversion '# Hello World', got '%s'", snap.Markdown)
	}
	if snap.StatusCode != 200 {
		t.Errorf("expected status code 200, got %d", snap.StatusCode)
	}
}

func TestWARC_SURTAndCDXJ(t *testing.T) {
	// 1. Test SURT Conversions
	tests := []struct {
		url      string
		expected string
	}{
		{"https://news.ycombinator.com/item?id=123", "com,ycombinator,news)/item?id=123"},
		{"http://example.org:8080/path/to/page", "org,example:8080)/path/to/page"},
		{"https://sub.domain.co.uk/", "uk,co,domain,sub)/"},
		{"http://127.0.0.1:3000/api", "127.0.0.1:3000)/api"},
	}

	for _, tt := range tests {
		surt := warc.ToSURT(tt.url)
		if surt != tt.expected {
			t.Errorf("ToSURT(%q) = %q, expected %q", tt.url, surt, tt.expected)
		}
	}

	// 2. Test CDXJ Formatting & Parsing
	orig := warc.CDXJEntry{
		SURT:        "com,ycombinator,news)/item?id=123",
		Timestamp:   "20261018000400",
		OriginalURL: "https://news.ycombinator.com/item?id=123",
		MimeType:    "text/html",
		StatusCode:  200,
		Digest:      "a3f19e8c9284f",
		Length:      18452,
		Offset:      1024,
		Filename:    "tc_archive.warc.gz",
	}

	line := warc.FormatCDXJ(orig)
	parsed, err := warc.ParseCDXJ(line)
	if err != nil {
		t.Fatalf("failed to parse formatted CDXJ line: %v", err)
	}

	if parsed.SURT != orig.SURT || parsed.Timestamp != orig.Timestamp || parsed.StatusCode != 200 || parsed.Length != 18452 {
		t.Errorf("parsed CDXJ mismatch: %+v vs %+v", parsed, orig)
	}
}

func TestServer_CDXEndpoint(t *testing.T) {
	tempDir := t.TempDir()
	store, err := storage.NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create temp store: %v", err)
	}

	snap := &archive.Snapshot{
		ID:          "tc_20261018000400_abc12345",
		URL:         "https://example.com/test",
		Timestamp:   time.Now().UTC(),
		WaybackTS:   "20261018000400",
		Title:       "Example Domain",
		StatusCode:  200,
		ContentType: "text/html; charset=utf-8",
		HTML:        "<html><body>Example</body></html>",
		Markdown:    "Example",
		ContentHash: "abc1234567890",
		SizeBytes:   32,
	}
	if err := store.Save(snap); err != nil {
		t.Fatalf("failed to save test snapshot: %v", err)
	}

	srv := server.NewServer(store, nil)

	// 1. Test CDX JSON Endpoint
	req := httptest.NewRequest("GET", "/cdx?url=https://example.com/test&output=json", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /cdx, got %d", w.Code)
	}

	var entries []warc.CDXJEntry
	if err := json.Unmarshal(w.Body.Bytes(), &entries); err != nil {
		t.Fatalf("failed to decode CDX JSON response: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 CDX entry, got %d", len(entries))
	}
	if entries[0].OriginalURL != snap.URL {
		t.Errorf("expected URL %s in CDX entry, got %s", snap.URL, entries[0].OriginalURL)
	}

	// 2. Test CDXJ Text Endpoint
	req2 := httptest.NewRequest("GET", "/cdx?output=cdxj", nil)
	w2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /cdx?output=cdxj, got %d", w2.Code)
	}
	if !strings.Contains(w2.Body.String(), "com,example)/test") {
		t.Errorf("expected CDXJ line with SURT in output: %s", w2.Body.String())
	}
}
