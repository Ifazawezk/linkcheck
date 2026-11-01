package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// writeFiles creates files under a new temporary folder and returns its path.
func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestLocalCheck(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"README.md":          "",
		"docs/setup.md":      "",
		"docs/my file.md":    "",
		"docs/مقدمة.md":      "",
		"docs/guide/deep.md": "",
		"img/logo.png":       "",
	})
	source := filepath.Join(dir, "docs", "setup.md")
	tests := []struct {
		target string
		broken bool
	}{
		{"guide/deep.md", false},
		{"./guide/deep.md", false},
		{"../README.md", false},
		{"../img/logo.png", false},
		{"guide/", false},
		{"guide", false},
		{"my%20file.md", false},
		{"my file.md", false},
		{"مقدمة.md", false},
		{"%D9%85%D9%82%D8%AF%D9%85%D8%A9.md", false}, // the same name as a browser writes it
		{"%D9%85%D9%82%D8%AF%D9%85%D8%A9-gone.md", true},
		{"../README.md?plain=1", false},
		{"gone.md", true},
		{"../docs/gone.md", true},
		{"guide/gone/", true},
		{"guide/deep.md/", false}, // a slash after a file is cleaned away
	}
	lc := NewLocalChecker("")
	for _, tt := range tests {
		r, ok := lc.Check(source, tt.target)
		if !ok {
			t.Errorf("%s: should be checked", tt.target)
			continue
		}
		if r.URL != tt.target {
			t.Errorf("%s: result is for %q, want the link as written", tt.target, r.URL)
		}
		if r.Broken() != tt.broken {
			t.Errorf("%s: broken=%v (%s), want %v", tt.target, r.Broken(), r.Err, tt.broken)
		}
	}
}

func TestLocalCheckSkipsWhatIsNotAFile(t *testing.T) {
	lc := NewLocalChecker("")
	targets := []string{
		"https://go.dev", "mailto:a@b.c", "tel:+97412345678", "javascript:void(0)", "data:image/png;base64,AA==",
		"//cdn.example.com/a.js", "?page=2", "{{ .Site.BaseURL }}/a", "{% url 'home' %}", "${CDN}/a.js", "<%= path %>",
		"%BASE_URL%favicon.svg", "%PUBLIC_URL%/logo.png",
		"/absolute/path.md",
	}
	for _, target := range targets {
		if r, ok := lc.Check("README.md", target); ok {
			t.Errorf("%s: should be skipped, got %+v", target, r)
		}
	}
}

func TestLocalCheckRoot(t *testing.T) {
	dir := writeFiles(t, map[string]string{"public/index.html": "", "public/css/site.css": ""})
	lc := NewLocalChecker(filepath.Join(dir, "public"))
	source := filepath.Join(dir, "public", "index.html")
	if r, ok := lc.Check(source, "/css/site.css"); !ok || r.Broken() {
		t.Errorf("/css/site.css: got %+v, %v", r, ok)
	}
	if r, ok := lc.Check(source, "/css/gone.css"); !ok || !r.Broken() {
		t.Errorf("/css/gone.css: got %+v, %v", r, ok)
	}
}

func TestLocalCheckPagesLinkedWithoutAnExtension(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"index.html": "", "about.html": "", "README.md": "", "notes.txt": "",
		"guide/health.md": "", "guide/index.mdx": "",
	})
	html, md, text := filepath.Join(dir, "index.html"), filepath.Join(dir, "README.md"), filepath.Join(dir, "notes.txt")
	tests := []struct {
		source, target string
		broken         bool
	}{
		{html, "about", false},
		{md, "guide/health", false},
		{md, "./guide/index", false},
		{md, "guide/HEALTH", true}, // the case still has to be right
		{md, "about", true},        // Markdown pages don't link to built HTML names
		{md, "guide/gone", true},
		{html, "guide/health", true},
		{text, "guide/health", true},
	}
	lc := NewLocalChecker("")
	for _, tt := range tests {
		if r, _ := lc.Check(tt.source, tt.target); r.Broken() != tt.broken {
			t.Errorf("%s in %s: broken=%v (%s), want %v", tt.target, filepath.Base(tt.source), r.Broken(), r.Err, tt.broken)
		}
	}
}

func TestLocalCheckAnchors(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"README.md":      "# Demo\n\n## Getting started\n\n## مقدمة\n",
		"docs/setup.md":  "# Setup\n\n## Install\n\n```\n# comment\n```\n",
		"docs/guide.mdx": "## Deep part\n",
		"page.html":      `<h2 id="intro">Intro</h2><a name="old"></a>`,
		"tool.py":        "print('hi')\n",
	})
	readme := filepath.Join(dir, "README.md")
	tests := []struct {
		target string
		broken bool
	}{
		{"#getting-started", false},
		{"#Getting-Started", false},
		{"#getting-startd", true},
		{"#%D9%85%D9%82%D8%AF%D9%85%D8%A9", false}, // مقدمة as a browser writes it
		{"#مقدمة", false},
		{"#user-content-getting-started", false},
		{"#top", false},
		{"#readme", false},
		{"docs/setup.md#install", false},
		{"docs/setup.md#instal", true},
		{"docs/setup.md#comment", true},
		{"docs/setup.md#readme", true},
		{"docs/setup#install", false},
		{"docs/guide#deep-part", false},
		{"docs/guide#gone", true},
		{"page.html#intro", false},
		{"page.html#old", false},
		{"page.html#gone", true},
		{"tool.py#L10", false},
		{"docs/setup.md?plain=1#L3", false},
		{"#/about", false},
		{"#!/about", false},
	}
	lc := NewLocalChecker("")
	for _, tt := range tests {
		r, ok := lc.Check(readme, tt.target)
		if !ok {
			t.Errorf("%s: should be checked", tt.target)
			continue
		}
		if r.Broken() != tt.broken {
			t.Errorf("%s: broken=%v (%s), want %v", tt.target, r.Broken(), r.Err, tt.broken)
		}
	}
	if r, ok := lc.Check(readme, "#"); ok {
		t.Errorf("an empty anchor is not a link to check: %+v", r)
	}
}

func TestLocalCheckWrongCase(t *testing.T) {
	dir := writeFiles(t, map[string]string{"docs/setup.md": "", "README.md": ""})
	source := filepath.Join(dir, "README.md")
	lc := NewLocalChecker("")
	for _, target := range []string{"docs/SETUP.md", "Docs/setup.md", "docs/../DOCS/setup.md"} {
		r, ok := lc.Check(source, target)
		if !ok || !r.Broken() {
			t.Errorf("%s: should be broken, got %+v", target, r)
			continue
		}
		// A file system that respects case can't find the file at all.
		if runtime.GOOS == "windows" && !strings.Contains(strings.ToLower(r.Err), "wrong case") {
			t.Errorf("%s: got %q, want a message about case", target, r.Err)
		}
	}
	if r, _ := lc.Check(source, "docs/setup.md"); r.Broken() {
		t.Errorf("exact name should pass: %+v", r)
	}
}
