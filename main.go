// linkcheck finds broken links in Markdown and HTML files.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	workers := flag.Int("workers", 16, "number of links to check at the same time")
	timeout := flag.Duration("timeout", 10*time.Second, "timeout for each request")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: linkcheck [flags] FILE...")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}

	var urls []string
	for _, path := range flag.Args() {
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		urls = append(urls, ExtractLinks(string(data), nil)...)
	}

	results := NewChecker(*workers, *timeout).CheckAll(context.Background(), urls)
	broken := 0
	for _, r := range results {
		if r.Broken() {
			broken++
			if r.Err != "" {
				fmt.Printf("BROKEN  %s  (%s)\n", r.URL, r.Err)
			} else {
				fmt.Printf("BROKEN  %s  (%d)\n", r.URL, r.Status)
			}
		}
	}
	fmt.Printf("%d links checked, %d broken\n", len(results), broken)
	if broken > 0 {
		os.Exit(1)
	}
}
