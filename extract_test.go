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

func TestExtractMarkdownForms(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want []string
	}{
		{"parentheses in the url", "[Go](https://w.org/wiki/Go_(language))", []string{"https://w.org/wiki/Go_(language)"}},
		{"parentheses and a title", `[Go](https://w.org/Go_(a) "Go") next`, []string{"https://w.org/Go_(a)"}},
		{"sentence in parentheses", "(see [Go](https://go.dev))", []string{"https://go.dev"}},
		{"two links on a line", "[a](https://a.dev) and [b](https://b.dev)", []string{"https://a.dev", "https://b.dev"}},
		{"single quoted title", "[a](https://a.dev 'title')", []string{"https://a.dev"}},
		{"title in parentheses", "[a](https://a.dev (title))", []string{"https://a.dev"}},
		{"angle brackets with a space", "[a](<https://a.dev/x y>)", []string{"https://a.dev/x%20y"}},
		{"image", "![logo](https://a.dev/logo.png)", []string{"https://a.dev/logo.png"}},
		{"reference definition", "[a][1]\n\n[1]: https://a.dev\n", []string{"https://a.dev"}},
		{"reference with title", "  [Guide]: <https://a.dev> \"The guide\"\r\n", []string{"https://a.dev"}},
		{"footnote is not a reference", "[^1]: https://a.dev is good\n", nil},
		{"prose is not a reference", "[Note]: https://a.dev is good\n", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractLinks(tt.doc, nil); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindLinksKeepsOrderAndLines(t *testing.T) {
	doc := "# Title\n\n<https://b.dev>\n<a href=\"c.html\">c</a>\n[a](a.md)\r\n\r\n[r]: r.md\n"
	want := []Found{{"https://b.dev", 3}, {"c.html", 4}, {"a.md", 5}, {"r.md", 7}}
	if got := FindLinks(doc); !reflect.DeepEqual(got, want) {
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
