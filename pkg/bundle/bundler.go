package bundle

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
)

var (
	// Regex matchers for HTML DOM elements
	linkCSSRegex = regexp.MustCompile(`(?i)<link\s+[^>]*?(?:rel=["']?stylesheet["']?[^>]*?href=["']([^"']+)["']|href=["']([^"']+)["'][^>]*?rel=["']?stylesheet["']?)[^>]*?>`)
	imgSrcRegex  = regexp.MustCompile(`(?i)<img\s+([^>]*?)src=["']([^"']+)["']([^>]*?)>`)
	cssURLRegex  = regexp.MustCompile(`(?i)url\(["']?([^"')]+)["']?\)`)
	scriptRegex  = regexp.MustCompile(`(?is)<script\b([^>]*)>(.*?)</script>`)
	titleRegex   = regexp.MustCompile(`(?i)<title[^>]*>(.*?)</title>`)
)

// Bundler coordinates the retrieval and inlining of assets into a single-file DOM.
type Bundler struct {
	cfg    BundleConfig
	client *http.Client
}

// NewBundler creates an initialized single-file DOM serializer.
func NewBundler(cfg BundleConfig) *Bundler {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}
	if cfg.MaxAssetBytes <= 0 {
		cfg.MaxAssetBytes = 5 * 1024 * 1024
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = "TimeCapsule-Bundler/1.2"
	}

	return &Bundler{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// ResolveURL computes an absolute target URL from a base URL and relative reference.
func ResolveURL(baseURI, refURI string) string {
	refURI = strings.TrimSpace(refURI)
	if refURI == "" || strings.HasPrefix(refURI, "data:") || strings.HasPrefix(refURI, "javascript:") || strings.HasPrefix(refURI, "#") {
		return refURI
	}

	base, err := url.Parse(baseURI)
	if err != nil {
		return refURI
	}

	// Handle protocol-relative URLs (e.g. "//cdn.example.com/lib.css")
	if strings.HasPrefix(refURI, "//") {
		if base.Scheme != "" {
			return base.Scheme + ":" + refURI
		}
		return "https:" + refURI
	}

	ref, err := url.Parse(refURI)
	if err != nil {
		return refURI
	}

	return base.ResolveReference(ref).String()
}

// DetectMimeType infers MIME type based on file extension, header, and content sniffing.
func DetectMimeType(urlStr string, headerMime string, data []byte) string {
	if headerMime != "" && !strings.Contains(headerMime, "text/plain") && !strings.Contains(headerMime, "octet-stream") {
		parts := strings.Split(headerMime, ";")
		return strings.TrimSpace(parts[0])
	}

	u, err := url.Parse(urlStr)
	ext := ""
	if err == nil {
		ext = strings.ToLower(path.Ext(u.Path))
	}

	switch ext {
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	case ".ico":
		return "image/x-icon"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".otf":
		return "font/otf"
	}

	if len(data) > 0 {
		return http.DetectContentType(data)
	}

	return "application/octet-stream"
}

func (b *Bundler) fetchAsset(ctx context.Context, targetURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create asset request: %w", err)
	}

	req.Header.Set("User-Agent", b.cfg.UserAgent)
	req.Header.Set("Accept", "*/*")

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("asset fetch error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("HTTP error %d", resp.StatusCode)
	}

	limitReader := io.LimitReader(resp.Body, b.cfg.MaxAssetBytes+1)
	data, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read asset body: %w", err)
	}

	if int64(len(data)) > b.cfg.MaxAssetBytes {
		return nil, "", fmt.Errorf("asset exceeded max size cap (%d bytes)", b.cfg.MaxAssetBytes)
	}

	mimeType := DetectMimeType(targetURL, resp.Header.Get("Content-Type"), data)
	return data, mimeType, nil
}

// BundleHTML serializes raw HTML into a self-contained, hermetic single-file document.
func (b *Bundler) BundleHTML(ctx context.Context, targetURL, rawHTML string) (*BundleResult, error) {
	start := time.Now()
	var inlinedAssets []InlinedAsset
	inlinedCount := 0
	skippedCount := 0

	title := ""
	if m := titleRegex.FindStringSubmatch(rawHTML); len(m) > 1 {
		title = strings.TrimSpace(m[1])
	}
	if title == "" {
		title = targetURL
	}

	processedHTML := rawHTML

	// 1. Inline Stylesheets (<link rel="stylesheet"> -> <style>)
	if b.cfg.InlineStyles {
		processedHTML = linkCSSRegex.ReplaceAllStringFunc(processedHTML, func(match string) string {
			m := linkCSSRegex.FindStringSubmatch(match)
			href := m[1]
			if href == "" {
				href = m[2]
			}
			href = strings.TrimSpace(href)
			if href == "" || strings.HasPrefix(href, "data:") {
				return match
			}

			resolved := ResolveURL(targetURL, href)
			cssBytes, _, err := b.fetchAsset(ctx, resolved)
			if err != nil {
				inlinedAssets = append(inlinedAssets, InlinedAsset{
					OriginalURL: href,
					ResolvedURL: resolved,
					AssetType:   "stylesheet",
					Status:      "failed",
					ErrorMsg:    err.Error(),
				})
				skippedCount++
				return match // Preserve original link on failure
			}

			cssContent := string(cssBytes)

			// Inline internal url(...) references inside CSS
			cssContent = cssURLRegex.ReplaceAllStringFunc(cssContent, func(urlMatch string) string {
				sub := cssURLRegex.FindStringSubmatch(urlMatch)
				if len(sub) < 2 {
					return urlMatch
				}
				cssRef := strings.Trim(strings.TrimSpace(sub[1]), `"'`)
				if strings.HasPrefix(cssRef, "data:") || cssRef == "" {
					return urlMatch
				}
				resolvedSub := ResolveURL(resolved, cssRef)
				subBytes, subMime, subErr := b.fetchAsset(ctx, resolvedSub)
				if subErr != nil {
					return urlMatch
				}
				b64 := base64.StdEncoding.EncodeToString(subBytes)
				inlinedAssets = append(inlinedAssets, InlinedAsset{
					OriginalURL: cssRef,
					ResolvedURL: resolvedSub,
					AssetType:   "nested-css-asset",
					MimeType:    subMime,
					SizeBytes:   int64(len(subBytes)),
					Status:      "inlined",
				})
				inlinedCount++
				return fmt.Sprintf("url('data:%s;base64,%s')", subMime, b64)
			})

			inlinedAssets = append(inlinedAssets, InlinedAsset{
				OriginalURL: href,
				ResolvedURL: resolved,
				AssetType:   "stylesheet",
				MimeType:    "text/css",
				SizeBytes:   int64(len(cssBytes)),
				Status:      "inlined",
			})
			inlinedCount++

			return fmt.Sprintf("<style data-timecapsule-inlined=\"%s\">\n%s\n</style>", resolved, cssContent)
		})
	}

	// 2. Inline Images (<img src="..."> -> <img src="data:...">)
	if b.cfg.InlineImages {
		processedHTML = imgSrcRegex.ReplaceAllStringFunc(processedHTML, func(match string) string {
			m := imgSrcRegex.FindStringSubmatch(match)
			before := m[1]
			src := strings.TrimSpace(m[2])
			after := m[3]

			if src == "" || strings.HasPrefix(src, "data:") {
				return match
			}

			resolved := ResolveURL(targetURL, src)
			imgBytes, mimeType, err := b.fetchAsset(ctx, resolved)
			if err != nil {
				inlinedAssets = append(inlinedAssets, InlinedAsset{
					OriginalURL: src,
					ResolvedURL: resolved,
					AssetType:   "image",
					Status:      "failed",
					ErrorMsg:    err.Error(),
				})
				skippedCount++
				return match
			}

			b64 := base64.StdEncoding.EncodeToString(imgBytes)
			dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, b64)

			inlinedAssets = append(inlinedAssets, InlinedAsset{
				OriginalURL: src,
				ResolvedURL: resolved,
				AssetType:   "image",
				MimeType:    mimeType,
				SizeBytes:   int64(len(imgBytes)),
				Status:      "inlined",
			})
			inlinedCount++

			return fmt.Sprintf("<img %ssrc=\"%s\" data-original-src=\"%s\"%s>", before, dataURI, resolved, after)
		})
	}

	// 3. Script Neutralization (safe offline reader mode)
	if b.cfg.NeutralizeScripts {
		processedHTML = scriptRegex.ReplaceAllStringFunc(processedHTML, func(match string) string {
			m := scriptRegex.FindStringSubmatch(match)
			attrs := m[1]
			body := m[2]
			return fmt.Sprintf("<script type=\"text/plain\" data-timecapsule-neutralized=\"true\"%s>%s</script>", attrs, body)
		})
	}

	// 4. Inject Archival Hermetic Header Comment
	headerComment := fmt.Sprintf("<!-- TimeCapsule Single-File Hermetic Web Archive -->\n"+
		"<!-- Source URL:  %s -->\n"+
		"<!-- Preserved:   %s UTC -->\n"+
		"<!-- Assets:      %d inlined, %d skipped -->\n",
		targetURL, time.Now().UTC().Format(time.RFC3339), inlinedCount, skippedCount)

	finalHTML := headerComment + processedHTML

	return &BundleResult{
		OriginalURL:       targetURL,
		Title:             title,
		OriginalSizeBytes: int64(len(rawHTML)),
		BundledSizeBytes:  int64(len(finalHTML)),
		BundledHTML:       finalHTML,
		AssetsInlined:     inlinedCount,
		AssetsSkipped:     skippedCount,
		Assets:            inlinedAssets,
		DurationMs:        time.Since(start).Milliseconds(),
		CreatedAt:         time.Now().UTC(),
	}, nil
}
