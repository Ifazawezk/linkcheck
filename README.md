# linkcheck

A fast broken-link checker for Markdown files, HTML files and websites. One binary, no dependencies.

```
$ linkcheck README.md docs/*.md
BROKEN  https://go.dev/this-page-does-not-exist  (404)  in docs/setup.md
BROKEN  https://old-domain.example/api  (dial tcp: lookup old-domain.example: no such host)  in README.md
57 links checked, 2 broken
```

## Install

```bash
go install github.com/Ifazawezk/linkcheck@latest
```

## Usage

```bash
linkcheck README.md docs/*.md          # check files
linkcheck https://mysite.com           # crawl a site (1 level deep by default)
linkcheck -depth 3 https://mysite.com  # follow same-site links 3 levels deep
linkcheck -json README.md > links.json # full results as JSON
linkcheck -ignore 'localhost|example\.com' README.md
```

| Flag | Default | |
|---|---|---|
| `-workers` | 16 | links checked at the same time |
| `-timeout` | 10s | timeout per request |
| `-depth` | 1 | how many levels of same-site pages to crawl |
| `-json` | off | print every result as JSON |
| `-ignore` | | skip links matching a regular expression |

The exit code is `1` when any link is broken, so it works as a CI step:

```yaml
- run: go install github.com/Ifazawezk/linkcheck@latest
- run: linkcheck README.md docs/*.md
```

## How it works

- Finds links in Markdown (`[text](url)`, `<url>`) and HTML (`href`, `src`), and resolves relative links on crawled pages.
- Tries a `HEAD` request first and falls back to `GET`, since many servers reject `HEAD` even when the page exists.
- Counts 4xx/5xx responses and network errors as broken. `429 Too Many Requests` is not counted, since the page usually exists.
- Checks links in parallel with a worker pool, and checks each URL only once.

## License

MIT
