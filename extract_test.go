package main

import (
	"net/url"
	"reflect"
	"testing"
)

func TestExtractMarkdown(t *testing.T) {
	doc := `See [the docs](https://example.com/docs "Docs") and <https://go.dev>.
Jump to [a section](#setup), mail [me](mailto:a@b.c), or [again](https://example.com/docs#intro).`
	got := ExtractLinks(doc, nil)
	want := []string{"https://example.com/docs", "https://go.dev"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestExtractHTMLResolvesRelative(t *testing.T) {
	base, _ := url.Parse("https://site.com/blog/post.html")
	doc := `<a href="/about">About</a> <img src='img/logo.png'> <a href="javascript:void(0)">x</a>`
	got := ExtractLinks(doc, base)
	want := []string{"https://site.com/about", "https://site.com/blog/img/logo.png"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
