package tests

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Noooblien/timecapsule/pkg/archive"
	"github.com/Noooblien/timecapsule/pkg/reader"
	"github.com/Noooblien/timecapsule/pkg/server"
	"github.com/Noooblien/timecapsule/pkg/storage"
)

const readerSampleHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Autonomous Systems in Production | Engineering Blog</title>
  <meta name="author" content="Dr. Elena Vance">
  <meta name="description" content="A comprehensive breakdown of self-healing distributed agents.">
  <meta property="article:published_time" content="2026-08-15T10:00:00Z">
</head>
<body>
  <nav>
    <a href="/">Home</a>
    <a href="/archive">Archive</a>
    <button>Subscribe</button>
  </nav>

  <aside class="sidebar">
    <div class="ad-banner">Buy SuperCloud GPUs now!</div>
  </aside>

  <main>
    <article>
      <h1>Autonomous Systems in Production</h1>
      <p class="byline">By Dr. Elena Vance &bull; August 15, 2026</p>
      
      <p>Modern engineering demands <strong>resilience</strong> and <em>high availability</em> across distributed environments.</p>

      <h2>Key Performance Benchmarks</h2>
      <p>The following table illustrates latency benchmarks under synthetic chaos conditions:</p>

      <table>
        <thead>
          <tr>
            <th>Engine</th>
            <th>Throughput (req/s)</th>
            <th>P99 Latency</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td>TimeCapsule v2</td>
            <td>14,500</td>
            <td>1.2ms</td>
          </tr>
          <tr>
            <td>Standard Crawler</td>
            <td>850</td>
            <td>45.0ms</td>
          </tr>
        </tbody>
      </table>

      <h2>Architecture Implementation</h2>
      <p>Here is the core orchestration loop implemented in Go:</p>

      <pre><code class="language-go">package main

import "fmt"

func main() {
    fmt.Println("Agent initialized")
}
</code></pre>

      <blockquote>
        "Simplicity is prerequisite for reliability." &mdash; Edsger W. Dijkstra
      </blockquote>

      <h3>Feature Highlights</h3>
      <ul>
        <li>Zero external dependencies</li>
        <li>Sub-millisecond DOM streaming</li>
        <li>ISO 28500 WARC archiving</li>
      </ul>

      <p>Read more at <a href="https://example.com/research">the research lab</a>.</p>
    </article>
  </main>

  <footer>
    <p>&copy; 2026 Engineering Blog. All rights reserved.</p>
    <a href="/privacy">Privacy Policy</a>
  </footer>
</body>
</html>`

func TestReader_EndToEndMarkdown(t *testing.T) {
	opts := reader.DefaultOptions("https://blog.example.com/autonomous-systems")
	opts.IncludeFrontmatter = true
	article := reader.ExtractArticle(readerSampleHTML, opts)

	if !strings.Contains(article.Title, "Autonomous Systems in Production") {
		t.Fatalf("unexpected title: %q", article.Title)
	}
	if article.Author != "Dr. Elena Vance" {
		t.Fatalf("unexpected author: %q", article.Author)
	}
	if article.WordCount == 0 {
		t.Fatalf("expected non-zero word count")
	}
	if article.ReadingTimeMin < 1 {
		t.Fatalf("expected reading time >= 1 min")
	}

	mdWithFM := article.ToMarkdownWithFrontmatter()

	// Verify frontmatter presence
	if !strings.HasPrefix(mdWithFM, "---\n") {
		t.Fatalf("frontmatter header missing:\n%s", mdWithFM)
	}
	if !strings.Contains(mdWithFM, "Dr. Elena Vance") {
		t.Fatalf("author missing in frontmatter")
	}

	// Verify noise elements were stripped
	if strings.Contains(mdWithFM, "Buy SuperCloud GPUs now!") {
		t.Fatalf("noise ad banner was not stripped")
	}
	if strings.Contains(mdWithFM, "Privacy Policy") {
		t.Fatalf("footer was not stripped")
	}

	// Verify Markdown content elements
	if !strings.Contains(mdWithFM, "# Autonomous Systems in Production") {
		t.Fatalf("h1 missing in markdown")
	}
	if !strings.Contains(mdWithFM, "## Key Performance Benchmarks") {
		t.Fatalf("h2 missing in markdown")
	}
	if !strings.Contains(mdWithFM, "| Engine | Throughput (req/s) | P99 Latency |") {
		t.Fatalf("table markdown missing or malformed")
	}
	if !strings.Contains(mdWithFM, "```go") || !strings.Contains(mdWithFM, "fmt.Println(\"Agent initialized\")") {
		t.Fatalf("code fence with go syntax missing")
	}
	if !strings.Contains(mdWithFM, "> \"Simplicity is prerequisite for reliability.\"") {
		t.Fatalf("blockquote missing")
	}
	if !strings.Contains(mdWithFM, "- Zero external dependencies") {
		t.Fatalf("bullet list item missing")
	}
	if !strings.Contains(mdWithFM, "[the research lab](https://example.com/research)") {
		t.Fatalf("markdown link missing")
	}
}

func TestReader_ServerEndpoint(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tc_reader_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := storage.NewStore(filepath.Join(tempDir, "data"))
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	archiver := archive.NewArchiver(10 * time.Second)
	srv := server.NewServer(store, archiver)

	// Save test snapshot
	snap := &archive.Snapshot{
		ID:        "tc_reader_test_01",
		URL:       "https://blog.example.com/autonomous-systems",
		Title:     "Autonomous Systems in Production",
		HTML:      readerSampleHTML,
		Markdown:  archive.ExtractMarkdown(readerSampleHTML, "Autonomous Systems in Production"),
		Timestamp: time.Now().UTC(),
		WaybackTS: "20261018120000",
		SizeBytes: int64(len(readerSampleHTML)),
	}
	if err := store.Save(snap); err != nil {
		t.Fatalf("failed to save snapshot: %v", err)
	}

	// 1. Test GET /reader/tc_reader_test_01
	req := httptest.NewRequest(http.MethodGet, "/reader/tc_reader_test_01", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /reader/, got %d", w.Code)
	}
	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Fatalf("expected Content-Type text/html, got %s", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Autonomous Systems in Production") {
		t.Fatalf("reader view missing title in HTML body")
	}
	if !strings.Contains(body, "Dr. Elena Vance") {
		t.Fatalf("reader view missing author badge")
	}
	if !strings.Contains(body, "reader-article") {
		t.Fatalf("reader view missing reader-article CSS class")
	}

	// 2. Test 404 on non-existent snapshot
	req404 := httptest.NewRequest(http.MethodGet, "/reader/tc_non_existent", nil)
	w404 := httptest.NewRecorder()
	srv.ServeHTTP(w404, req404)
	if w404.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-existent snapshot, got %d", w404.Code)
	}
}
