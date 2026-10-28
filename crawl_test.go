package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
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
