package reader

import (
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	noiseTags = []string{"script", "style", "noscript", "nav", "footer", "aside", "svg", "canvas", "form", "button", "iframe"}
	noiseTagRegexes = func() []*regexp.Regexp {
		var list []*regexp.Regexp
		for _, tag := range noiseTags {
			list = append(list, regexp.MustCompile(fmt.Sprintf(`(?is)<%s[^>]*>.*?</%s>`, tag, tag)))
			list = append(list, regexp.MustCompile(fmt.Sprintf(`(?i)<%s[^>]*/>`, tag)))
		}
		return list
	}()
	// Strip comments
	commentRegex = regexp.MustCompile(`(?s)<!--.*?-->`)

	// Metadata regexes
	metaAuthorRegex      = regexp.MustCompile(`(?i)<meta\s+[^>]*name=["']author["'][^>]*content=["']([^"']+)["']`)
	metaAuthorPropRegex  = regexp.MustCompile(`(?i)<meta\s+[^>]*content=["']([^"']+)["'][^>]*name=["']author["']`)
	metaDateRegex        = regexp.MustCompile(`(?i)<meta\s+[^>]*(?:name|property)=["'](?:article:published_time|date|pubdate)["'][^>]*content=["']([^"']+)["']`)
	metaDatePropRegex    = regexp.MustCompile(`(?i)<meta\s+[^>]*content=["']([^"']+)["'][^>]*(?:name|property)=["'](?:article:published_time|date|pubdate)["']`)
	metaDescRegex        = regexp.MustCompile(`(?i)<meta\s+[^>]*(?:name|property)=["'](?:description|og:description)["'][^>]*content=["']([^"']+)["']`)
	metaDescPropRegex    = regexp.MustCompile(`(?i)<meta\s+[^>]*content=["']([^"']+)["'][^>]*(?:name|property)=["'](?:description|og:description)["']`)
	timeTagRegex         = regexp.MustCompile(`(?i)<time\s+[^>]*datetime=["']([^"']+)["'][^>]*>(.*?)</time>`)
	titleTagRegex        = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	h1TagRegex           = regexp.MustCompile(`(?is)<h1[^>]*>(.*?)</h1>`)

	// Tag matching
	tagRegex    = regexp.MustCompile(`<[^>]+>`)
	wsNormRegex = regexp.MustCompile(`[ \t\r\f]+`)
	multiNlRegex = regexp.MustCompile(`\n{3,}`)
)

// Extract parses the HTML and generates a structured Article.
func Extract(rawHTML string, opts ReaderOptions) *Article {
	article := &Article{
		BaseURL:     opts.BaseURL,
		ExtractedAt: time.Now(),
	}

	// 1. Extract metadata before stripping tags
	if opts.ExtractMetadata {
		article.Author = extractFirstMatch(rawHTML, metaAuthorRegex, metaAuthorPropRegex)
		article.PublishedAt = extractFirstMatch(rawHTML, metaDateRegex, metaDatePropRegex)
		if article.PublishedAt == "" {
			if m := timeTagRegex.FindStringSubmatch(rawHTML); len(m) > 1 {
				article.PublishedAt = strings.TrimSpace(m[1])
			}
		}
		article.Description = html.UnescapeString(extractFirstMatch(rawHTML, metaDescRegex, metaDescPropRegex))
	}

	// Extract Title
	if m := titleTagRegex.FindStringSubmatch(rawHTML); len(m) > 1 {
		article.Title = strings.TrimSpace(stripTags(html.UnescapeString(m[1])))
	}
	if article.Title == "" {
		if m := h1TagRegex.FindStringSubmatch(rawHTML); len(m) > 1 {
			article.Title = strings.TrimSpace(stripTags(html.UnescapeString(m[1])))
		}
	}
	if article.Title == "" {
		article.Title = "Untitled Document"
	}

	// 2. Strip noise if requested
	cleanHTML := rawHTML
	if opts.StripNavAndAds {
		for _, reg := range noiseTagRegexes {
			cleanHTML = reg.ReplaceAllString(cleanHTML, "")
		}
	}
	cleanHTML = commentRegex.ReplaceAllString(cleanHTML, "")

	// 3. Convert HTML structure to Markdown
	converter := newHTMLConverter(opts)
	markdown := converter.Convert(cleanHTML)

	// Clean excessive newlines
	markdown = strings.TrimSpace(multiNlRegex.ReplaceAllString(markdown, "\n\n"))

	article.Content = markdown
	article.WordCount = countWords(markdown)
	article.ReadingTimeMin = (article.WordCount + 199) / 200 // 200 WPM
	if article.ReadingTimeMin < 1 && article.WordCount > 0 {
		article.ReadingTimeMin = 1
	}

	return article
}

func extractFirstMatch(s string, regexes ...*regexp.Regexp) string {
	for _, reg := range regexes {
		m := reg.FindStringSubmatch(s)
		if len(m) > 1 {
			val := strings.TrimSpace(m[1])
			if val != "" {
				return val
			}
		}
	}
	return ""
}

func stripTags(s string) string {
	return tagRegex.ReplaceAllString(s, "")
}

func countWords(s string) int {
	words := strings.Fields(s)
	count := 0
	for _, w := range words {
		// skip pure markdown delimiters like ---, |, #
		clean := strings.Trim(w, "#*-_|`~>:")
		if clean != "" {
			count++
		}
	}
	return count
}

func resolveURL(baseStr, refStr string) string {
	refStr = strings.TrimSpace(refStr)
	if refStr == "" || strings.HasPrefix(refStr, "#") || strings.HasPrefix(refStr, "javascript:") || strings.HasPrefix(refStr, "data:") {
		return refStr
	}
	if baseStr == "" {
		return refStr
	}

	baseURL, err := url.Parse(baseStr)
	if err != nil {
		return refStr
	}
	refURL, err := url.Parse(refStr)
	if err != nil {
		return refStr
	}
	return baseURL.ResolveReference(refURL).String()
}
