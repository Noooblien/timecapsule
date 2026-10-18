package reader

import (
	"fmt"
	"strings"
)

// Convert is a drop-in helper to convert raw HTML into clean, structured Markdown.
func Convert(rawHTML, title, baseURL string) string {
	opts := DefaultOptions(baseURL)
	article := Extract(rawHTML, opts)
	if title != "" && article.Title == "Untitled Document" {
		article.Title = title
	}
	return article.Content
}

// ExtractArticle extracts the structured article with metadata and word count.
func ExtractArticle(rawHTML string, opts ReaderOptions) *Article {
	return Extract(rawHTML, opts)
}

// RenderReaderHTML generates an offline, responsive, distraction-free HTML Reader page.
func RenderReaderHTML(article *Article) string {
	// Simple, elegant, typography-focused reader view template
	lines := strings.Split(article.Content, "\n")
	var htmlBody strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "### ") {
			htmlBody.WriteString(fmt.Sprintf("<h3>%s</h3>\n", strings.TrimPrefix(trimmed, "### ")))
		} else if strings.HasPrefix(trimmed, "## ") {
			htmlBody.WriteString(fmt.Sprintf("<h2>%s</h2>\n", strings.TrimPrefix(trimmed, "## ")))
		} else if strings.HasPrefix(trimmed, "# ") {
			htmlBody.WriteString(fmt.Sprintf("<h1>%s</h1>\n", strings.TrimPrefix(trimmed, "# ")))
		} else if strings.HasPrefix(trimmed, "> ") {
			htmlBody.WriteString(fmt.Sprintf("<blockquote>%s</blockquote>\n", strings.TrimPrefix(trimmed, "> ")))
		} else if strings.HasPrefix(trimmed, "- ") {
			htmlBody.WriteString(fmt.Sprintf("<ul><li>%s</li></ul>\n", strings.TrimPrefix(trimmed, "- ")))
		} else if strings.HasPrefix(trimmed, "```") {
			htmlBody.WriteString("<pre><code>")
		} else {
			htmlBody.WriteString(fmt.Sprintf("<p>%s</p>\n", trimmed))
		}
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s — Reader View</title>
<style>
  :root {
    --bg: #121214;
    --text: #e1e1e6;
    --muted: #a8a8b3;
    --accent: #8257e5;
    --border: #29292e;
    --card-bg: #202024;
  }
  @media (prefers-color-scheme: light) {
    :root {
      --bg: #f8f9fa;
      --text: #212529;
      --muted: #6c757d;
      --accent: #6f42c1;
      --border: #dee2e6;
      --card-bg: #ffffff;
    }
  }
  body {
    background: var(--bg);
    color: var(--text);
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
    line-height: 1.75;
    margin: 0;
    padding: 2rem 1rem;
    display: flex;
    justify-content: center;
  }
  article {
    max-width: 740px;
    width: 100%%;
  }
  header {
    border-bottom: 1px solid var(--border);
    margin-bottom: 2.5rem;
    padding-bottom: 1.5rem;
  }
  h1 { font-size: 2.25rem; line-height: 1.25; margin-bottom: 0.75rem; }
  h2 { font-size: 1.5rem; margin-top: 2rem; }
  h3 { font-size: 1.25rem; margin-top: 1.5rem; }
  .meta {
    color: var(--muted);
    font-size: 0.95rem;
    display: flex;
    flex-wrap: wrap;
    gap: 1.25rem;
  }
  blockquote {
    border-left: 4px solid var(--accent);
    margin: 1.5rem 0;
    padding-left: 1rem;
    color: var(--muted);
    font-style: italic;
  }
  pre {
    background: var(--card-bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 1rem;
    overflow-x: auto;
  }
  p { margin-bottom: 1.25rem; font-size: 1.1rem; }
  a { color: var(--accent); text-decoration: none; }
  a:hover { text-decoration: underline; }
</style>
</head>
<body>
<article class="reader-article">
  <header>
    <h1>%s</h1>
    <div class="meta">
      %s
      <span>📖 %d words</span>
      <span>⏱️ %d min read</span>
      %s
    </div>
  </header>
  <main>
    %s
  </main>
</article>
</body>
</html>`,
		article.Title,
		article.Title,
		renderAuthorTag(article.Author),
		article.WordCount,
		article.ReadingTimeMin,
		renderSourceTag(article.BaseURL),
		htmlBody.String(),
	)
}

func renderAuthorTag(author string) string {
	if author == "" {
		return ""
	}
	return fmt.Sprintf("<span>✍️ %s</span>", author)
}

func renderSourceTag(url string) string {
	if url == "" {
		return ""
	}
	return fmt.Sprintf("<span>🔗 <a href=\"%s\" target=\"_blank\">Original Source</a></span>", url)
}
