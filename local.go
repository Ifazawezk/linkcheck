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
	Root    string
	dirs    map[string]map[string]bool // file names in each folder read so far
	anchors map[string]map[string]bool // anchors in each file read so far
}

func NewLocalChecker(root string) *LocalChecker {
	return &LocalChecker{Root: root, dirs: map[string]map[string]bool{}, anchors: map[string]map[string]bool{}}
}

// Check checks a link target found in the file source. It returns false if
// the target isn't something it can check on disk.
func (lc *LocalChecker) Check(source, target string) (Result, bool) {
	u, ok := localTarget(target)
	if !ok {
		return Result{}, false
	}
	file := source
	r := Result{URL: target}
	switch {
	case u.Path == "" && u.Fragment == "":
		return Result{}, false
	case u.Path != "":
		dir, rel := filepath.Dir(source), u.Path
		if strings.HasPrefix(rel, "/") {
			if lc.Root == "" {
				return Result{}, false
			}
			dir, rel = lc.Root, strings.TrimLeft(rel, "/")
		}
		if file, r.Err = lc.findFile(source, dir, rel); r.Err != "" {
			return r, true
		}
	}
	if !lc.hasAnchor(file, u.Fragment) && u.RawQuery == "" {
		r.Err = "no such heading or anchor"
	}
	return r, true
}

// findFile finds the file a link points at, and returns what is wrong with the
// link if there is a problem.
func (lc *LocalChecker) findFile(source, dir, rel string) (file, problem string) {
	path := filepath.Join(dir, filepath.FromSlash(rel))
	for _, ext := range append([]string{""}, pageExtensions(source)...) {
		if _, err := os.Stat(path + ext); err != nil {
			continue
		}
		if name := lc.wrongCase(dir, strings.TrimRight(rel, "/")+ext); name != "" {
			return "", fmt.Sprintf("wrong case, on disk it is %s", strings.TrimSuffix(name, ext))
		}
		return path + ext, ""
	}
	return "", "file not found"
}

// hasAnchor reports whether a link to fragment in file lands somewhere. Only
// Markdown and HTML have anchors to compare: script.py#L10 is left alone.
func (lc *LocalChecker) hasAnchor(file, fragment string) bool {
	if fragment == "" || !(isMarkdown(file) || isHTML(file)) {
		return true
	}
	// #/about and #!/about are routes for a page's scripts, not places in it.
	if strings.HasPrefix(fragment, "/") || strings.HasPrefix(fragment, "!") {
		return true
	}
	// GitHub's page anchors compare without regard to case, so this does too.
	fragment = strings.TrimPrefix(strings.ToLower(fragment), "user-content-")
	if fragment == "top" || (fragment == "readme" && strings.HasPrefix(strings.ToLower(filepath.Base(file)), "readme")) {
		return true
	}
	anchors, ok := lc.anchors[file]
	if !ok {
		data, err := os.ReadFile(file)
		if err != nil {
			return true // the file itself was checked already
		}
		if isMarkdown(file) {
			anchors = markdownAnchors(string(data))
		} else {
			anchors = htmlAnchors(string(data), map[string]bool{})
		}
		lc.anchors[file] = anchors
	}
	return anchors[fragment]
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
