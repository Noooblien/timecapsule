package tests

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/Noooblien/timecapsule/pkg/archive"
	"github.com/Noooblien/timecapsule/pkg/warc"
)

func TestWARC_RevisitRecordGenerationAndParsing(t *testing.T) {
	now := time.Now().UTC()
	targetURL := "https://news.ycombinator.com"
	htmlBody := "<html><body><h1>Hacker News</h1><p>Y Combinator stories</p></body></html>"
	contentHash := archive.ComputeHash([]byte(htmlBody))

	// Base Snapshot (Full Response Record)
	baseSnap := &archive.Snapshot{
		ID:          "tc_20261021120000_a1b2c3d4",
		URL:         targetURL,
		Timestamp:   now.Add(-2 * time.Hour),
		WaybackTS:   archive.FormatWaybackTS(now.Add(-2 * time.Hour)),
		Title:       "Hacker News Initial",
		StatusCode:  200,
		ContentType: "text/html; charset=utf-8",
		Headers: map[string]string{
			"Server":       "nginx",
			"Content-Type": "text/html; charset=utf-8",
		},
		HTML:        htmlBody,
		ContentHash: contentHash,
		SizeBytes:   int64(len(htmlBody)),
	}

	// Subsequent Revisit Snapshot (Unchanged Content, same Hash)
	revisitSnap := &archive.Snapshot{
		ID:          "tc_20261021140000_a1b2c3d4",
		URL:         targetURL,
		Timestamp:   now,
		WaybackTS:   archive.FormatWaybackTS(now),
		Title:       "Hacker News Unchanged",
		StatusCode:  200,
		ContentType: "text/html; charset=utf-8",
		Headers: map[string]string{
			"Server":       "nginx",
			"Content-Type": "text/html; charset=utf-8",
		},
		HTML:        htmlBody,
		ContentHash: contentHash,
		SizeBytes:   int64(len(htmlBody)),
	}

	// 1. Write Revisit WARC Stream
	var buf bytes.Buffer
	writer := warc.NewWriter(&buf, false) // uncompressed for inspection

	cdxEntries, err := writer.WriteSnapshotRevisit(revisitSnap, baseSnap, "test_archive.warc")
	if err != nil {
		t.Fatalf("failed to write snapshot revisit: %v", err)
	}

	if len(cdxEntries) != 1 {
		t.Fatalf("expected 1 CDXJ entry for revisit, got %d", len(cdxEntries))
	}
	if cdxEntries[0].MimeType != "warc/revisit" {
		t.Fatalf("expected CDXJ mime 'warc/revisit', got '%s'", cdxEntries[0].MimeType)
	}

	rawWARC := buf.String()
	// Verify WARC headers
	if !strings.Contains(rawWARC, "WARC-Type: revisit") {
		t.Fatalf("missing WARC-Type: revisit in output:\n%s", rawWARC)
	}
	if !strings.Contains(rawWARC, "WARC-Profile: http://netpreserve.org/warc/1.0/revisit/identical-payload-digest") {
		t.Fatalf("missing WARC-Profile in output:\n%s", rawWARC)
	}
	if !strings.Contains(rawWARC, "WARC-Refers-To: <urn:uuid:resp-tc_20261021120000_a1b2c3d4>") {
		t.Fatalf("missing WARC-Refers-To pointing to base record:\n%s", rawWARC)
	}
	if !strings.Contains(rawWARC, "WARC-Refers-To-Target-URI: https://news.ycombinator.com") {
		t.Fatalf("missing WARC-Refers-To-Target-URI:\n%s", rawWARC)
	}

	// 2. Read back WARC Stream using warc.Reader
	reader, err := warc.NewReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("failed to init reader: %v", err)
	}

	var parsedRecords []*warc.Record
	for {
		rec, err := reader.NextRecord()
		if err != nil {
			break
		}
		parsedRecords = append(parsedRecords, rec)
	}

	// Expected: warcinfo, request, revisit
	if len(parsedRecords) != 3 {
		t.Fatalf("expected 3 records (warcinfo, request, revisit), got %d", len(parsedRecords))
	}

	revisitRec := parsedRecords[2]
	if revisitRec.Type != warc.RecordTypeRevisit {
		t.Fatalf("expected record 3 to be revisit, got %s", revisitRec.Type)
	}
	if revisitRec.Profile != warc.ProfileIdenticalPayloadDigest {
		t.Fatalf("expected profile %s, got %s", warc.ProfileIdenticalPayloadDigest, revisitRec.Profile)
	}
	if revisitRec.RefersToRecordID != "<urn:uuid:resp-tc_20261021120000_a1b2c3d4>" {
		t.Fatalf("mismatched RefersToRecordID: %s", revisitRec.RefersToRecordID)
	}
	if revisitRec.HTTPStatusCode != 200 {
		t.Fatalf("expected HTTP 200 in revisit HTTP headers, got %d", revisitRec.HTTPStatusCode)
	}
}
