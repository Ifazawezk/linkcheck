package main

import (
	"path/filepath"
	"regexp"
	"strings"
)

var (
	htmlComment = regexp.MustCompile(`(?s)<!--.*?-->`)
	blankLine   = regexp.MustCompile(`\n[ \t\r]*\n`)
)

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
