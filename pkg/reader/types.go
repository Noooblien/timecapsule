package reader

import (
	"fmt"
	"strings"
	"time"
)

// ReaderOptions controls how HTML is parsed and converted to Markdown.
type ReaderOptions struct {
	BaseURL          string // Base URL for resolving relative links/images
	IncludeFrontmatter bool   // Prepend YAML frontmatter
	ExtractMetadata  bool   // Scan <meta> tags for author, date, description
	PreserveImages   bool   // Include ![alt](src) images (default: true)
	PreserveLinks    bool   // Include [text](url) hyperlinks (default: true)
	StripNavAndAds   bool   // Strip nav, footer, sidebar, and boilerplate (default: true)
}

// DefaultOptions returns production standard reader settings.
func DefaultOptions(baseURL string) ReaderOptions {
	return ReaderOptions{
		BaseURL:          baseURL,
		IncludeFrontmatter: false,
		ExtractMetadata:  true,
		PreserveImages:   true,
		PreserveLinks:    true,
		StripNavAndAds:   true,
	}
}

// Article represents the distilled, clean textual content of a web page.
type Article struct {
	Title          string    `json:"title"`
	Author         string    `json:"author,omitempty"`
	PublishedAt    string    `json:"published_at,omitempty"`
	Description    string    `json:"description,omitempty"`
	BaseURL        string    `json:"base_url,omitempty"`
	Content        string    `json:"content"`
	WordCount      int       `json:"word_count"`
	ReadingTimeMin int       `json:"reading_time_min"`
	ExtractedAt    time.Time `json:"extracted_at"`
}

// ToMarkdownWithFrontmatter serializes the article with YAML frontmatter.
func (a *Article) ToMarkdownWithFrontmatter() string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("title: %q\n", a.Title))
	if a.Author != "" {
		b.WriteString(fmt.Sprintf("author: %q\n", a.Author))
	}
	if a.PublishedAt != "" {
		b.WriteString(fmt.Sprintf("date: %q\n", a.PublishedAt))
	}
	if a.Description != "" {
		b.WriteString(fmt.Sprintf("description: %q\n", a.Description))
	}
	if a.BaseURL != "" {
		b.WriteString(fmt.Sprintf("source: %q\n", a.BaseURL))
	}
	b.WriteString(fmt.Sprintf("word_count: %d\n", a.WordCount))
	b.WriteString(fmt.Sprintf("reading_time_min: %d\n", a.ReadingTimeMin))
	b.WriteString("---\n\n")
	b.WriteString(a.Content)
	return b.String()
}
