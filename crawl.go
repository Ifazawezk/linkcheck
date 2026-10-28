package main

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Link is a URL together with the page or file it was found on.
type Link struct {
	URL    string
	Source string
}

// Crawl starts at start and follows links on the same host up to depth levels,
// returning every link found on the pages it visited (including external ones).
func (c *Checker) Crawl(ctx context.Context, start string, depth int) ([]Link, error) {
	root, err := url.Parse(start)
	if err != nil {
		return nil, err
	}

	visited := map[string]bool{start: true}
	found := map[string]bool{}
	var links []Link
	level := []string{start}

	for d := 0; d <= depth && len(level) > 0; d++ {
		var next []string
		for _, page := range level {
			body, base, ok := c.fetchHTML(ctx, page)
			if !ok {
				continue
			}
			for _, l := range ExtractLinks(body, base) {
				if !found[l] {
					found[l] = true
					links = append(links, Link{URL: l, Source: page})
				}
				u, _ := url.Parse(l)
				if u.Host == root.Host && !visited[l] && d < depth {
					visited[l] = true
					next = append(next, l)
				}
			}
		}
		level = next
	}
	return links, nil
}

// fetchHTML downloads a page and returns its body if it is HTML.
func (c *Checker) fetchHTML(ctx context.Context, page string) (string, *url.URL, bool) {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, page, nil)
	if err != nil {
		return "", nil, false
	}
	req.Header.Set("User-Agent", "linkcheck/1.0 (+https://github.com/Ifazawezk/linkcheck)")
	resp, err := c.Client.Do(req)
	if err != nil {
		return "", nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 || !strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
		return "", nil, false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return "", nil, false
	}
	return string(body), resp.Request.URL, true
}
