package visualdiff

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

var (
	tagPattern       = regexp.MustCompile(`(?s)<\s*(/)?\s*([a-zA-Z0-9\-_:]+)([^>]*)>`)
	attrPattern      = regexp.MustCompile(`([a-zA-Z0-9\-_:]+)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+)))?`)
	commentPattern   = regexp.MustCompile(`(?s)<!--.*?-->`)
	styleTagPattern  = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	scriptTagPattern = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)

	voidElements = map[string]bool{
		"area": true, "base": true, "br": true, "col": true, "embed": true,
		"hr": true, "img": true, "input": true, "link": true, "meta": true,
		"param": true, "source": true, "track": true, "wbr": true,
	}
)

// ParseHTML parses an HTML string into a DOM tree.
func ParseHTML(html string, config VisualDiffConfig) *DOMNode {
	// Pre-process: strip comments
	clean := commentPattern.ReplaceAllString(html, "")
	if config.IgnoreScripts {
		clean = scriptTagPattern.ReplaceAllString(clean, "")
	}
	if config.IgnoreStyles {
		clean = styleTagPattern.ReplaceAllString(clean, "")
	}

	root := &DOMNode{
		ID:            "root",
		Tag:           "document",
		Path:          "document",
		Attributes:    make(map[string]string),
		SemanticBlock: BlockMain,
	}

	stack := []*DOMNode{root}
	lastIdx := 0

	matches := tagPattern.FindAllStringSubmatchIndex(clean, -1)
	for _, m := range matches {
		tagStart := m[0]
		tagEnd := m[1]

		// Text between previous tag end and current tag start
		if tagStart > lastIdx {
			rawText := clean[lastIdx:tagStart]
			trimmedText := strings.TrimSpace(rawText)
			if trimmedText != "" && len(stack) > 0 {
				curr := stack[len(stack)-1]
				if curr.TextContent == "" {
					curr.TextContent = trimmedText
				} else {
					curr.TextContent += " " + trimmedText
				}
			}
		}
		lastIdx = tagEnd

		isClose := m[2] != -1 && clean[m[2]:m[3]] == "/"
		tagName := strings.ToLower(clean[m[4]:m[5]])
		rawAttrs := ""
		if m[6] != -1 && m[7] != -1 {
			rawAttrs = clean[m[6]:m[7]]
		}

		if isClose {
			// Find matching open tag in stack
			for i := len(stack) - 1; i > 0; i-- {
				if stack[i].Tag == tagName {
					stack = stack[:i]
					break
				}
			}
			continue
		}

		// Open tag or self-closing
		attrs := parseAttributes(rawAttrs)
		isSelfClosing := strings.HasSuffix(strings.TrimSpace(rawAttrs), "/") || voidElements[tagName]

		parent := stack[len(stack)-1]
		childIdx := len(parent.Children) + 1

		node := &DOMNode{
			Tag:        tagName,
			Attributes: attrs,
			Depth:      parent.Depth + 1,
		}

		// Compute Path
		node.Path = computePath(parent.Path, tagName, attrs, childIdx)
		node.ID = node.Path
		node.SemanticBlock = detectSemanticBlock(tagName, attrs, parent.SemanticBlock)

		parent.Children = append(parent.Children, node)

		if !isSelfClosing {
			stack = append(stack, node)
		}
	}

	// Compute subtree hashes and finalize tree
	finalizeTree(root)

	return root
}

func parseAttributes(raw string) map[string]string {
	attrs := make(map[string]string)
	matches := attrPattern.FindAllStringSubmatch(raw, -1)
	for _, m := range matches {
		key := strings.ToLower(m[1])
		val := ""
		if m[2] != "" {
			val = m[2]
		} else if m[3] != "" {
			val = m[3]
		} else if m[4] != "" {
			val = m[4]
		}
		attrs[key] = val
	}
	return attrs
}

func computePath(parentPath, tag string, attrs map[string]string, childIdx int) string {
	if tag == "html" {
		return "html"
	}
	if tag == "body" {
		return "body"
	}

	var selector string
	if id, ok := attrs["id"]; ok && id != "" {
		selector = fmt.Sprintf("%s#%s", tag, id)
	} else if class, ok := attrs["class"]; ok && class != "" {
		firstClass := strings.Fields(class)
		if len(firstClass) > 0 {
			selector = fmt.Sprintf("%s.%s", tag, firstClass[0])
		} else {
			selector = fmt.Sprintf("%s:nth-child(%d)", tag, childIdx)
		}
	} else {
		selector = fmt.Sprintf("%s:nth-child(%d)", tag, childIdx)
	}

	if parentPath == "" || parentPath == "document" || parentPath == "html" {
		return selector
	}
	return parentPath + " > " + selector
}

func detectSemanticBlock(tag string, attrs map[string]string, parentBlock SemanticBlockType) SemanticBlockType {
	id := strings.ToLower(attrs["id"])
	class := strings.ToLower(attrs["class"])
	combined := id + " " + class

	switch {
	case strings.Contains(combined, "price") || strings.Contains(combined, "pricing") || strings.Contains(combined, "tier") || strings.Contains(combined, "plan"):
		return BlockPricing
	case strings.Contains(combined, "hero") || strings.Contains(combined, "jumbotron") || strings.Contains(combined, "lead"):
		return BlockHero
	case strings.Contains(combined, "cta") || strings.Contains(combined, "call-to-action") || strings.Contains(combined, "signup") || strings.Contains(combined, "get-started"):
		return BlockCTA
	case tag == "header" || strings.Contains(combined, "header"):
		return BlockHeader
	case tag == "nav" || strings.Contains(combined, "nav") || strings.Contains(combined, "menu") || strings.Contains(combined, "navbar"):
		return BlockNavigation
	case tag == "footer" || strings.Contains(combined, "footer"):
		return BlockFooter
	case tag == "aside" || strings.Contains(combined, "sidebar"):
		return BlockSidebar
	case tag == "main" || strings.Contains(combined, "content"):
		return BlockMain
	default:
		if parentBlock != "" && parentBlock != BlockGeneral {
			return parentBlock
		}
		return BlockGeneral
	}
}

func finalizeTree(node *DOMNode) string {
	h := sha256.New()
	h.Write([]byte(node.Tag + ":" + node.Path + ":" + node.TextContent))
	for k, v := range node.Attributes {
		h.Write([]byte("," + k + "=" + v))
	}
	for _, child := range node.Children {
		childHash := finalizeTree(child)
		h.Write([]byte(";" + childHash))
	}
	hashStr := hex.EncodeToString(h.Sum(nil))
	node.SubtreeHash = hashStr
	return hashStr
}
