package archive

import (
	"bytes"
	"fmt"
	"time"
)

// GenerateWARC formats a Snapshot as an ISO 28500 WARC response record.
func GenerateWARC(snap *Snapshot) []byte {
	var buf bytes.Buffer

	// WARC Record Header
	warcID := fmt.Sprintf("<urn:uuid:%s>", snap.ID)
	warcDate := snap.Timestamp.Format(time.RFC3339)

	httpPayload := fmt.Sprintf("HTTP/1.1 %d OK\r\nContent-Type: %s\r\nContent-Length: %d\r\n\r\n%s",
		snap.StatusCode, snap.ContentType, len(snap.HTML), snap.HTML)

	recordHeader := fmt.Sprintf("WARC/1.0\r\n"+
		"WARC-Type: response\r\n"+
		"WARC-Record-ID: %s\r\n"+
		"WARC-Date: %s\r\n"+
		"WARC-Target-URI: %s\r\n"+
		"Content-Type: application/http; msgtype=response\r\n"+
		"Content-Length: %d\r\n\r\n",
		warcID, warcDate, snap.URL, len(httpPayload))

	buf.WriteString(recordHeader)
	buf.WriteString(httpPayload)
	buf.WriteString("\r\n\r\n")

	return buf.Bytes()
}
