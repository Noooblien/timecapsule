package reader

import (
	"bytes"
	"fmt"
	"html"
	"regexp"
	"strings"
)

var (
	tagScanner = regexp.MustCompile(`(?s)(<[^>]+>|[^<]+)`)
	openTagReg = regexp.MustCompile(`^<([a-zA-Z0-9]+)([^>]*)>$`)
	closeTagReg = regexp.MustCompile(`^</([a-zA-Z0-9]+)>$`)
	attrReg    = regexp.MustCompile(`([a-zA-Z0-9_\-]+)=(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
)

type htmlConverter struct {
	opts ReaderOptions
}

func newHTMLConverter(opts ReaderOptions) *htmlConverter {
	return &htmlConverter{opts: opts}
}

// Convert converts the HTML string to formatted Markdown.
func (c *htmlConverter) Convert(rawHTML string) string {
	tokens := tagScanner.FindAllString(rawHTML, -1)

	var out bytes.Buffer

	// State tracking
	inPre := false
	preHeaderPending := false
	codeLang := ""
	inBlockquote := false
	inList := 0
	listType := "" // "ul" or "ol"
	listIndex := 0
	inTable := false
	tableRows := make([][]string, 0)
	currentRow := make([]string, 0)
	inCell := false
	var cellBuf bytes.Buffer

	linkHref := ""
	var linkBuf bytes.Buffer
	inLink := false

	headerLevel := 0
	var headerBuf bytes.Buffer

	for _, tok := range tokens {
		if strings.HasPrefix(tok, "<") {
			// It's a tag
			if strings.HasPrefix(tok, "</") {
				// Close tag
				m := closeTagReg.FindStringSubmatch(tok)
				if len(m) > 1 {
					tag := strings.ToLower(m[1])
					switch tag {
					case "h1", "h2", "h3", "h4", "h5", "h6":
						if headerLevel > 0 {
							prefix := strings.Repeat("#", headerLevel) + " "
							txt := strings.TrimSpace(headerBuf.String())
							if txt != "" {
								out.WriteString("\n\n" + prefix + txt + "\n\n")
							}
							headerLevel = 0
							headerBuf.Reset()
						}
					case "p":
						out.WriteString("\n\n")
					case "blockquote":
						inBlockquote = false
						out.WriteString("\n\n")
					case "pre":
						if preHeaderPending {
							out.WriteString(fmt.Sprintf("\n\n```%s\n", codeLang))
							preHeaderPending = false
						}
						inPre = false
						out.WriteString("\n```\n\n")
						codeLang = ""
					case "ul", "ol":
						if inList > 0 {
							inList--
						}
						out.WriteString("\n")
					case "li":
						out.WriteString("\n")
					case "table":
						inTable = false
						c.flushTable(&out, tableRows)
						tableRows = nil
					case "tr":
						if inTable && len(currentRow) > 0 {
							tableRows = append(tableRows, currentRow)
							currentRow = nil
						}
					case "th", "td":
						if inTable {
							currentRow = append(currentRow, strings.TrimSpace(cellBuf.String()))
							cellBuf.Reset()
							inCell = false
						}
					case "a":
						if inLink {
							inLink = false
							text := strings.TrimSpace(linkBuf.String())
							if text == "" {
								text = linkHref
							}
							if c.opts.PreserveLinks && linkHref != "" {
								resolved := resolveURL(c.opts.BaseURL, linkHref)
								c.writeText(&out, inPre, inBlockquote, inTable, inCell, &cellBuf, &headerBuf, headerLevel,
									fmt.Sprintf("[%s](%s)", text, resolved))
							} else {
								c.writeText(&out, inPre, inBlockquote, inTable, inCell, &cellBuf, &headerBuf, headerLevel, text)
							}
							linkHref = ""
							linkBuf.Reset()
						}
					case "strong", "b":
						c.writeText(&out, inPre, inBlockquote, inTable, inCell, &cellBuf, &headerBuf, headerLevel, "**")
					case "em", "i":
						c.writeText(&out, inPre, inBlockquote, inTable, inCell, &cellBuf, &headerBuf, headerLevel, "*")
					case "code":
						if !inPre {
							c.writeText(&out, inPre, inBlockquote, inTable, inCell, &cellBuf, &headerBuf, headerLevel, "`")
						}
					}
				}
				continue
			}

			// Open or self-closing tag
			m := openTagReg.FindStringSubmatch(tok)
			if len(m) > 1 {
				tag := strings.ToLower(m[1])
				attrs := parseAttributes(m[2])

				switch tag {
				case "h1":
					headerLevel = 1
					headerBuf.Reset()
				case "h2":
					headerLevel = 2
					headerBuf.Reset()
				case "h3":
					headerLevel = 3
					headerBuf.Reset()
				case "h4":
					headerLevel = 4
					headerBuf.Reset()
				case "h5":
					headerLevel = 5
					headerBuf.Reset()
				case "h6":
					headerLevel = 6
					headerBuf.Reset()
				case "p":
					out.WriteString("\n\n")
					if inBlockquote {
						out.WriteString("> ")
					}
				case "blockquote":
					inBlockquote = true
					out.WriteString("\n\n> ")
				case "pre":
					inPre = true
					preHeaderPending = true
					codeLang = detectLanguage(attrs)
				case "code":
					if inPre {
						if codeLang == "" {
							codeLang = detectLanguage(attrs)
						}
					} else {
						c.writeText(&out, inPre, inBlockquote, inTable, inCell, &cellBuf, &headerBuf, headerLevel, "`")
					}
				case "ul":
					inList++
					listType = "ul"
					out.WriteString("\n")
				case "ol":
					inList++
					listType = "ol"
					listIndex = 0
					out.WriteString("\n")
				case "li":
					indent := strings.Repeat("  ", inList-1)
					if listType == "ol" {
						listIndex++
						out.WriteString(fmt.Sprintf("%s%d. ", indent, listIndex))
					} else {
						out.WriteString(fmt.Sprintf("%s- ", indent))
					}
				case "table":
					inTable = true
					tableRows = nil
					currentRow = nil
				case "tr":
					if inTable {
						currentRow = make([]string, 0)
					}
				case "th", "td":
					if inTable {
						inCell = true
						cellBuf.Reset()
					}
				case "a":
					linkHref = attrs["href"]
					inLink = true
					linkBuf.Reset()
				case "img":
					if c.opts.PreserveImages {
						src := attrs["src"]
						alt := attrs["alt"]
						if src != "" {
							resolved := resolveURL(c.opts.BaseURL, src)
							c.writeText(&out, inPre, inBlockquote, inTable, inCell, &cellBuf, &headerBuf, headerLevel,
								fmt.Sprintf("![%s](%s)", alt, resolved))
						}
					}
				case "strong", "b":
					c.writeText(&out, inPre, inBlockquote, inTable, inCell, &cellBuf, &headerBuf, headerLevel, "**")
				case "em", "i":
					c.writeText(&out, inPre, inBlockquote, inTable, inCell, &cellBuf, &headerBuf, headerLevel, "*")
				case "br":
					out.WriteString("\n")
				case "hr":
					out.WriteString("\n\n---\n\n")
				}
			}
			continue
		}

		// Text content token
		text := html.UnescapeString(tok)
		if inPre {
			if preHeaderPending {
				out.WriteString(fmt.Sprintf("\n\n```%s\n", codeLang))
				preHeaderPending = false
			}
			out.WriteString(text)
		} else if inLink {
			linkBuf.WriteString(text)
		} else if headerLevel > 0 {
			headerBuf.WriteString(text)
		} else if inTable && inCell {
			cellBuf.WriteString(text)
		} else {
			cleanText := wsNormRegex.ReplaceAllString(text, " ")
			if inBlockquote && strings.HasSuffix(out.String(), "> ") {
				cleanText = strings.TrimLeft(cleanText, " \t\r\n")
			}
			out.WriteString(cleanText)
		}
	}

	return out.String()
}

func (c *htmlConverter) writeText(out *bytes.Buffer, inPre, inBlockquote, inTable, inCell bool, cellBuf, headerBuf *bytes.Buffer, headerLevel int, s string) {
	if headerLevel > 0 {
		headerBuf.WriteString(s)
	} else if inTable && inCell {
		cellBuf.WriteString(s)
	} else {
		out.WriteString(s)
	}
}

func (c *htmlConverter) flushTable(out *bytes.Buffer, rows [][]string) {
	if len(rows) == 0 {
		return
	}

	// Calculate column counts
	maxCols := 0
	for _, r := range rows {
		if len(r) > maxCols {
			maxCols = len(r)
		}
	}
	if maxCols == 0 {
		return
	}

	out.WriteString("\n\n")

	// Header row
	headerRow := rows[0]
	out.WriteString("|")
	for i := 0; i < maxCols; i++ {
		val := ""
		if i < len(headerRow) {
			val = cleanTableCell(headerRow[i])
		}
		out.WriteString(" " + val + " |")
	}
	out.WriteString("\n|")

	// Separator row
	for i := 0; i < maxCols; i++ {
		out.WriteString(" --- |")
	}
	out.WriteString("\n")

	// Data rows
	for rowIdx := 1; rowIdx < len(rows); rowIdx++ {
		r := rows[rowIdx]
		out.WriteString("|")
		for i := 0; i < maxCols; i++ {
			val := ""
			if i < len(r) {
				val = cleanTableCell(r[i])
			}
			out.WriteString(" " + val + " |")
		}
		out.WriteString("\n")
	}
	out.WriteString("\n")
}

func cleanTableCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

func parseAttributes(attrStr string) map[string]string {
	attrs := make(map[string]string)
	matches := attrReg.FindAllStringSubmatch(attrStr, -1)
	for _, m := range matches {
		if len(m) > 1 {
			name := strings.ToLower(m[1])
			val := m[2]
			if val == "" {
				val = m[3]
			}
			if val == "" {
				val = m[4]
			}
			attrs[name] = val
		}
	}
	return attrs
}

func detectLanguage(attrs map[string]string) string {
	class := attrs["class"]
	if class == "" {
		return ""
	}
	for _, part := range strings.Fields(class) {
		if strings.HasPrefix(part, "language-") {
			return strings.TrimPrefix(part, "language-")
		}
		if strings.HasPrefix(part, "lang-") {
			return strings.TrimPrefix(part, "lang-")
		}
	}
	return ""
}
