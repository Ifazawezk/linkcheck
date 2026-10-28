package main

import (
	"net/url"
	"regexp"
	"strings"
)

var (
	// [text](url) and [text](url "title") in Markdown, and bare <https://...> autolinks.
	mdLink     = regexp.MustCompile(`\]\(\s*<?([^)\s>]+)>?(?:\s+"[^"]*")?\s*\)`)
	mdAutolink = regexp.MustCompile(`<(https?://[^>\s]+)>`)
	// href="..." and src="..." in HTML.
	htmlLink = regexp.MustCompile(`(?i)(?:href|src)\s*=\s*["']([^"']+)["']`)
)

// ExtractLinks returns the unique http(s) links in a Markdown or HTML document,
// resolved against base. Anchors, mailto: and javascript: links are skipped.
func ExtractLinks(doc string, base *url.URL) []string {
	var raw []string
	for _, re := range []*regexp.Regexp{mdLink, mdAutolink, htmlLink} {
		for _, m := range re.FindAllStringSubmatch(doc, -1) {
			raw = append(raw, m[1])
		}
	}

	seen := map[string]bool{}
	var out []string
	for _, r := range raw {
		r = strings.TrimSpace(r)
		if r == "" || strings.HasPrefix(r, "#") {
			continue
		}
		u, err := url.Parse(r)
		if err != nil {
			continue
		}
		if base != nil {
			u = base.ResolveReference(u)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			continue
		}
		u.Fragment = ""
		s := u.String()
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
