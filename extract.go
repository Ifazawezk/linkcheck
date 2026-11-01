package main

import (
	"net/url"
	"regexp"
	"slices"
	"strings"
)

var (
	// [text](url), [text](url "title") and [text](<url with spaces>) in Markdown.
	// A url can hold one level of parentheses, as in /wiki/Go_(language).
	mdLink = regexp.MustCompile(`\]\(\s*(?:<([^<>\n]+)>|((?:[^()\s]|\([^()\s]*\))+))(?:\s+(?:"[^"\n]*"|'[^'\n]*'|\([^()\n]*\)))?\s*\)`)
	// Bare <https://...> autolinks.
	mdAutolink = regexp.MustCompile(`<(https?://[^>\s]+)>`)
	// [label]: url reference definitions, but not [^1]: footnotes.
	mdReference = regexp.MustCompile(`(?m)^ {0,3}\[[^\]^\n][^\]\n]*\]:[ \t]*<?([^\s<>]+)>?(?:[ \t]+(?:"[^"\n]*"|'[^'\n]*'|\([^()\n]*\)))?[ \t\r]*$`)
	// href="..." and src="..." in HTML.
	htmlLink = regexp.MustCompile(`(?i)(?:href|src)\s*=\s*["']([^"']+)["']`)
	// Comments, and script and style tags with what is inside them.
	htmlHidden = regexp.MustCompile(`(?is)<!--.*?-->|(<(?:script|style)\b[^>]*>).*?</(?:script|style)\s*>`)
)

// Found is a link target as written in a document, and the line it is on.
type Found struct {
	Target string
	Line   int
}

// FindLinks returns every link target in a Markdown document, or in text of an
// unknown kind, in document order.
func FindLinks(doc string) []Found {
	return findLinks(doc, mdLink, mdAutolink, mdReference, htmlLink)
}

// FindHTMLLinks returns every link target in an HTML document. The Markdown
// patterns would read script code like a[b](c) as a link, and comments and
// scripts can hold markup that is not on the page.
func FindHTMLLinks(doc string) []Found {
	b := []byte(doc)
	for _, m := range htmlHidden.FindAllStringSubmatchIndex(doc, -1) {
		from := m[0]
		if m[2] >= 0 {
			from = m[3] // keep the opening tag, for <script src="...">
		}
		blank(b, from, m[1])
	}
	return findLinks(string(b), htmlLink)
}

func findLinks(doc string, patterns ...*regexp.Regexp) []Found {
	type hit struct {
		offset int
		target string
	}
	var hits []hit
	for _, re := range patterns {
		for _, m := range re.FindAllStringSubmatchIndex(doc, -1) {
			for g := 2; g+1 < len(m); g += 2 {
				if m[g] >= 0 {
					hits = append(hits, hit{m[g], doc[m[g]:m[g+1]]})
				}
			}
		}
	}
	slices.SortStableFunc(hits, func(a, b hit) int { return a.offset - b.offset })

	var out []Found
	line, pos := 1, 0
	for _, h := range hits {
		line += strings.Count(doc[pos:h.offset], "\n")
		pos = h.offset
		if t := strings.TrimSpace(h.target); t != "" {
			out = append(out, Found{t, line})
		}
	}
	return out
}

// ExtractLinks returns the unique http(s) links in a Markdown or HTML document,
// resolved against base.
func ExtractLinks(doc string, base *url.URL) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range FindLinks(doc) {
		if s, ok := httpURL(f.Target, base); ok && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// httpURL resolves a link target against base and returns it without its
// fragment. Anchors, mailto: and javascript: links are not http(s) URLs.
func httpURL(target string, base *url.URL) (string, bool) {
	if strings.HasPrefix(target, "#") {
		return "", false
	}
	u, err := url.Parse(target)
	if err != nil {
		return "", false
	}
	if base != nil {
		u = base.ResolveReference(u)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", false
	}
	u.Fragment = ""
	return u.String(), true
}
