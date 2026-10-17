package archive

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var (
	titleRegex   = regexp.MustCompile(`(?i)<title[^>]*>(.*?)</title>`)
	scriptRegex  = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	styleRegex   = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	tagRegex     = regexp.MustCompile(`<[^>]+>`)
	wsRegex      = regexp.MustCompile(`\s+`)
	h1Regex      = regexp.MustCompile(`(?i)<h1[^>]*>(.*?)</h1>`)
	h2Regex      = regexp.MustCompile(`(?i)<h2[^>]*>(.*?)</h2>`)
	pRegex       = regexp.MustCompile(`(?i)<p[^>]*>(.*?)</p>`)
	assetLinkReg = regexp.MustCompile(`(?i)(?:href|src)=["']([^"']+\.(?:css|js|png|jpg|jpeg|gif|svg|webp))["']`)
)

// Archiver handles HTTP retrieval and parsing of web pages.
type Archiver struct {
	client *http.Client
}

// NewArchiver creates an Archiver with a reasonable timeout and modern User-Agent.
func NewArchiver(timeout time.Duration) *Archiver {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &Archiver{
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Capture fetches target URL and produces a full Snapshot.
func (a *Archiver) Capture(ctx context.Context, targetURL string) (*Snapshot, error) {
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = "https://" + targetURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; TimeCapsule/1.0; +https://github.com/Noooblien/timecapsule)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch failed for %s: %w", targetURL, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	rawHTML := string(bodyBytes)
	now := time.Now().UTC()
	hash := ComputeHash(bodyBytes)
	title := ExtractTitle(rawHTML, targetURL)
	markdown := ExtractMarkdown(rawHTML, title)
	assets := ExtractAssets(rawHTML)

	headersMap := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			headersMap[k] = v[0]
		}
	}

	snap := &Snapshot{
		ID:          fmt.Sprintf("tc_%s_%s", FormatWaybackTS(now), hash[:8]),
		URL:         targetURL,
		Timestamp:   now,
		WaybackTS:   FormatWaybackTS(now),
		Title:       title,
		StatusCode:  resp.StatusCode,
		ContentType: resp.Header.Get("Content-Type"),
		Headers:     headersMap,
		HTML:        rawHTML,
		Markdown:    markdown,
		ContentHash: hash,
		SizeBytes:   int64(len(bodyBytes)),
		AssetCount:  len(assets),
	}

	return snap, nil
}

// ExtractTitle pulls <title> from HTML or falls back to URL.
func ExtractTitle(html, fallback string) string {
	matches := titleRegex.FindStringSubmatch(html)
	if len(matches) > 1 {
		clean := strings.TrimSpace(matches[1])
		clean = strings.ReplaceAll(clean, "\n", " ")
		if clean != "" {
			return clean
		}
	}
	return fallback
}

// ExtractMarkdown converts core textual elements (headings, paragraphs) to clean markdown.
func ExtractMarkdown(html, title string) string {
	cleaned := scriptRegex.ReplaceAllString(html, "")
	cleaned = styleRegex.ReplaceAllString(cleaned, "")

	var sb strings.Builder
	sb.WriteString("# " + title + "\n\n")

	// Extract H1s
	h1Matches := h1Regex.FindAllStringSubmatch(cleaned, -1)
	for _, m := range h1Matches {
		if len(m) > 1 {
			txt := strings.TrimSpace(tagRegex.ReplaceAllString(m[1], ""))
			if txt != "" && txt != title {
				sb.WriteString("## " + txt + "\n\n")
			}
		}
	}

	// Extract H2s
	h2Matches := h2Regex.FindAllStringSubmatch(cleaned, -1)
	for _, m := range h2Matches {
		if len(m) > 1 {
			txt := strings.TrimSpace(tagRegex.ReplaceAllString(m[1], ""))
			if txt != "" {
				sb.WriteString("### " + txt + "\n\n")
			}
		}
	}

	// Extract paragraphs
	pMatches := pRegex.FindAllStringSubmatch(cleaned, -1)
	for _, m := range pMatches {
		if len(m) > 1 {
			txt := strings.TrimSpace(tagRegex.ReplaceAllString(m[1], ""))
			txt = wsRegex.ReplaceAllString(txt, " ")
			if len(txt) > 20 {
				sb.WriteString(txt + "\n\n")
			}
		}
	}

	result := sb.String()
	if strings.TrimSpace(result) == "# "+title {
		// Fallback: strip all tags
		raw := tagRegex.ReplaceAllString(cleaned, " ")
		raw = wsRegex.ReplaceAllString(raw, " ")
		return "# " + title + "\n\n" + strings.TrimSpace(raw)
	}

	return result
}

// ExtractAssets finds referenced image, css, and js asset paths.
func ExtractAssets(html string) []string {
	matches := assetLinkReg.FindAllStringSubmatch(html, -1)
	seen := make(map[string]bool)
	var list []string
	for _, m := range matches {
		if len(m) > 1 {
			asset := strings.TrimSpace(m[1])
			if !seen[asset] && asset != "" {
				seen[asset] = true
				list = append(list, asset)
			}
		}
	}
	return list
}
