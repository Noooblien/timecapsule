package reader

import (
	"strings"
	"testing"
)

func TestReader_HeadingsAndParagraphs(t *testing.T) {
	htmlInput := `
<!DOCTYPE html>
<html>
<head><title>My Blog Post</title></head>
<body>
  <h1>Welcome to TimeCapsule</h1>
  <p>TimeCapsule is an offline web archival engine written in Go.</p>
  <h2>Key Features</h2>
  <p>It supports WARC, single-file bundles, and clean markdown.</p>
  <h3>Subheading Details</h3>
  <p>Here are more detailed descriptions.</p>
</body>
</html>`

	opts := DefaultOptions("https://example.com/post")
	article := Extract(htmlInput, opts)

	if article.Title != "My Blog Post" {
		t.Fatalf("expected title 'My Blog Post', got %q", article.Title)
	}

	content := article.Content
	if !strings.Contains(content, "# Welcome to TimeCapsule") {
		t.Fatalf("missing h1 markdown: %s", content)
	}
	if !strings.Contains(content, "## Key Features") {
		t.Fatalf("missing h2 markdown: %s", content)
	}
	if !strings.Contains(content, "### Subheading Details") {
		t.Fatalf("missing h3 markdown: %s", content)
	}
	if !strings.Contains(content, "TimeCapsule is an offline web archival engine written in Go.") {
		t.Fatalf("missing paragraph content: %s", content)
	}
	if article.WordCount <= 0 {
		t.Fatalf("expected positive word count, got %d", article.WordCount)
	}
	if article.ReadingTimeMin != 1 {
		t.Fatalf("expected reading time 1 min, got %d", article.ReadingTimeMin)
	}
}

func TestReader_CodeBlocksAndTables(t *testing.T) {
	htmlInput := `
<html>
<body>
  <h2>Benchmark Table</h2>
  <table>
    <thead>
      <tr><th>Tool</th><th>Latency</th><th>Throughput</th></tr>
    </thead>
    <tbody>
      <tr><td>TimeCapsule</td><td>1.2ms</td><td>8500 rps</td></tr>
      <tr><td>Legacy VCR</td><td>15.8ms</td><td>420 rps</td></tr>
    </tbody>
  </table>

  <h2>Code Example</h2>
  <pre><code class="language-go">func main() {
    fmt.Println("Hello, World!")
}</code></pre>
</body>
</html>`

	opts := DefaultOptions("https://example.com")
	article := Extract(htmlInput, opts)

	content := article.Content

	// Check table
	if !strings.Contains(content, "| Tool | Latency | Throughput |") {
		t.Fatalf("markdown table header missing in: %s", content)
	}
	if !strings.Contains(content, "| --- | --- | --- |") {
		t.Fatalf("markdown table separator missing in: %s", content)
	}
	if !strings.Contains(content, "| TimeCapsule | 1.2ms | 8500 rps |") {
		t.Fatalf("markdown table row missing in: %s", content)
	}

	// Check code block
	if !strings.Contains(content, "```go\nfunc main() {\n    fmt.Println(\"Hello, World!\")\n}\n```") {
		t.Fatalf("fenced code block missing or malformed in: %s", content)
	}
}

func TestReader_ListsAndBlockquotes(t *testing.T) {
	htmlInput := `
<html>
<body>
  <blockquote>This is an important quote from the creator.</blockquote>
  <ul>
    <li>First bullet</li>
    <li>Second bullet</li>
  </ul>
  <ol>
    <li>Step one</li>
    <li>Step two</li>
  </ol>
</body>
</html>`

	opts := DefaultOptions("")
	article := Extract(htmlInput, opts)
	content := article.Content

	if !strings.Contains(content, "> This is an important quote from the creator.") {
		t.Fatalf("blockquote missing in: %s", content)
	}
	if !strings.Contains(content, "- First bullet") || !strings.Contains(content, "- Second bullet") {
		t.Fatalf("unordered list missing in: %s", content)
	}
	if !strings.Contains(content, "1. Step one") || !strings.Contains(content, "2. Step two") {
		t.Fatalf("ordered list missing in: %s", content)
	}
}

func TestReader_InlineFormattingAndLinks(t *testing.T) {
	htmlInput := `
<html>
<body>
  <p>Here is <strong>bold</strong> and <em>italic</em> text with <code>inline_code()</code>.</p>
  <p>Check out <a href="/docs/intro">the documentation</a> and an image: <img src="/images/logo.png" alt="TimeCapsule Logo"></p>
</body>
</html>`

	opts := DefaultOptions("https://timecapsule.dev/blog/intro")
	article := Extract(htmlInput, opts)
	content := article.Content

	if !strings.Contains(content, "**bold**") {
		t.Fatalf("bold markdown missing in: %s", content)
	}
	if !strings.Contains(content, "*italic*") {
		t.Fatalf("italic markdown missing in: %s", content)
	}
	if !strings.Contains(content, "`inline_code()`") {
		t.Fatalf("inline code markdown missing in: %s", content)
	}
	// Check relative link resolution
	if !strings.Contains(content, "[the documentation](https://timecapsule.dev/docs/intro)") {
		t.Fatalf("resolved link missing in: %s", content)
	}
	// Check image resolution
	if !strings.Contains(content, "![TimeCapsule Logo](https://timecapsule.dev/images/logo.png)") {
		t.Fatalf("resolved image missing in: %s", content)
	}
}

func TestReader_MetadataAndFrontmatter(t *testing.T) {
	htmlInput := `
<!DOCTYPE html>
<html>
<head>
  <title>Article with Meta</title>
  <meta name="author" content="Grace Hopper">
  <meta property="article:published_time" content="2026-10-18T12:00:00Z">
  <meta name="description" content="A comprehensive guide to compiler design and debugging.">
</head>
<body>
  <h1>Article with Meta</h1>
  <p>The compiler was revolutionary for computing history.</p>
</body>
</html>`

	opts := DefaultOptions("https://example.com/compiler")
	opts.IncludeFrontmatter = true
	article := Extract(htmlInput, opts)

	if article.Author != "Grace Hopper" {
		t.Fatalf("expected author 'Grace Hopper', got %q", article.Author)
	}
	if article.PublishedAt != "2026-10-18T12:00:00Z" {
		t.Fatalf("expected published date '2026-10-18T12:00:00Z', got %q", article.PublishedAt)
	}
	if article.Description != "A comprehensive guide to compiler design and debugging." {
		t.Fatalf("unexpected description: %q", article.Description)
	}

	fm := article.ToMarkdownWithFrontmatter()
	if !strings.HasPrefix(fm, "---\n") {
		t.Fatalf("expected frontmatter opening delimiter")
	}
	if !strings.Contains(fm, `author: "Grace Hopper"`) {
		t.Fatalf("author missing in frontmatter: %s", fm)
	}
	if !strings.Contains(fm, `word_count:`) {
		t.Fatalf("word_count missing in frontmatter: %s", fm)
	}
}

func TestReader_NoiseStripping(t *testing.T) {
	htmlInput := `
<html>
<head><style>body { color: red; }</style></head>
<body>
  <nav><a href="/">Home</a> | <a href="/about">About</a></nav>
  <aside class="sidebar"><h3>Ads</h3><p>Buy our product</p></aside>
  <main>
    <h1>Pure Article</h1>
    <p>This is the essential article body that should remain intact.</p>
  </main>
  <footer><p>&copy; 2026 Company Inc. All rights reserved.</p></footer>
  <script>console.log("analytics tracking");</script>
</body>
</html>`

	opts := DefaultOptions("https://example.com")
	article := Extract(htmlInput, opts)
	content := article.Content

	if strings.Contains(content, "Buy our product") {
		t.Fatalf("aside noise was not stripped: %s", content)
	}
	if strings.Contains(content, "analytics tracking") {
		t.Fatalf("script was not stripped: %s", content)
	}
	if strings.Contains(content, "Home | About") {
		t.Fatalf("nav was not stripped: %s", content)
	}
	if strings.Contains(content, "All rights reserved") {
		t.Fatalf("footer was not stripped: %s", content)
	}
	if !strings.Contains(content, "This is the essential article body that should remain intact.") {
		t.Fatalf("main article body missing: %s", content)
	}
}

func TestReader_RenderReaderHTML(t *testing.T) {
	article := &Article{
		Title:          "Sample Reader Title",
		Author:         "Ada Lovelace",
		BaseURL:        "https://example.org/sample",
		Content:        "# Sample Reader Title\n\n> Remarkable quote.\n\n- First point\n\nNormal paragraph text.",
		WordCount:      240,
		ReadingTimeMin: 2,
	}

	htmlPage := RenderReaderHTML(article)
	if !strings.Contains(htmlPage, "Sample Reader Title — Reader View") {
		t.Fatalf("title missing in reader HTML: %s", htmlPage)
	}
	if !strings.Contains(htmlPage, "Ada Lovelace") {
		t.Fatalf("author missing in reader HTML: %s", htmlPage)
	}
	if !strings.Contains(htmlPage, "240 words") {
		t.Fatalf("word count missing in reader HTML: %s", htmlPage)
	}
	if !strings.Contains(htmlPage, "<blockquote>Remarkable quote.</blockquote>") {
		t.Fatalf("rendered blockquote missing in reader HTML: %s", htmlPage)
	}
}
