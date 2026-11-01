package main

import (
	"fmt"
	"html"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var (
	htmlComment = regexp.MustCompile(`(?s)<!--.*?-->`)
	blankLine   = regexp.MustCompile(`\n[ \t\r]*\n`)
)

var (
	atxHeading    = regexp.MustCompile(`^ {0,3}#{1,6}[ \t]+(.+?)(?:[ \t]+#+)?[ \t\r]*$`)
	setextLine    = regexp.MustCompile(`^ {0,3}(?:=+|-+)[ \t\r]*$`)
	customID      = regexp.MustCompile(`[ \t]*\{#([^\s}]+)\}$`)
	headingLink   = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)
	headingTag    = regexp.MustCompile(`<[^>]*>`)
	emphasisMarks = regexp.MustCompile(`(^|\s)_+|_+(\s|$)`)
	idAttribute   = regexp.MustCompile(`(?i)\s(?:id|name)\s*=\s*["']([^"']+)["']`)
)

// markdownAnchors returns the anchors a Markdown document has, in lower case:
// one for each heading, plus any set by hand with {#id} or an HTML id or name.
// Headings are only looked for outside code blocks, where # is often a comment.
func markdownAnchors(doc string) map[string]bool {
	doc = blankFences(doc)
	anchors := map[string]bool{}
	lines := strings.Split(doc, "\n")
	for i, line := range lines {
		text := ""
		if m := atxHeading.FindStringSubmatch(line); m != nil {
			text = m[1]
		} else if i+1 < len(lines) && setextLine.MatchString(lines[i+1]) {
			text = strings.TrimSpace(line)
		}
		if text == "" {
			continue
		}
		if m := customID.FindStringSubmatch(text); m != nil {
			anchors[strings.ToLower(m[1])] = true
			continue
		}
		// Repeated headings get -1, -2 and so on, the way GitHub numbers them.
		base := headingSlug(text)
		slug := base
		for n := 1; anchors[slug]; n++ {
			slug = fmt.Sprintf("%s-%d", base, n)
		}
		anchors[slug] = true
	}
	return htmlAnchors(doc, anchors)
}

// htmlAnchors adds the id and name attributes in doc to anchors.
func htmlAnchors(doc string, anchors map[string]bool) map[string]bool {
	for _, m := range idAttribute.FindAllStringSubmatch(doc, -1) {
		anchors[strings.ToLower(m[1])] = true
	}
	return anchors
}

// headingSlug turns a heading into its anchor the way GitHub does: formatting
// goes, spaces become hyphens, and letters and numbers in any script stay.
func headingSlug(text string) string {
	text = headingLink.ReplaceAllString(text, "$1")
	text = headingTag.ReplaceAllString(text, "")
	text = emphasisMarks.ReplaceAllString(text, "$1$2")
	text = html.UnescapeString(text)
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(text)) {
		switch {
		case r == ' ' || r == '-':
			b.WriteRune('-')
		case unicode.IsLetter(r), unicode.IsMark(r), unicode.IsNumber(r), unicode.Is(unicode.Pc, r):
			b.WriteRune(r)
		}
	}
	return b.String()
}

// isMarkdown reports whether path is a Markdown file.
func isMarkdown(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown", ".mdown", ".mkd", ".mdx":
		return true
	}
	return false
}

// hideCode blanks out the parts of a Markdown document that are not rendered
// as links: code blocks, inline code and HTML comments. The text in them is
// usually sample code, so its links are not real. Line breaks stay in place,
// so anything found afterwards is still on the right line.
func hideCode(doc string) string {
	doc = blankInlineCode(blankFences(doc))
	return htmlComment.ReplaceAllStringFunc(doc, func(s string) string {
		b := []byte(s)
		blank(b, 0, len(b))
		return string(b)
	})
}

// blank overwrites b[from:to] with spaces, apart from line breaks.
func blank(b []byte, from, to int) {
	for i := from; i < to; i++ {
		if b[i] != '\n' && b[i] != '\r' {
			b[i] = ' '
		}
	}
}

// blankFences blanks out ``` and ~~~ code blocks. A block with no closing fence
// runs to the end of the document, as it does when rendered.
func blankFences(doc string) string {
	b := []byte(doc)
	open := ""
	offset := 0
	for _, line := range strings.SplitAfter(doc, "\n") {
		text := strings.TrimLeft(line, " \t")
		marker := fenceMarker(text)
		switch {
		case open != "":
			blank(b, offset, offset+len(line))
			closes := marker != "" && marker[0] == open[0] && len(marker) >= len(open) &&
				strings.TrimSpace(text[len(marker):]) == ""
			if closes {
				open = ""
			}
		case marker != "":
			open = marker
			blank(b, offset, offset+len(line))
		}
		offset += len(line)
	}
	return string(b)
}

// fenceMarker returns the run of backticks or tildes that line starts with, if
// it is long enough to be a code fence.
func fenceMarker(line string) string {
	if line == "" || (line[0] != '`' && line[0] != '~') {
		return ""
	}
	n := len(line) - len(strings.TrimLeft(line, line[:1]))
	if n < 3 || (line[0] == '`' && strings.Contains(line[n:], "`")) {
		return "" // "``` x ```" on one line is inline code
	}
	return line[:n]
}

// blankInlineCode blanks out `inline code`. A span closes with a run of the same
// number of backticks in the same paragraph; a lone backtick is plain text.
func blankInlineCode(doc string) string {
	b := []byte(doc)
	for i := 0; i < len(b); {
		if b[i] != '`' {
			i++
			continue
		}
		n := backticks(b, i)
		end := closingBackticks(doc, b, i+n, n)
		if end < 0 {
			i += n
			continue
		}
		blank(b, i, end+n)
		i = end + n
	}
	return string(b)
}

func backticks(b []byte, i int) int {
	n := 0
	for i+n < len(b) && b[i+n] == '`' {
		n++
	}
	return n
}

// closingBackticks finds a run of exactly n backticks in the paragraph that
// continues from position from. It returns -1 if there is none.
func closingBackticks(doc string, b []byte, from, n int) int {
	limit := len(b)
	if loc := blankLine.FindStringIndex(doc[from:]); loc != nil {
		limit = from + loc[0]
	}
	for j := from; j < limit; {
		if b[j] != '`' {
			j++
			continue
		}
		run := backticks(b, j)
		if run == n {
			return j
		}
		j += run
	}
	return -1
}
