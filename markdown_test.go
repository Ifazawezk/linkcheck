package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestHideCodeSkipsSampleLinks(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want []string
	}{
		{"backtick fence", "```html\n<a href=\"https://x.dev\">x</a>\n```\n[y](https://y.dev)", []string{"https://y.dev"}},
		{"tilde fence", "~~~\n[x](https://x.dev)\n~~~\n[y](https://y.dev)", []string{"https://y.dev"}},
		{"indented fence in a list", "1. step\n\n    ```\n    [x](https://x.dev)\n    ```\n[y](https://y.dev)", []string{"https://y.dev"}},
		{"longer fence holds a shorter one", "````\n```\n[x](https://x.dev)\n```\n[w](https://w.dev)\n````\n[y](https://y.dev)", []string{"https://y.dev"}},
		{"backticks do not close tildes", "~~~\n```\n[x](https://x.dev)\n~~~\n[y](https://y.dev)", []string{"https://y.dev"}},
		{"closing fence has no text after it", "```\n``` not closed\n[x](https://x.dev)\n```\n[y](https://y.dev)", []string{"https://y.dev"}},
		{"unclosed fence runs to the end", "[y](https://y.dev)\n```\n[x](https://x.dev)", []string{"https://y.dev"}},
		{"windows line endings", "```\r\n[x](https://x.dev)\r\n```\r\n[y](https://y.dev)\r\n", []string{"https://y.dev"}},
		{"inline code", "Use `[x](https://x.dev)` or [y](https://y.dev)", []string{"https://y.dev"}},
		{"double backticks", "Use ``a ` [x](https://x.dev)`` or [y](https://y.dev)", []string{"https://y.dev"}},
		{"code on one line with three backticks", "``` [x](https://x.dev) ``` [y](https://y.dev)", []string{"https://y.dev"}},
		{"lone backtick is plain text", "a ` b\n\n[y](https://y.dev) and ` c", []string{"https://y.dev"}},
		{"html comment", "<!-- [x](https://x.dev)\n[w](https://w.dev) -->\n[y](https://y.dev)", []string{"https://y.dev"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractLinks(hideCode(tt.doc), nil); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHideCodeKeepsLineNumbers(t *testing.T) {
	doc := "```\na\nb\n```\n`x\ny`\n<!--\n-->\n[y](y.md)\n"
	got := hideCode(doc)
	if strings.Count(got, "\n") != strings.Count(doc, "\n") || len(got) != len(doc) {
		t.Fatalf("hideCode moved text: %q", got)
	}
	if want := []Found{{"y.md", 9}}; !reflect.DeepEqual(FindLinks(got), want) {
		t.Fatalf("got %v, want %v", FindLinks(got), want)
	}
}

func TestHeadingSlug(t *testing.T) {
	tests := []struct{ heading, want string }{
		{"Getting started", "getting-started"},
		{"Install & run!", "install--run"},
		{"What's new in v1.2?", "whats-new-in-v12"},
		{"The `-json` flag", "the--json-flag"},
		{"**Bold** and *italic*", "bold-and-italic"},
		{"_Why_ this works", "why-this-works"},
		{"snake_case_name", "snake_case_name"},
		{"[Docs](https://a.dev) here", "docs-here"},
		{`<a name="x"></a>Setup`, "setup"},
		{"Q&amp;A", "qa"},
		{"\U0001F680 Launch", "-launch"},             // an emoji is dropped
		{"a " + string(rune(0x2013)) + " b", "a--b"}, // and so is an en dash
		{"مقدمة سريعة", "مقدمة-سريعة"},
		{"Café Crème", "café-crème"},
	}
	for _, tt := range tests {
		if got := headingSlug(tt.heading); got != tt.want {
			t.Errorf("headingSlug(%q) = %q, want %q", tt.heading, got, tt.want)
		}
	}
}

func TestMarkdownAnchors(t *testing.T) {
	doc := strings.Join([]string{
		"# Title",
		"## Setup ##",
		"## Setup",
		"### Setup",
		"```bash",
		"# not a heading",
		"```",
		"Under line",
		"==========",
		"Second under line",
		"---",
		"",
		"---",
		"#nospace",
		"## Custom {#my-id}",
		`<a name="Old-Anchor"></a>`,
		`<div class="x" id="box"></div>`,
		"The `code` sits in # a sentence",
	}, "\n")
	want := []string{"title", "setup", "setup-1", "setup-2", "under-line", "second-under-line", "my-id", "old-anchor", "box"}
	got := markdownAnchors(doc)
	for _, a := range want {
		if !got[a] {
			t.Errorf("missing anchor %q in %v", a, got)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %d anchors, want %d: %v", len(got), len(want), got)
	}
}

func TestMarkdownAnchorsWindowsLineEndings(t *testing.T) {
	got := markdownAnchors("# One\r\n\r\nTwo words\r\n---\r\n")
	if !got["one"] || !got["two-words"] {
		t.Fatalf("got %v", got)
	}
}

func TestIsMarkdown(t *testing.T) {
	for path, want := range map[string]bool{"README.md": true, "docs/A.MARKDOWN": true, "a.html": false, "notes": false} {
		if got := isMarkdown(path); got != want {
			t.Errorf("isMarkdown(%q) = %v, want %v", path, got, want)
		}
	}
}
