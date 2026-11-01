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

func TestIsMarkdown(t *testing.T) {
	for path, want := range map[string]bool{"README.md": true, "docs/A.MARKDOWN": true, "a.html": false, "notes": false} {
		if got := isMarkdown(path); got != want {
			t.Errorf("isMarkdown(%q) = %v, want %v", path, got, want)
		}
	}
}
