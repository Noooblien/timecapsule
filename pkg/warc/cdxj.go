package warc

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ToSURT converts a standard web URL into Sort-friendly URI Technique (SURT) canonical form.
// Example: https://news.ycombinator.com/item?id=1 -> org,ycombinator,news)/item?id=1
func ToSURT(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	host := strings.ToLower(u.Host)
	port := ""
	if strings.Contains(host, ":") {
		h, p, err := net.SplitHostPort(host)
		if err == nil {
			host = h
			port = ":" + p
		}
	}

	var surtHost string
	// Check if IP address
	if ip := net.ParseIP(host); ip != nil {
		surtHost = host + port + ")"
	} else {
		// Reverse domain segments
		parts := strings.Split(host, ".")
		var reversed []string
		for i := len(parts) - 1; i >= 0; i-- {
			if parts[i] != "" {
				reversed = append(reversed, parts[i])
			}
		}
		surtHost = strings.Join(reversed, ",") + port + ")"
	}

	path := u.Path
	if path == "" {
		path = "/"
	}
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}

	return surtHost + path
}

// FormatCDXJ formats a CDXJEntry into a standard single-line CDXJ 1.1 string.
func FormatCDXJ(entry CDXJEntry) string {
	payload := map[string]interface{}{
		"url":    entry.OriginalURL,
		"mime":   entry.MimeType,
		"status": entry.StatusCode,
		"digest": entry.Digest,
		"length": entry.Length,
		"offset": entry.Offset,
	}
	if entry.Filename != "" {
		payload["filename"] = entry.Filename
	}

	jsonBytes, _ := json.Marshal(payload)
	return fmt.Sprintf("%s %s %s\n", entry.SURT, entry.Timestamp, string(jsonBytes))
}

// ParseCDXJ parses a single CDXJ 1.1 line into a CDXJEntry.
func ParseCDXJ(line string) (*CDXJEntry, error) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
		return nil, fmt.Errorf("empty or comment line")
	}

	parts := strings.SplitN(line, " ", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid CDXJ format: expected at least 3 space-separated fields")
	}

	surt := parts[0]
	timestamp := parts[1]
	jsonStr := parts[2]

	var meta struct {
		URL      string      `json:"url"`
		Mime     string      `json:"mime"`
		Status   interface{} `json:"status"`
		Digest   string      `json:"digest"`
		Length   int64       `json:"length"`
		Offset   int64       `json:"offset"`
		Filename string      `json:"filename"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &meta); err != nil {
		return nil, fmt.Errorf("failed to parse CDXJ JSON metadata: %w", err)
	}

	statusCode := 200
	switch v := meta.Status.(type) {
	case float64:
		statusCode = int(v)
	case int:
		statusCode = v
	case string:
		fmt.Sscanf(v, "%d", &statusCode)
	}

	return &CDXJEntry{
		SURT:        surt,
		Timestamp:   timestamp,
		OriginalURL: meta.URL,
		MimeType:    meta.Mime,
		StatusCode:  statusCode,
		Digest:      meta.Digest,
		Length:      meta.Length,
		Offset:      meta.Offset,
		Filename:    meta.Filename,
	}, nil
}
