package warc

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/Noooblien/timecapsule/pkg/archive"
)

// Reader parses ISO 28500 compliant WARC records from uncompressed or GZIP compressed streams.
type Reader struct {
	br *bufio.Reader
}

// NewReader initializes a WARC Reader, auto-detecting and decompressing GZIP streams.
func NewReader(r io.Reader) (*Reader, error) {
	br := bufio.NewReader(r)
	peek, err := br.Peek(2)
	if err == nil && len(peek) >= 2 && peek[0] == 0x1f && peek[1] == 0x8b {
		gzr, err := gzip.NewReader(br)
		if err != nil {
			return nil, fmt.Errorf("failed to open gzip stream: %w", err)
		}
		return &Reader{br: bufio.NewReader(gzr)}, nil
	}
	return &Reader{br: br}, nil
}

// NextRecord advances to and returns the next WARC record in the stream, or io.EOF.
func (r *Reader) NextRecord() (*Record, error) {
	// 1. Skip leading blank lines / CRLF until WARC header magic
	var firstLine string
	for {
		lineBytes, _, err := r.br.ReadLine()
		if err != nil {
			return nil, err
		}
		line := strings.TrimSpace(string(lineBytes))
		if strings.HasPrefix(line, "WARC/") {
			firstLine = line
			break
		}
	}

	rec := &Record{
		Version:     firstLine,
		Headers:     make(map[string]string),
		HTTPHeaders: make(map[string]string),
	}

	// 2. Read WARC header lines
	var contentLength int64 = -1
	for {
		lineBytes, _, err := r.br.ReadLine()
		if err != nil {
			return nil, fmt.Errorf("error reading warc headers: %w", err)
		}
		line := string(lineBytes)
		if line == "" || line == "\r" {
			break // Empty line signifies end of WARC headers
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			rec.Headers[k] = v

			switch strings.ToLower(k) {
			case "warc-type":
				rec.Type = RecordType(strings.ToLower(v))
			case "warc-record-id":
				rec.RecordID = v
			case "warc-date":
				if parsedDate, err := time.Parse(time.RFC3339, v); err == nil {
					rec.Date = parsedDate
				}
			case "warc-target-uri":
				rec.TargetURI = v
			case "content-type":
				rec.ContentType = v
			case "content-length":
				if cl, err := strconv.ParseInt(v, 10, 64); err == nil {
					contentLength = cl
				}
			case "warc-payload-digest":
				rec.PayloadDigest = v
			case "warc-block-digest":
				rec.BlockDigest = v
			case "warc-warcinfo-id":
				rec.WarcinfoID = v
			case "warc-ip-address":
				rec.IPAddress = v
			case "warc-profile":
				rec.Profile = v
			case "warc-refers-to":
				rec.RefersToRecordID = v
			case "warc-refers-to-target-uri":
				rec.RefersToTargetURI = v
			case "warc-refers-to-date":
				if parsedDate, err := time.Parse(time.RFC3339, v); err == nil {
					rec.RefersToDate = parsedDate
				}
			}
		}
	}

	if contentLength < 0 {
		return nil, fmt.Errorf("missing Content-Length in warc record")
	}

	// 3. Read exact content block
	block := make([]byte, contentLength)
	if _, err := io.ReadFull(r.br, block); err != nil {
		return nil, fmt.Errorf("failed to read warc payload block (%d bytes): %w", contentLength, err)
	}
	rec.RawBlock = block

	// 4. Parse HTTP protocol headers if applicable
	if rec.Type == RecordTypeResponse || rec.Type == RecordTypeRequest || rec.Type == RecordTypeRevisit {
		parseHTTPPayload(rec)
	}

	return rec, nil
}

func parseHTTPPayload(rec *Record) {
	br := bufio.NewReader(bytes.NewReader(rec.RawBlock))

	// Status Line or Request Line
	lineBytes, _, err := br.ReadLine()
	if err != nil {
		return
	}
	firstLine := string(lineBytes)
	rec.HTTPStatusLine = firstLine

	if rec.Type == RecordTypeResponse || rec.Type == RecordTypeRevisit {
		parts := strings.Split(firstLine, " ")
		if len(parts) >= 2 {
			if code, err := strconv.Atoi(parts[1]); err == nil {
				rec.HTTPStatusCode = code
			}
		}
	}

	// HTTP Headers
	for {
		lineBytes, _, err := br.ReadLine()
		if err != nil {
			break
		}
		line := string(lineBytes)
		if line == "" || line == "\r" {
			break // Blank line indicates start of HTTP body
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			rec.HTTPHeaders[k] = v
		}
	}

	// Remainder is HTTP body
	body, _ := io.ReadAll(br)
	rec.HTTPPayload = body
}

// ImportToSnapshot converts a parsed WARC response record into a TimeCapsule Snapshot.
func ImportToSnapshot(rec *Record) (*archive.Snapshot, error) {
	if rec.Type != RecordTypeResponse {
		return nil, fmt.Errorf("only WARC response records can be imported as snapshots (got: %s)", rec.Type)
	}

	htmlContent := string(rec.HTTPPayload)
	contentHash := archive.ComputeHash(rec.HTTPPayload)
	title := archive.ExtractTitle(htmlContent, rec.TargetURI)
	markdown := archive.ExtractMarkdown(htmlContent, title)
	assets := archive.ExtractAssets(htmlContent)

	waybackTS := archive.FormatWaybackTS(rec.Date)
	if waybackTS == "" || rec.Date.IsZero() {
		now := time.Now().UTC()
		rec.Date = now
		waybackTS = archive.FormatWaybackTS(now)
	}

	hashPrefix := contentHash
	if len(hashPrefix) > 8 {
		hashPrefix = hashPrefix[:8]
	}
	id := fmt.Sprintf("tc_%s_%s", rec.Date.Format("20060102150405"), hashPrefix)

	contentType := rec.HTTPHeaders["Content-Type"]
	if contentType == "" {
		contentType = "text/html; charset=utf-8"
	}

	statusCode := rec.HTTPStatusCode
	if statusCode == 0 {
		statusCode = 200
	}

	return &archive.Snapshot{
		ID:          id,
		URL:         rec.TargetURI,
		Timestamp:   rec.Date,
		WaybackTS:   waybackTS,
		Title:       title,
		StatusCode:  statusCode,
		ContentType: contentType,
		Headers:     rec.HTTPHeaders,
		HTML:        htmlContent,
		Markdown:    markdown,
		ContentHash: contentHash,
		SizeBytes:   int64(len(rec.HTTPPayload)),
		AssetCount:  len(assets),
	}, nil
}
