package warc

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/Noooblien/timecapsule/pkg/archive"
)

// Writer writes ISO 28500 compliant WARC records to an output stream.
type Writer struct {
	w       io.Writer
	useGzip bool
	offset  int64
}

// NewWriter creates a new WARC writer. When useGzip is true, each record is written
// as an independent GZIP member conforming to ISO 28500 Annex D.
func NewWriter(w io.Writer, useGzip bool) *Writer {
	return &Writer{
		w:       w,
		useGzip: useGzip,
	}
}

// CurrentOffset returns the cumulative byte offset written.
func (wr *Writer) CurrentOffset() int64 {
	return wr.offset
}

// WriteRecord serializes a single WARCRecord to the output stream.
// Returns the byte offset at which the record begins, and the total byte length written.
func (wr *Writer) WriteRecord(rec *Record) (int64, int64, error) {
	recordOffset := wr.offset

	// Determine payload block
	block := rec.RawBlock
	if len(block) == 0 {
		if rec.Type == RecordTypeResponse || rec.Type == RecordTypeRequest {
			block = assembleHTTPBlock(rec)
		}
	}

	// Calculate digests if missing
	if rec.PayloadDigest == "" && len(block) > 0 {
		h := sha256.Sum256(block)
		rec.PayloadDigest = "sha256:" + hex.EncodeToString(h[:])
	}
	if rec.RecordID == "" {
		rec.RecordID = fmt.Sprintf("<urn:uuid:%s-%d>", rec.Type, time.Now().UnixNano())
	}
	if rec.Version == "" {
		rec.Version = "WARC/1.0"
	}
	if rec.Date.IsZero() {
		rec.Date = time.Now().UTC()
	}

	var headerBuf bytes.Buffer
	headerBuf.WriteString(fmt.Sprintf("%s\r\n", rec.Version))
	headerBuf.WriteString(fmt.Sprintf("WARC-Type: %s\r\n", rec.Type))
	headerBuf.WriteString(fmt.Sprintf("WARC-Record-ID: %s\r\n", rec.RecordID))
	headerBuf.WriteString(fmt.Sprintf("WARC-Date: %s\r\n", rec.Date.Format(time.RFC3339)))

	if rec.TargetURI != "" {
		headerBuf.WriteString(fmt.Sprintf("WARC-Target-URI: %s\r\n", rec.TargetURI))
	}
	if rec.IPAddress != "" {
		headerBuf.WriteString(fmt.Sprintf("WARC-IP-Address: %s\r\n", rec.IPAddress))
	}
	if rec.WarcinfoID != "" {
		headerBuf.WriteString(fmt.Sprintf("WARC-Warcinfo-ID: %s\r\n", rec.WarcinfoID))
	}
	if rec.PayloadDigest != "" {
		headerBuf.WriteString(fmt.Sprintf("WARC-Payload-Digest: %s\r\n", rec.PayloadDigest))
	}
	for _, c := range rec.ConcurrentTo {
		headerBuf.WriteString(fmt.Sprintf("WARC-Concurrent-To: %s\r\n", c))
	}

	contentType := rec.ContentType
	if contentType == "" {
		if rec.Type == RecordTypeResponse {
			contentType = "application/http; msgtype=response"
		} else if rec.Type == RecordTypeRequest {
			contentType = "application/http; msgtype=request"
		} else if rec.Type == RecordTypeWarcinfo {
			contentType = "application/warc-fields"
		} else {
			contentType = "application/octet-stream"
		}
	}
	headerBuf.WriteString(fmt.Sprintf("Content-Type: %s\r\n", contentType))
	headerBuf.WriteString(fmt.Sprintf("Content-Length: %d\r\n", len(block)))
	headerBuf.WriteString("\r\n")

	uncompressed := append(headerBuf.Bytes(), block...)
	uncompressed = append(uncompressed, []byte("\r\n\r\n")...)

	var bytesWritten int64
	if wr.useGzip {
		var gzBuf bytes.Buffer
		gzWriter := gzip.NewWriter(&gzBuf)
		if _, err := gzWriter.Write(uncompressed); err != nil {
			return 0, 0, fmt.Errorf("failed to gzip warc record: %w", err)
		}
		if err := gzWriter.Close(); err != nil {
			return 0, 0, fmt.Errorf("failed to finalize gzip member: %w", err)
		}
		gzBytes := gzBuf.Bytes()
		n, err := wr.w.Write(gzBytes)
		if err != nil {
			return 0, 0, err
		}
		bytesWritten = int64(n)
	} else {
		n, err := wr.w.Write(uncompressed)
		if err != nil {
			return 0, 0, err
		}
		bytesWritten = int64(n)
	}

	wr.offset += bytesWritten
	return recordOffset, bytesWritten, nil
}

// WriteSnapshotArchive writes an entire ISO 28500 archive suite (warcinfo, request, response, metadata)
// for a given TimeCapsule Snapshot and returns the corresponding CDXJ index entries.
func (wr *Writer) WriteSnapshotArchive(snap *archive.Snapshot, filename string) ([]CDXJEntry, error) {
	var cdxEntries []CDXJEntry

	warcinfoID := fmt.Sprintf("<urn:uuid:info-%s>", snap.ID)
	responseID := fmt.Sprintf("<urn:uuid:resp-%s>", snap.ID)
	requestID := fmt.Sprintf("<urn:uuid:req-%s>", snap.ID)

	// 1. Warcinfo Record
	warcinfoBlock := fmt.Sprintf("software: TimeCapsule/1.2.0 (+https://github.com/Noooblien/timecapsule)\r\n"+
		"format: WARC File Format 1.0\r\n"+
		"conformsTo: http://bibnum.bnf.fr/WARC/WARC_ISO_28500_version1_latestdraft.pdf\r\n"+
		"robots: ignore\r\n"+
		"http-header-user-agent: Mozilla/5.0 (compatible; TimeCapsule/1.2.0; +https://github.com/Noooblien/timecapsule)\r\n")

	infoRec := &Record{
		Type:        RecordTypeWarcinfo,
		RecordID:    warcinfoID,
		Date:        snap.Timestamp,
		ContentType: "application/warc-fields",
		RawBlock:    []byte(warcinfoBlock),
	}
	if _, _, err := wr.WriteRecord(infoRec); err != nil {
		return nil, fmt.Errorf("failed to write warcinfo record: %w", err)
	}

	// 2. Request Record
	reqRec := &Record{
		Type:         RecordTypeRequest,
		RecordID:     requestID,
		Date:         snap.Timestamp,
		TargetURI:    snap.URL,
		WarcinfoID:   warcinfoID,
		ConcurrentTo: []string{responseID},
		RawBlock:     []byte(fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s\r\nUser-Agent: TimeCapsule/1.2.0\r\nAccept: text/html,application/xhtml+xml\r\n\r\n", extractHost(snap.URL))),
	}
	if _, _, err := wr.WriteRecord(reqRec); err != nil {
		return nil, fmt.Errorf("failed to write request record: %w", err)
	}

	// 3. Response Record
	respRec := &Record{
		Type:           RecordTypeResponse,
		RecordID:       responseID,
		Date:           snap.Timestamp,
		TargetURI:      snap.URL,
		WarcinfoID:     warcinfoID,
		ConcurrentTo:   []string{requestID},
		HTTPStatusCode: snap.StatusCode,
		HTTPStatusLine: fmt.Sprintf("HTTP/1.1 %d OK", snap.StatusCode),
		HTTPHeaders:    snap.Headers,
		HTTPPayload:    []byte(snap.HTML),
		PayloadDigest:  "sha256:" + snap.ContentHash,
	}

	offset, length, err := wr.WriteRecord(respRec)
	if err != nil {
		return nil, fmt.Errorf("failed to write response record: %w", err)
	}

	// CDXJ Index Entry for Response Record
	surt := ToSURT(snap.URL)
	cdxEntries = append(cdxEntries, CDXJEntry{
		SURT:        surt,
		Timestamp:   snap.WaybackTS,
		OriginalURL: snap.URL,
		MimeType:    snap.ContentType,
		StatusCode:  snap.StatusCode,
		Digest:      snap.ContentHash,
		Length:      length,
		Offset:      offset,
		Filename:    filename,
	})

	// 4. Metadata Record
	metaBlock := fmt.Sprintf("title: %s\r\nassetCount: %d\r\nmarkdownLength: %d\r\ncontentHash: %s\r\n",
		snap.Title, snap.AssetCount, len(snap.Markdown), snap.ContentHash)
	metaRec := &Record{
		Type:        RecordTypeMetadata,
		RecordID:    fmt.Sprintf("<urn:uuid:meta-%s>", snap.ID),
		Date:        snap.Timestamp,
		TargetURI:   snap.URL,
		WarcinfoID:  warcinfoID,
		ContentType: "application/warc-fields",
		RawBlock:    []byte(metaBlock),
	}
	if _, _, err := wr.WriteRecord(metaRec); err != nil {
		return nil, fmt.Errorf("failed to write metadata record: %w", err)
	}

	return cdxEntries, nil
}

func assembleHTTPBlock(rec *Record) []byte {
	var buf bytes.Buffer
	if rec.HTTPStatusLine != "" {
		buf.WriteString(rec.HTTPStatusLine + "\r\n")
	} else if rec.HTTPStatusCode > 0 {
		buf.WriteString(fmt.Sprintf("HTTP/1.1 %d OK\r\n", rec.HTTPStatusCode))
	} else {
		buf.WriteString("HTTP/1.1 200 OK\r\n")
	}

	// Write HTTP headers sorted for deterministic output
	var keys []string
	for k := range rec.HTTPHeaders {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	hasContentType := false
	hasContentLength := false
	for _, k := range keys {
		v := rec.HTTPHeaders[k]
		if strings.EqualFold(k, "Content-Type") {
			hasContentType = true
		}
		if strings.EqualFold(k, "Content-Length") {
			hasContentLength = true
		}
		buf.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}

	if !hasContentType && rec.ContentType != "" {
		buf.WriteString(fmt.Sprintf("Content-Type: %s\r\n", rec.ContentType))
	}
	if !hasContentLength {
		buf.WriteString(fmt.Sprintf("Content-Length: %d\r\n", len(rec.HTTPPayload)))
	}
	buf.WriteString("\r\n")
	buf.Write(rec.HTTPPayload)

	return buf.Bytes()
}

func extractHost(rawURL string) string {
	parts := strings.Split(rawURL, "://")
	if len(parts) > 1 {
		rawURL = parts[1]
	}
	slashIdx := strings.Index(rawURL, "/")
	if slashIdx != -1 {
		return rawURL[:slashIdx]
	}
	return rawURL
}
