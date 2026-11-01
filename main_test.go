package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunFindsBrokenRelativeLinks(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"README.md":     "# Demo\n\n[ok](docs/setup.md)\n\n[gone](docs/gone.md)\n\n```\n[sample](docs/sample.md)\n```\n",
		"docs/setup.md": "[back](../README.md)\n[gone too](../gone.md)\n",
	})
	readme, setup := filepath.Join(dir, "README.md"), filepath.Join(dir, "docs", "setup.md")

	var stdout, stderr bytes.Buffer
	code := run([]string{readme, setup}, &stdout, &stderr)
	want := fmt.Sprintf("BROKEN  docs/gone.md  (file not found)  in %s:5\nBROKEN  ../gone.md  (file not found)  in %s:2\n", readme, setup)
	if code != 1 || stdout.String() != want {
		t.Fatalf("exit %d, stdout:\n%s\nwant:\n%s", code, stdout.String(), want)
	}
	if !strings.Contains(stderr.String(), "4 links checked, 2 broken") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestRunReportsEveryPlaceALocalLinkBreaks(t *testing.T) {
	dir := writeFiles(t, map[string]string{"a.md": "[x](gone.md)\n[y](gone.md)\n"})
	var stdout, stderr bytes.Buffer
	if code := run([]string{filepath.Join(dir, "a.md")}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if n := strings.Count(stdout.String(), "BROKEN"); n != 2 {
		t.Errorf("got %d reports, want one for each line:\n%s", n, stdout.String())
	}
}

func TestRunReadsHTMLFilesAsHTML(t *testing.T) {
	dir := writeFiles(t, map[string]string{
		"page.html": `<script>var r = e[t](n);</script><a href="gone.html">x</a>`,
	})
	var stdout, stderr bytes.Buffer
	if code := run([]string{filepath.Join(dir, "page.html")}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if n := strings.Count(stdout.String(), "BROKEN"); n != 1 || !strings.Contains(stdout.String(), "gone.html") {
		t.Errorf("want only gone.html reported, got:\n%s", stdout.String())
	}
}

func TestRunIgnoreAppliesToLocalLinks(t *testing.T) {
	dir := writeFiles(t, map[string]string{"a.md": "[x](drafts/gone.md)\n"})
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-ignore", "^drafts/", filepath.Join(dir, "a.md")}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d, stdout: %s", code, stdout.String())
	}
}

func TestRunRootChecksAbsoluteLinks(t *testing.T) {
	dir := writeFiles(t, map[string]string{"public/index.html": `<link href="/css/gone.css">`})
	page := filepath.Join(dir, "public", "index.html")
	var stdout, stderr bytes.Buffer
	if code := run([]string{page}, &stdout, &stderr); code != 0 {
		t.Errorf("without -root the link is skipped, got exit %d", code)
	}
	if code := run([]string{"-root", filepath.Join(dir, "public"), page}, &stdout, &stderr); code != 1 {
		t.Errorf("with -root the link is checked, got exit %d", code)
	}
}

func TestRunJSONHasLineNumbers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	dir := writeFiles(t, map[string]string{"a.md": "first\n[web](" + srv.URL + "/x)\n[file](gone.md)\n"})

	var stdout, stderr bytes.Buffer
	if code := run([]string{"-json", filepath.Join(dir, "a.md")}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	var got []Result
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, stdout.String())
	}
	if len(got) != 2 || got[0].Line != 2 || got[0].Status != 404 || got[1].Line != 3 || got[1].URL != "gone.md" {
		t.Fatalf("got %+v", got)
	}
}

func TestRunUsageErrors(t *testing.T) {
	for _, args := range [][]string{nil, {"-nope"}, {"-ignore", "(", "a.md"}, {"does-not-exist.md"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
	}
}
