package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Link is a URL together with the page or file it was found on, and for a
// file the line.
type Link struct {
	URL    string
	Source string
	Line   int
}

// Crawl starts at start and follows links on the same host up to depth levels,
// returning every link found on the pages it visited (including external ones).
// It is an error if the start page can't be read, because nothing would be
// checked and a site that is down would look like one with no broken links.
// Other pages that can't be read are not errors here: they are links too, so
// they get reported when the links are checked.
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
			body, base, err := c.fetchHTML(ctx, page)
			if err != nil {
				if d == 0 {
					return nil, fmt.Errorf("can't crawl %s: %w", start, err)
				}
				continue
			}
			if d == 0 {
				// The site may redirect, say from example.com to www.example.com.
				// The rest of it lives on the host we ended up on.
				root = base
				visited[base.String()] = true
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

// fetchHTML downloads a page, with the same retries as Check, and returns its
// body and the URL it ended up at. It is an error if the page isn't HTML.
func (c *Checker) fetchHTML(ctx context.Context, page string) (string, *url.URL, error) {
	var body, kind string
	var base *url.URL
	status, err := c.retry(ctx, func() (int, error) {
		ctx, cancel := context.WithTimeout(ctx, c.Timeout)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, page, nil)
		if err != nil {
			return 0, err
		}
		req.Header.Set("User-Agent", "linkcheck/1.0 (+https://github.com/Ifazawezk/linkcheck)")
		resp, err := c.Client.Do(req)
		if err != nil {
			return 0, err
		}
		defer resp.Body.Close()
		kind, base = resp.Header.Get("Content-Type"), resp.Request.URL
		if resp.StatusCode >= 400 || !strings.Contains(kind, "text/html") {
			return resp.StatusCode, nil
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
		body = string(data)
		return resp.StatusCode, err
	})
	switch {
	case err != nil:
		return "", nil, err
	case status >= 400:
		return "", nil, fmt.Errorf("status %d", status)
	case !strings.Contains(kind, "text/html"):
		return "", nil, fmt.Errorf("not an HTML page (%s)", kind)
	}
	return body, base, nil
}
