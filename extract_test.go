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

func TestFindHTMLLinksSkipsWhatIsNotOnThePage(t *testing.T) {
	doc := `<script src="app.js"></script>
<SCRIPT type="module">
  var r = e[t](n);
  el.innerHTML = '<a href="from-script.html">x</a>';
</SCRIPT>
<style>a { background: url(x.png); }</style>
<!-- <a href="old.html">
old</a> -->
<a href="real.html">real</a>
[not](markdown.md)`
	want := []Found{{"app.js", 1}, {"real.html", 9}}
	if got := FindHTMLLinks(doc); !reflect.DeepEqual(got, want) {
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

func TestFindMarkdownLinksReadsBareURLs(t *testing.T) {
	doc := "See https://go.dev/doc.\n" +
		"(about https://w.org/wiki/Go_(language)) and (https://x.dev/a)\n" +
		"[https://a.dev](https://a.dev/x) and <https://b.dev>\n" +
		"[ref]: https://c.dev\n" +
		"**https://d.dev/path?q=1**, done, and https://e.dev/a_b_\n" +
		"<a href=\"https://f.dev\">f</a> x=https://not-a-link.dev\n"
	want := []Found{
		{"https://go.dev/doc", 1},
		{"https://w.org/wiki/Go_(language)", 2}, {"https://x.dev/a", 2},
		{"https://a.dev/x", 3}, {"https://b.dev", 3},
		{"https://c.dev", 4},
		{"https://d.dev/path?q=1", 5}, {"https://e.dev/a_b", 5},
		{"https://f.dev", 6},
	}
	if got := FindMarkdownLinks(doc); !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %v\nwant %v", got, want)
	}
}

func TestFindMarkdownLinksSkipsPlaceholderHosts(t *testing.T) {
	doc := "Start it and open http://localhost:3000 or http://127.0.0.1:8000/admin.\n" +
		"Set API_URL to https://api.example.com/v1 or https://my.app.test/.\n" +
		"[local](http://localhost:3000) is a real link, so it stays.\n"
	want := []Found{{"http://localhost:3000", 3}}
	if got := FindMarkdownLinks(doc); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
