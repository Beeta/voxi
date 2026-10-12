// Package md parses and renders the Markdown a model returns for a summary.
// The summary instruction is the caller's own, with nothing appended to it,
// so a reply can be any shape at all; this package handles the Markdown one.
//
// The parser is a copy of the one MyGo uses for its release notes
// (github.com/egoist/mygo@v0.3.5, plugins/updater/internal/markdown, MIT),
// which cannot be imported because it sits under an internal path. Three
// changes are local: a soft break between CJK characters no longer becomes a
// space, tables are recognised rather than read as paragraphs, and the HTML
// writer was dropped since the summary renders natively.
package md

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// BlockKind is what a Block is.
type BlockKind uint8

const (
	Paragraph BlockKind = iota
	Heading
	List
	Quote
	Code
	Rule
	Table
)

// Block is a block of a document.
type Block struct {
	Kind BlockKind
	// Level is the level of a heading, from 1 to 6.
	Level int
	// Inlines are the text of a paragraph or heading.
	Inlines []Inline
	// Text is the text of fenced code.
	Text string
	// Ordered is set for a numbered list, whose Items hold the blocks of
	// each item; Blocks are those of a quote.
	Ordered bool
	Items   [][]Block
	Blocks  []Block
	// Rows are the cells of a table; the first row is its header.
	Rows [][]string
}

// InlineKind is what an Inline is.
type InlineKind uint8

const (
	Text InlineKind = iota
	CodeSpan
	Emphasis
	Strong
	Link
)

// Inline is a run of text in a paragraph or heading.
type Inline struct {
	Kind InlineKind
	// Text is the text of a Text or CodeSpan.
	Text string
	// Children are the text of an Emphasis, Strong or Link.
	Children []Inline
	// URL is where a Link goes: an http, https or mailto URL.
	URL string
}

// Parse parses a Markdown document.
func Parse(src string) []Block {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\t", "    ")
	return parseBlocks(strings.Split(src, "\n"))
}

// parseBlocks parses lines as blocks.
func parseBlocks(lines []string) []Block {
	var blocks []Block
	var para []string
	flush := func() {
		if len(para) > 0 {
			blocks = append(blocks, Block{Kind: Paragraph, Inlines: parseInline(strings.Join(para, "\n"))})
			para = nil
		}
	}
	for i := 0; i < len(lines); {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			flush()
			i++
		case isFence(trimmed):
			flush()
			fence := trimmed[:3]
			var code []string
			for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), fence); i++ {
				code = append(code, lines[i])
			}
			i++
			blocks = append(blocks, Block{Kind: Code, Text: strings.Join(code, "\n")})
		case heading(trimmed) > 0:
			flush()
			n := heading(trimmed)
			text := strings.TrimRight(strings.TrimSpace(trimmed[n:]), "#")
			blocks = append(blocks, Block{Kind: Heading, Level: n, Inlines: parseInline(strings.TrimSpace(text))})
			i++
		case isRule(trimmed):
			flush()
			blocks = append(blocks, Block{Kind: Rule})
			i++
		case startsTable(lines, i):
			flush()
			var rows [][]string
			rows, i = parseTable(lines, i)
			blocks = append(blocks, Block{Kind: Table, Rows: rows})
		case strings.HasPrefix(trimmed, ">"):
			flush()
			var quote []string
			for ; i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), ">"); i++ {
				q := strings.TrimPrefix(strings.TrimSpace(lines[i]), ">")
				quote = append(quote, strings.TrimPrefix(q, " "))
			}
			blocks = append(blocks, Block{Kind: Quote, Blocks: parseBlocks(quote)})
		default:
			if _, _, ok := listMarker(line); ok && (len(para) == 0 || startsList(line)) {
				flush()
				var list Block
				list, i = parseList(lines, i)
				blocks = append(blocks, list)
				continue
			}
			para = append(para, trimmed)
			i++
		}
	}
	flush()
	return blocks
}

// parseList parses the list starting at lines[i] and returns it with the
// index of the line after it.
func parseList(lines []string, i int) (Block, int) {
	indent, ordered, _ := listMarker(lines[i])
	list := Block{Kind: List, Ordered: ordered}
	for i < len(lines) {
		ind, ord, ok := listMarker(lines[i])
		if !ok || ind != indent || ord != ordered {
			break
		}
		// The item holds the rest of its first line, the lines indented to
		// its content, lists nested less deep than that, and the lines
		// that continue its paragraph.
		width := markerWidth(lines[i])
		item := []string{lines[i][width:]}
		blank := false
	lines:
		for i++; i < len(lines); i++ {
			line := lines[i]
			if strings.TrimSpace(line) == "" {
				blank = true
				item = append(item, "")
				continue
			}
			ind := leadingSpaces(line)
			_, _, marker := listMarker(line)
			switch {
			case ind >= width:
				item = append(item, line[width:])
			case ind > indent && marker:
				item = append(item, line[ind:])
			case !blank && !marker && !isBlockStart(line):
				item = append(item, strings.TrimSpace(line))
			default:
				break lines
			}
			blank = false
		}
		for len(item) > 0 && item[len(item)-1] == "" {
			item = item[:len(item)-1]
		}
		list.Items = append(list.Items, parseBlocks(item))
	}
	return list, i
}

// listMarker reports whether line starts a list item, with the indentation
// of its marker and whether the list is ordered.
func listMarker(line string) (indent int, ordered, ok bool) {
	indent = leadingSpaces(line)
	rest := line[indent:]
	if len(rest) >= 2 && strings.ContainsRune("-*+", rune(rest[0])) && rest[1] == ' ' {
		return indent, false, !isRule(strings.TrimSpace(rest))
	}
	n := 0
	for n < len(rest) && n < 9 && rest[n] >= '0' && rest[n] <= '9' {
		n++
	}
	if n > 0 && len(rest) > n+1 && (rest[n] == '.' || rest[n] == ')') && rest[n+1] == ' ' {
		return indent, true, true
	}
	return 0, false, false
}

// markerWidth returns the column where the content of the list item that
// line starts begins: after its marker and the space after it.
func markerWidth(line string) int {
	w := leadingSpaces(line)
	for w < len(line) && line[w] != ' ' {
		w++
	}
	start := w
	for w < len(line) && line[w] == ' ' && w-start < 4 {
		w++
	}
	return w
}

// startsList reports whether line starts a list item that may interrupt a
// paragraph: a bullet, or an ordered item numbered 1.
func startsList(line string) bool {
	_, ordered, ok := listMarker(line)
	if !ok {
		return false
	}
	return !ordered || strings.HasPrefix(strings.TrimSpace(line), "1")
}

func isBlockStart(line string) bool {
	t := strings.TrimSpace(line)
	return heading(t) > 0 || isFence(t) || isRule(t) || strings.HasPrefix(t, ">")
}

func leadingSpaces(s string) int {
	n := 0
	for n < len(s) && s[n] == ' ' {
		n++
	}
	return n
}

func heading(t string) int {
	n := 0
	for n < len(t) && n < 7 && t[n] == '#' {
		n++
	}
	if n == 0 || n > 6 || (len(t) > n && t[n] != ' ') {
		return 0
	}
	return n
}

func isFence(t string) bool { return strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") }

func isRule(t string) bool {
	if len(t) < 3 || !strings.ContainsRune("-*_", rune(t[0])) {
		return false
	}
	n := 0
	for _, c := range t {
		switch {
		case c == rune(t[0]):
			n++
		case c != ' ':
			return false
		}
	}
	return n >= 3
}

// startsTable reports whether a table starts at lines[i]: a row of cells
// with a delimiter row under it. The delimiter is what tells a table from a
// paragraph that happens to use pipes.
func startsTable(lines []string, i int) bool {
	if i+1 >= len(lines) || !isTableRow(lines[i]) {
		return false
	}
	return isTableDelim(strings.TrimSpace(lines[i+1]))
}

// isTableRow reports whether line looks like a table row: pipes around cells.
func isTableRow(line string) bool {
	t := strings.TrimSpace(line)
	return strings.HasPrefix(t, "|") && strings.HasSuffix(t, "|") && strings.Count(t, "|") >= 2
}

// isTableDelim reports whether line is a table's delimiter row: cells of
// dashes, each optionally with a colon for the column's alignment.
func isTableDelim(t string) bool {
	if !strings.ContainsRune(t, '-') {
		return false
	}
	for _, c := range t {
		switch c {
		case '|', '-', ':', ' ':
		default:
			return false
		}
	}
	return true
}

// parseTable parses the table starting at lines[i]: a header row, the
// delimiter row, then rows until one that is not a row. It returns the
// cells and the index of the line after the table.
func parseTable(lines []string, i int) ([][]string, int) {
	rows := [][]string{tableCells(lines[i])}
	for i += 2; i < len(lines) && isTableRow(lines[i]); i++ {
		rows = append(rows, tableCells(lines[i]))
	}
	return rows, i
}

// tableCells splits a table row into its cells.
func tableCells(line string) []string {
	t := strings.Trim(strings.TrimSpace(line), "|")
	parts := strings.Split(t, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// inlines collects the inlines of a text, merging runs of text.
type inlines []Inline

func (in *inlines) text(s string) {
	if n := len(*in); n > 0 && (*in)[n-1].Kind == Text {
		(*in)[n-1].Text += s
		return
	}
	*in = append(*in, Inline{Kind: Text, Text: s})
}

// link adds children linked to href, or the children alone when the URL is
// not one the app opens.
func (in *inlines) link(href string, children []Inline) {
	if !SafeURL(href) {
		for _, c := range children {
			if c.Kind == Text {
				in.text(c.Text)
			} else {
				*in = append(*in, c)
			}
		}
		return
	}
	*in = append(*in, Inline{Kind: Link, URL: href, Children: children})
}

// parseInline parses the text of a paragraph or heading.
func parseInline(s string) []Inline {
	var in inlines
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '\\' && i+1 < len(s) && isPunct(s[i+1]):
			in.text(s[i+1 : i+2])
			i += 2
			continue
		case c == '\n':
			// A newline inside a paragraph is a soft break, which reads as
			// a space — except between CJK characters, where no space is
			// written and "第一点\n第二点" must not gain one.
			if !wideAround(s, i) {
				in.text(" ")
			}
			i++
			continue
		case c == '`':
			n := run(s[i:], '`')
			if end := strings.Index(s[i+n:], s[i:i+n]); end >= 0 {
				code := strings.TrimSpace(strings.ReplaceAll(s[i+n:i+n+end], "\n", " "))
				in = append(in, Inline{Kind: CodeSpan, Text: code})
				i += n + end + n
				continue
			}
			in.text(s[i : i+n])
			i += n
			continue
		case c == '[' || c == '!' && strings.HasPrefix(s[i:], "!["):
			image := c == '!'
			start := i
			if image {
				start++
			}
			if text, href, n, ok := parseLink(s[start:]); ok {
				var children []Inline
				if image {
					children = []Inline{{Kind: Text, Text: text}}
				} else {
					children = parseInline(text)
				}
				in.link(href, children)
				i = start + n
				continue
			}
		case c == '<':
			if end := strings.IndexByte(s[i:], '>'); end > 0 {
				if u := s[i+1 : i+end]; !strings.ContainsAny(u, " <") && SafeURL(u) {
					in.link(u, []Inline{{Kind: Text, Text: u}})
					i += end + 1
					continue
				}
			}
		case c == '*' || c == '_':
			n := min(run(s[i:], c), 2)
			if c == '_' && i > 0 && isWord(s[i-1]) {
				break
			}
			delim := s[i : i+n]
			rest := s[i+n:]
			if rest != "" && rest[0] != ' ' && rest[0] != '\n' {
				if end := closing(rest, delim); end > 0 {
					kind := Emphasis
					if n == 2 {
						kind = Strong
					}
					in = append(in, Inline{Kind: kind, Children: parseInline(rest[:end])})
					i += n + end + n
					continue
				}
			}
			in.text(delim)
			i += n
			continue
		case c == 'h' && (i == 0 || !isWord(s[i-1])) && (strings.HasPrefix(s[i:], "https://") || strings.HasPrefix(s[i:], "http://")):
			end := i
			for end < len(s) && s[end] > ' ' && s[end] != '<' {
				end++
			}
			u := strings.TrimRight(s[i:end], ".,:;!?\"')")
			if strings.Count(u, "(") > strings.Count(u, ")") && end > i+len(u) && s[i+len(u)] == ')' {
				u += ")"
			}
			in.link(u, []Inline{{Kind: Text, Text: u}})
			i += len(u)
			continue
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		in.text(s[i : i+size])
		i += size
	}
	return in
}

// parseLink parses "[text](url)" or `[text](url "title")` at the start of
// s, returning the text, the URL and the length of the link.
func parseLink(s string) (text, href string, n int, ok bool) {
	depth := 0
	end := -1
	for i := 0; i < len(s) && end < 0; i++ {
		switch s[i] {
		case '\\':
			i++
		case '[':
			depth++
		case ']':
			if depth--; depth == 0 {
				end = i
			}
		}
	}
	if end < 0 || !strings.HasPrefix(s[end+1:], "(") {
		return "", "", 0, false
	}
	close := strings.IndexByte(s[end+2:], ')')
	if close < 0 {
		return "", "", 0, false
	}
	dest := strings.TrimSpace(s[end+2 : end+2+close])
	if sp := strings.IndexAny(dest, " \n"); sp >= 0 {
		dest = dest[:sp] // drop the title
	}
	dest = strings.TrimSuffix(strings.TrimPrefix(dest, "<"), ">")
	return s[1:end], dest, end + 2 + close + 1, true
}

// SafeURL reports whether u is a URL the app opens: http, https or mailto.
// Anything else renders as plain text — the app has no router, so a
// scheme-less path handed to a link would go somewhere unpredictable.
func SafeURL(u string) bool {
	l := strings.ToLower(u)
	return strings.HasPrefix(l, "https://") || strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "mailto:")
}

// closing returns the index of the delimiter closing an emphasis in s.
func closing(s, delim string) int {
	for i := 1; i+len(delim) <= len(s); i++ {
		switch {
		case s[i] == '`':
			// Code spans take precedence.
			n := run(s[i:], '`')
			if end := strings.Index(s[i+n:], s[i:i+n]); end >= 0 {
				i += n + end + n - 1
			}
		case s[i] == '\\':
			i++
		case strings.HasPrefix(s[i:], delim) && s[i-1] != ' ' && s[i-1] != '\n':
			if len(delim) == 1 && i+1 < len(s) && s[i+1] == delim[0] {
				i++ // part of a strong delimiter
				continue
			}
			if delim[0] == '_' && i+len(delim) < len(s) && isWord(s[i+len(delim)]) {
				continue
			}
			return i
		}
	}
	return -1
}

func run(s string, c byte) int {
	n := 0
	for n < len(s) && s[n] == c {
		n++
	}
	return n
}

func isPunct(c byte) bool { return strings.IndexByte("!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~", c) >= 0 }

func isWord(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c >= 0x80
}

// wideAround reports whether the newline at s[i] sits between two characters
// of a script that writes without spaces between words. Such a break is not a
// word break, so it must not turn into a space. A break with Latin on either
// side keeps the Markdown reading, which is also what reads best at the seam
// of "总结\nsummary".
func wideAround(s string, i int) bool {
	before, ok := lastRune(s[:i])
	if !ok || !wide(before) {
		return false
	}
	after, ok := firstRune(s[i+1:])
	return ok && wide(after)
}

// wide reports whether r belongs to a script, or a punctuation block, that
// does not separate words with spaces.
func wide(r rune) bool {
	switch {
	case unicode.Is(unicode.Han, r),
		unicode.Is(unicode.Hiragana, r),
		unicode.Is(unicode.Katakana, r),
		unicode.Is(unicode.Hangul, r):
		return true
	}
	// CJK symbols and punctuation, and fullwidth forms.
	return r >= 0x3000 && r <= 0x303f || r >= 0xff00 && r <= 0xffef
}

func lastRune(s string) (rune, bool) {
	if s == "" {
		return 0, false
	}
	r, _ := utf8.DecodeLastRuneInString(s)
	return r, true
}

func firstRune(s string) (rune, bool) {
	if s == "" {
		return 0, false
	}
	r, _ := utf8.DecodeRuneInString(s)
	return r, true
}
