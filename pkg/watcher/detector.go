package watcher

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

var (
	commentRegex   = regexp.MustCompile(`(?s)<!--.*?-->`)
	scriptRegex    = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script>`)
	styleRegex     = regexp.MustCompile(`(?is)<style\b[^>]*>.*?</style>`)
	tagRegex       = regexp.MustCompile(`<[^>]+>`)
	isoDateRegex   = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?\b`)
	nonceRegex     = regexp.MustCompile(`(?i)(nonce|csrf[-_]?token|auth[-_]?key)=["'][a-zA-Z0-9_\-\.]+["']`)
	multiSpace     = regexp.MustCompile(`[ \t]+`)
	multiNewline   = regexp.MustCompile(`\n{3,}`)
)

// SanitizeContent removes dynamic noise, nonces, and ignored elements from raw HTML.
func SanitizeContent(htmlStr string, ignoreSelectors []string) string {
	s := commentRegex.ReplaceAllString(htmlStr, "")
	s = scriptRegex.ReplaceAllString(s, "")
	s = styleRegex.ReplaceAllString(s, "")

	// Remove custom ignored blocks by class or id keywords
	for _, sel := range ignoreSelectors {
		cleanSel := strings.TrimPrefix(strings.TrimPrefix(sel, "."), "#")
		if cleanSel == "" {
			continue
		}
		// Match tags with class/id containing cleanSel (RE2 does not support backreferences)
		pattern := fmt.Sprintf(`(?is)<(div|section|aside|p|span|header|footer|ul|li|table|article)\b[^>]*(?:class|id)=["'][^"']*?%s[^"']*?["'][^>]*>.*?</(div|section|aside|p|span|header|footer|ul|li|table|article)>`, regexp.QuoteMeta(cleanSel))
		re, err := regexp.Compile(pattern)
		if err == nil {
			s = re.ReplaceAllString(s, "")
		}
	}

	// Remove volatile timestamps and nonces
	s = isoDateRegex.ReplaceAllString(s, "[TIMESTAMP]")
	s = nonceRegex.ReplaceAllString(s, "$1=[NONCE]")

	// Strip remaining HTML tags to compare textual content and structure
	s = tagRegex.ReplaceAllString(s, " ")

	// Normalize whitespace
	s = multiSpace.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	var cleanLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleanLines = append(cleanLines, trimmed)
		}
	}

	res := strings.Join(cleanLines, "\n")
	res = multiNewline.ReplaceAllString(res, "\n\n")
	return res
}

// ComputeContentHash returns the SHA-256 digest of sanitized content.
func ComputeContentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

// ComputeTokenSimilarity calculates token-level Levenshtein similarity (0.0 to 1.0).
func ComputeTokenSimilarity(s1, s2 string) float64 {
	tokens1 := strings.Fields(s1)
	tokens2 := strings.Fields(s2)

	if len(tokens1) == 0 && len(tokens2) == 0 {
		return 1.0
	}
	if len(tokens1) == 0 || len(tokens2) == 0 {
		return 0.0
	}

	// Sample tokens if length is extremely large (> 2500 tokens) to guarantee sub-millisecond execution
	maxTok := 2500
	if len(tokens1) > maxTok {
		tokens1 = tokens1[:maxTok]
	}
	if len(tokens2) > maxTok {
		tokens2 = tokens2[:maxTok]
	}

	dist := tokenLevenshtein(tokens1, tokens2)
	maxLen := len(tokens1)
	if len(tokens2) > maxLen {
		maxLen = len(tokens2)
	}

	similarity := 1.0 - (float64(dist) / float64(maxLen))
	if similarity < 0.0 {
		similarity = 0.0
	}
	return similarity
}

func tokenLevenshtein(a, b []string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	prev := make([]int, lb+1)
	curr := make([]int, lb+1)

	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			ins := curr[j-1] + 1
			del := prev[j] + 1
			sub := prev[j-1] + cost

			min := ins
			if del < min {
				min = del
			}
			if sub < min {
				min = sub
			}
			curr[j] = min
		}
		copy(prev, curr)
	}

	return prev[lb]
}

// ClassifyDrift analyzes content differences and classifies drift severity and metrics.
func ClassifyDrift(oldClean, newClean string, statusCode int, threshold float64) (DriftSeverity, float64, string) {
	if statusCode >= 400 {
		return DriftOutage, 1.0, fmt.Sprintf("HTTP %d error response received", statusCode)
	}

	if oldClean == newClean {
		return DriftNone, 0.0, "Content identical (zero drift)"
	}

	similarity := ComputeTokenSimilarity(oldClean, newClean)
	delta := 1.0 - similarity

	if delta < threshold {
		return DriftNone, delta, fmt.Sprintf("Minor noise within threshold (delta: %.1f%%, threshold: %.1f%%)", delta*100, threshold*100)
	}

	if delta < 0.15 {
		return DriftCosmetic, delta, fmt.Sprintf("Cosmetic wording or formatting tweak (delta: %.1f%%)", delta*100)
	}

	if delta < 0.50 {
		return DriftSubstantial, delta, fmt.Sprintf("Substantial editorial content revision (delta: %.1f%%)", delta*100)
	}

	return DriftStructural, delta, fmt.Sprintf("Major structural or layout mutation (delta: %.1f%%)", delta*100)
}
