// linkcheck finds broken links in Markdown and HTML files, or on a website.
package main

import (
	"cmp"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the whole program apart from os.Exit, so tests can call it. It
// returns 0 when no link is broken, 1 when one is, and 2 when it can't run.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("linkcheck", flag.ContinueOnError)
	fs.SetOutput(stderr)
	workers := fs.Int("workers", 16, "number of links to check at the same time")
	timeout := fs.Duration("timeout", 10*time.Second, "timeout for each request")
	depth := fs.Int("depth", 1, "for a URL: how many levels of same-site pages to follow")
	asJSON := fs.Bool("json", false, "print all results as JSON")
	ignore := fs.String("ignore", "", "skip links matching this regular expression")
	retries := fs.Int("retries", 2, "extra attempts for network errors and 5xx responses")
	root := fs.String("root", "", "for files: the folder that links starting with / point into (default: skip those links)")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: linkcheck [flags] FILE|URL...")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err == flag.ErrHelp {
		return 0
	} else if err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		fs.Usage()
		return 2
	}

	var skip *regexp.Regexp
	if *ignore != "" {
		var err error
		if skip, err = regexp.Compile(*ignore); err != nil {
			fmt.Fprintln(stderr, "bad -ignore pattern:", err)
			return 2
		}
	}
	skipped := func(link string) bool { return skip != nil && skip.MatchString(link) }

	ctx := context.Background()
	checker := NewChecker(*workers, *timeout)
	checker.Retries = max(0, *retries)
	local := NewLocalChecker(*root)

	var links []Link
	var localResults []Result
	seen := map[string]bool{}
	add := func(l Link) {
		if !skipped(l.URL) && !seen[l.URL] {
			seen[l.URL] = true
			links = append(links, l)
		}
	}
	args, err := expandArgs(fs.Args())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	for _, arg := range args {
		if isURL(arg) {
			found, err := checker.Crawl(ctx, arg, *depth)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 2
			}
			for _, l := range found {
				add(l)
			}
			continue
		}
		data, err := os.ReadFile(arg)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		for _, f := range linksInFile(arg, string(data)) {
			if u, ok := httpURL(f.Target, nil); ok {
				add(Link{URL: u, Source: arg, Line: f.Line})
			} else if r, ok := local.Check(arg, f.Target); ok && !skipped(f.Target) {
				r.Source, r.Line = arg, f.Line
				localResults = append(localResults, r)
			}
		}
	}

	urls := make([]string, len(links))
	for i, l := range links {
		urls[i] = l.URL
	}
	results := checker.CheckAll(ctx, urls)
	for i := range results {
		results[i].Source, results[i].Line = links[i].Source, links[i].Line
	}
	results = append(results, localResults...)
	slices.SortStableFunc(results, func(a, b Result) int {
		return cmp.Or(cmp.Compare(a.Source, b.Source), cmp.Compare(a.Line, b.Line))
	})

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.Encode(results)
	}
	broken := 0
	for _, r := range results {
		if !r.Broken() {
			continue
		}
		broken++
		reason := fmt.Sprint(r.Status)
		if r.Err != "" {
			reason = r.Err
		}
		if !*asJSON {
			where := r.Source
			if r.Line > 0 {
				where = fmt.Sprintf("%s:%d", r.Source, r.Line)
			}
			fmt.Fprintf(stdout, "BROKEN  %s  (%s)  in %s\n", r.URL, reason, where)
		}
	}
	fmt.Fprintf(stderr, "%d links checked, %d broken\n", len(results), broken)
	if broken > 0 {
		return 1
	}
	return 0
}

func isURL(arg string) bool {
	return strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://")
}

// expandArgs replaces each folder with the Markdown and HTML files under it,
// and expands wildcards like docs/*.md, which PowerShell and cmd pass on as
// written. A file named twice is read once.
func expandArgs(args []string) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	add := func(path string) {
		if key := filepath.Clean(path); !seen[key] {
			seen[key] = true
			out = append(out, path)
		}
	}
	for _, arg := range args {
		if isURL(arg) {
			out = append(out, arg)
			continue
		}
		paths := []string{arg}
		if _, err := os.Stat(arg); err != nil && strings.ContainsAny(arg, "*?[") {
			matches, err := filepath.Glob(arg)
			if err != nil {
				return nil, fmt.Errorf("bad pattern %s: %v", arg, err)
			}
			if len(matches) == 0 {
				return nil, fmt.Errorf("no files match %s", arg)
			}
			paths = matches
		}
		for _, path := range paths {
			if info, err := os.Stat(path); err != nil || !info.IsDir() {
				add(path) // a missing file is reported when it is read
				continue
			}
			files, err := pagesIn(path)
			if err != nil {
				return nil, err
			}
			for _, f := range files {
				add(f)
			}
		}
	}
	return out, nil
}

// pagesIn lists the Markdown and HTML files under dir, skipping hidden
// folders like .git and installed packages in node_modules.
func pagesIn(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != dir && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if isMarkdown(path) || isHTML(path) {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// linksInFile finds the links in a file, reading it by its type.
func linksInFile(path, doc string) []Found {
	switch {
	case isHTML(path):
		return FindHTMLLinks(doc)
	case isMarkdown(path):
		return FindMarkdownLinks(hideCode(doc))
	}
	return FindLinks(doc)
}
