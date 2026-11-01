package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// templateCode marks a link that a template or build tool fills in later. A
// %NAME% needs three letters or more, so that %AF% in a percent-encoded path,
// where %AF is one byte of a real file name, doesn't count.
var templateCode = regexp.MustCompile(`\{\{|\{%|<%|\$\{|\$\(|%[A-Z][A-Z_]{2,}%`)

// LocalChecker checks links from one file to another on disk.
type LocalChecker struct {
	// Root is the folder that links starting with / point into. Those links
	// can't be followed without it, so they are skipped when it is empty.
	Root string
	dirs map[string]map[string]bool // file names in each folder read so far
}

func NewLocalChecker(root string) *LocalChecker {
	return &LocalChecker{Root: root, dirs: map[string]map[string]bool{}}
}

// Check checks a link target found in the file source. It returns false if
// the target isn't something it can check on disk.
func (lc *LocalChecker) Check(source, target string) (Result, bool) {
	u, ok := localTarget(target)
	if !ok || u.Path == "" {
		return Result{}, false
	}
	dir, rel := filepath.Dir(source), u.Path
	if strings.HasPrefix(rel, "/") {
		if lc.Root == "" {
			return Result{}, false
		}
		dir, rel = lc.Root, strings.TrimLeft(rel, "/")
	}
	r := Result{URL: target}
	path := filepath.Join(dir, filepath.FromSlash(rel))
	if _, err := os.Stat(path); err == nil {
		if name := lc.wrongCase(dir, rel); name != "" {
			r.Err = fmt.Sprintf("wrong case, on disk it is %s", name)
		}
		return r, true
	}
	// Static sites and docs sites link to a page by its name alone.
	for _, ext := range pageExtensions(source) {
		if _, err := os.Stat(path + ext); err == nil {
			if name := lc.wrongCase(dir, rel+ext); name != "" {
				r.Err = fmt.Sprintf("wrong case, on disk it is %s", strings.TrimSuffix(name, ext))
			}
			return r, true
		}
	}
	r.Err = "file not found"
	return r, true
}

// pageExtensions lists the extensions a link in source can leave out. VitePress,
// Docusaurus and similar tools turn [a](./guide/setup) into guide/setup.md.
func pageExtensions(source string) []string {
	switch {
	case isHTML(source):
		return []string{".html"}
	case isMarkdown(source):
		return []string{".md", ".mdx", ".markdown"}
	}
	return nil
}

// localTarget parses a link target that points at a file, rather than at a
// website, a mail address or something a template fills in later.
func localTarget(target string) (*url.URL, bool) {
	if templateCode.MatchString(target) {
		return nil, false
	}
	u, err := url.Parse(target)
	if err != nil || u.Scheme != "" || u.Host != "" {
		return nil, false
	}
	return u, true
}

// wrongCase compares each part of rel with the names on disk and returns the
// first real name that differs only in case. Such a link opens on Windows and
// macOS, but it is broken on GitHub and on most web servers.
func (lc *LocalChecker) wrongCase(dir, rel string) string {
	for _, part := range strings.Split(rel, "/") {
		if part == "" || part == "." {
			continue
		}
		if part != ".." {
			names := lc.names(dir)
			if !names[part] {
				for name := range names {
					if strings.EqualFold(name, part) {
						return name
					}
				}
			}
		}
		dir = filepath.Join(dir, part)
	}
	return ""
}

// names returns the file names in dir, reading each folder only once.
func (lc *LocalChecker) names(dir string) map[string]bool {
	if names, ok := lc.dirs[dir]; ok {
		return names
	}
	names := map[string]bool{}
	entries, _ := os.ReadDir(dir) // a folder that can't be listed has no names to compare
	for _, e := range entries {
		names[e.Name()] = true
	}
	lc.dirs[dir] = names
	return names
}

// isHTML reports whether path is an HTML file.
func isHTML(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".html", ".htm", ".xhtml":
		return true
	}
	return false
}
