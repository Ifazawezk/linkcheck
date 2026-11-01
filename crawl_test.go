package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCrawlStaysOnHostAndRespectsDepth(t *testing.T) {
	mux := http.NewServeMux()
	page := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, body)
		}
	}
	mux.HandleFunc("/", page(`<a href="/a">a</a> <a href="https://external.example/x">ext</a>`))
	mux.HandleFunc("/a", page(`<a href="/b">b</a>`))
	mux.HandleFunc("/b", page(`<a href="/c">c</a>`))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := NewChecker(4, time.Second)
	links, err := c.Crawl(context.Background(), srv.URL+"/", 1)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, l := range links {
		got = append(got, l.URL)
	}
	sort.Strings(got)
	want := []string{"https://external.example/x", srv.URL + "/a", srv.URL + "/b"}
	sort.Strings(want)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for _, l := range links {
		if l.URL == srv.URL+"/b" && l.Source != srv.URL+"/a" {
			t.Errorf("/b source = %s, want /a", l.Source)
		}
	}
}

func TestCrawlFailsWhenTheStartPageCanNotBeRead(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/down", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	mux.HandleFunc("/plain", func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Content-Type", "text/plain") })
	srv := httptest.NewServer(mux)
	defer srv.Close()
	closed := httptest.NewServer(mux)
	closed.Close()

	tests := []struct{ url, want string }{
		{srv.URL + "/down", "status 500"},
		{srv.URL + "/missing", "status 404"},
		{srv.URL + "/plain", "not an HTML page (text/plain)"},
		{closed.URL + "/", "can't crawl"},
	}
	c := NewChecker(4, time.Second)
	c.RetryWait = time.Millisecond
	for _, tt := range tests {
		links, err := c.Crawl(context.Background(), tt.url, 1)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: got links %v, error %v, want an error with %q", tt.url, links, err, tt.want)
		}
	}
}

func TestCrawlRetriesTheStartPage(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<a href="/a">a</a>`)
	}))
	defer srv.Close()

	c := NewChecker(4, time.Second)
	c.RetryWait = time.Millisecond
	links, err := c.Crawl(context.Background(), srv.URL+"/", 0)
	if err != nil || len(links) != 1 {
		t.Fatalf("got links %v, error %v, want one link after a retry", links, err)
	}
}

func TestCrawlDoesNotRetryAMissingPage(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewChecker(4, time.Second)
	c.RetryWait = time.Millisecond
	if _, err := c.Crawl(context.Background(), srv.URL+"/", 0); err == nil {
		t.Fatal("expected an error")
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("got %d requests, want 1", n)
	}
}

func TestCrawlFollowsTheSiteAfterARedirect(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<a href="/inner">inner</a>`)
	})
	mux.HandleFunc("/inner", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<a href="/deep">deep</a>`)
	})
	site := httptest.NewServer(mux)
	defer site.Close()
	old := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, site.URL+"/", http.StatusMovedPermanently)
	}))
	defer old.Close()

	c := NewChecker(4, time.Second)
	links, err := c.Crawl(context.Background(), old.URL+"/", 2)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, l := range links {
		got = append(got, l.URL)
	}
	if want := []string{site.URL + "/inner", site.URL + "/deep"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
