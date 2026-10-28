// linkcheck finds broken links in Markdown and HTML files, or on a website.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

func main() {
	workers := flag.Int("workers", 16, "number of links to check at the same time")
	timeout := flag.Duration("timeout", 10*time.Second, "timeout for each request")
	depth := flag.Int("depth", 1, "for a URL: how many levels of same-site pages to follow")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: linkcheck [flags] FILE|URL...")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}

	ctx := context.Background()
	checker := NewChecker(*workers, *timeout)

	var links []Link
	seen := map[string]bool{}
	add := func(l Link) {
		if !seen[l.URL] {
			seen[l.URL] = true
			links = append(links, l)
		}
	}
	for _, arg := range flag.Args() {
		if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
			found, err := checker.Crawl(ctx, arg, *depth)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			for _, l := range found {
				add(l)
			}
			continue
		}
		data, err := os.ReadFile(arg)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		for _, u := range ExtractLinks(string(data), nil) {
			add(Link{URL: u, Source: arg})
		}
	}

	urls := make([]string, len(links))
	for i, l := range links {
		urls[i] = l.URL
	}
	results := checker.CheckAll(ctx, urls)
	broken := 0
	for i, r := range results {
		if !r.Broken() {
			continue
		}
		broken++
		reason := fmt.Sprint(r.Status)
		if r.Err != "" {
			reason = r.Err
		}
		fmt.Printf("BROKEN  %s  (%s)  in %s\n", r.URL, reason, links[i].Source)
	}
	fmt.Printf("%d links checked, %d broken\n", len(results), broken)
	if broken > 0 {
		os.Exit(1)
	}
}
