package tests_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Noooblien/timecapsule/pkg/bundle"
)

func TestBundle_ResolveURL(t *testing.T) {
	base := "https://example.com/blog/article/index.html"

	tests := []struct {
		name     string
		ref      string
		expected string
	}{
		{
			name:     "relative same directory",
			ref:      "style.css",
			expected: "https://example.com/blog/article/style.css",
		},
		{
			name:     "relative parent directory",
			ref:      "../common.css",
			expected: "https://example.com/blog/common.css",
		},
		{
			name:     "absolute path",
			ref:      "/images/logo.png",
			expected: "https://example.com/images/logo.png",
		},
		{
			name:     "protocol relative",
			ref:      "//cdn.example.com/lib.js",
			expected: "https://cdn.example.com/lib.js",
		},
		{
			name:     "full https url",
			ref:      "https://external.com/font.woff2",
			expected: "https://external.com/font.woff2",
		},
		{
			name:     "data uri untouched",
			ref:      "data:image/png;base64,iVBORw0KGgoAAAANSUhEUg==",
			expected: "data:image/png;base64,iVBORw0KGgoAAAANSUhEUg==",
		},
		{
			name:     "javascript uri untouched",
			ref:      "javascript:void(0)",
			expected: "javascript:void(0)",
		},
		{
			name:     "anchor untouched",
			ref:      "#section-2",
			expected: "#section-2",
		},
		{
			name:     "empty string untouched",
			ref:      "",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := bundle.ResolveURL(base, tc.ref)
			if got != tc.expected {
				t.Errorf("ResolveURL(%q, %q) = %q; want %q", base, tc.ref, got, tc.expected)
			}
		})
	}
}

func TestBundle_DetectMimeType(t *testing.T) {
	tests := []struct {
		name       string
		urlStr     string
		headerMime string
		data       []byte
		expected   string
	}{
		{
			name:       "header with charset",
			urlStr:     "https://example.com/style.css",
			headerMime: "text/css; charset=UTF-8",
			expected:   "text/css",
		},
		{
			name:       "extension fallback png",
			urlStr:     "https://example.com/image.png",
			headerMime: "application/octet-stream",
			expected:   "image/png",
		},
		{
			name:       "extension fallback svg",
			urlStr:     "https://example.com/icon.svg",
			headerMime: "",
			expected:   "image/svg+xml",
		},
		{
			name:       "extension fallback woff2",
			urlStr:     "https://example.com/font.woff2",
			headerMime: "",
			expected:   "font/woff2",
		},
		{
			name:       "content sniffing html",
			urlStr:     "https://example.com/unknown",
			headerMime: "",
			data:       []byte("<!DOCTYPE html><html><body>Test</body></html>"),
			expected:   "text/html; charset=utf-8",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := bundle.DetectMimeType(tc.urlStr, tc.headerMime, tc.data)
			if got != tc.expected {
				t.Errorf("DetectMimeType() = %q; want %q", got, tc.expected)
			}
		})
	}
}

func TestBundle_SingleFileHermeticDOM(t *testing.T) {
	// Create mock static assets server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/style.css":
			w.Header().Set("Content-Type", "text/css")
			fmt.Fprint(w, `body { font-family: sans-serif; background-image: url('bg.png'); }`)
		case "/bg.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) // PNG header
		case "/banner.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}) // JPEG header
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	rawHTML := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
	<title>Test Page for Single-File Bundle</title>
	<link rel="stylesheet" href="%s/style.css">
	<script>alert("potentially malicious script!");</script>
</head>
<body>
	<h1>Welcome to TimeCapsule</h1>
	<img src="%s/banner.jpg" alt="Banner Image">
	<script src="tracker.js"></script>
</body>
</html>`, ts.URL, ts.URL)

	cfg := bundle.DefaultBundleConfig()
	cfg.Timeout = 5 * time.Second
	bundler := bundle.NewBundler(cfg)

	ctx := context.Background()
	res, err := bundler.BundleHTML(ctx, ts.URL+"/index.html", rawHTML)
	if err != nil {
		t.Fatalf("BundleHTML failed: %v", err)
	}

	if res.Title != "Test Page for Single-File Bundle" {
		t.Errorf("unexpected title: %q", res.Title)
	}

	if res.AssetsInlined != 3 { // style.css, bg.png (nested inside css), banner.jpg
		t.Errorf("expected 3 inlined assets, got %d", res.AssetsInlined)
	}

	// 1. Check style was inlined
	if !strings.Contains(res.BundledHTML, "<style data-timecapsule-inlined=") {
		t.Errorf("stylesheet was not converted to inline <style>")
	}

	// 2. Check nested bg.png was inlined into style
	expectedPNGBase64 := base64.StdEncoding.EncodeToString([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
	if !strings.Contains(res.BundledHTML, expectedPNGBase64) {
		t.Errorf("nested CSS url('bg.png') was not inlined into base64 Data URI")
	}

	// 3. Check banner image was inlined
	expectedJPGBase64 := base64.StdEncoding.EncodeToString([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10})
	if !strings.Contains(res.BundledHTML, expectedJPGBase64) {
		t.Errorf("banner.jpg was not inlined into base64 Data URI")
	}

	// 4. Check scripts were neutralized
	if !strings.Contains(res.BundledHTML, `type="text/plain" data-timecapsule-neutralized="true"`) {
		t.Errorf("active scripts were not neutralized")
	}
	if strings.Contains(res.BundledHTML, `<script>alert("potentially malicious script!");</script>`) {
		t.Errorf("un-neutralized active script found")
	}

	// 5. Check archival header banner
	if !strings.Contains(res.BundledHTML, "TimeCapsule Single-File Hermetic Web Archive") {
		t.Errorf("archival metadata comment header missing")
	}

	// 6. Test Scorecard rendering
	card := bundle.RenderBundleScorecard(res)
	if !strings.Contains(card, "TimeCapsule Single-File Verbatim DOM Serialization") {
		t.Errorf("scorecard title missing in:\n%s", card)
	}
	if !strings.Contains(card, "Assets Inlined:") {
		t.Errorf("scorecard missing inlined assets count in:\n%s", card)
	}
}

func TestBundle_AssetSizeCapAndFailures(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/huge.css":
			w.Header().Set("Content-Type", "text/css")
			// Return 2KB
			fmt.Fprint(w, strings.Repeat("/* comment */\n", 150))
		case "/404.css":
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	rawHTML := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
	<link rel="stylesheet" href="%s/huge.css">
	<link rel="stylesheet" href="%s/404.css">
</head>
<body><h1>Test</h1></body>
</html>`, ts.URL, ts.URL)

	cfg := bundle.DefaultBundleConfig()
	cfg.MaxAssetBytes = 100 // Cap at 100 bytes so huge.css exceeds limit
	bundler := bundle.NewBundler(cfg)

	ctx := context.Background()
	res, err := bundler.BundleHTML(ctx, ts.URL+"/index.html", rawHTML)
	if err != nil {
		t.Fatalf("BundleHTML failed: %v", err)
	}

	if res.AssetsInlined != 0 {
		t.Errorf("expected 0 assets inlined due to limits and 404, got %d", res.AssetsInlined)
	}

	if res.AssetsSkipped != 2 {
		t.Errorf("expected 2 skipped/failed assets, got %d", res.AssetsSkipped)
	}
}
