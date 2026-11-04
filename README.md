# linkcheck

Finds broken links in Markdown files, HTML files and websites. It's a single Go binary with no dependencies.

```
$ linkcheck README.md docs/*.md
BROKEN  docs/instalation.md  (file not found)  in README.md:12
BROKEN  https://old-domain.example/api  (dial tcp: lookup old-domain.example: no such host)  in README.md:30
BROKEN  https://go.dev/this-page-does-not-exist  (404)  in docs/setup.md:8
57 links checked, 3 broken
```

## Install

```bash
go install github.com/Ifazawezk/linkcheck@latest
```

## Usage

```bash
linkcheck README.md docs/*.md          # check files
linkcheck docs                         # every Markdown and HTML file under docs
linkcheck https://mysite.com           # crawl a site (1 level deep by default)
linkcheck -depth 3 https://mysite.com  # follow same-site links 3 levels deep
linkcheck -json README.md > links.json # full results as JSON
linkcheck -ignore 'localhost|example\.com' README.md
linkcheck -root public public/*.html   # also check links that start with /
```

| Flag | Default | |
|---|---|---|
| `-workers` | 16 | links checked at the same time |
| `-timeout` | 10s | timeout per request |
| `-depth` | 1 | how many levels of same-site pages to crawl |
| `-json` | off | print every result as JSON, with the `source` and `line` each link was found on |
| `-ignore` | | skip links matching a regular expression |
| `-retries` | 2 | extra attempts after a network error or 5xx response |
| `-root` | | for files, the folder that links starting with `/` point into |

A folder is searched for `.md` and `.html` files, skipping hidden folders and `node_modules`. Wildcards like `docs/*.md` also work in PowerShell and cmd, which pass them on as typed.

Exit code `1` means at least one link is broken. `2` means linkcheck couldn't do the job: a bad flag, an unreadable file, or a crawl whose first page didn't load, so a site that is down never passes as "0 broken". In CI:

```yaml
- run: go install github.com/Ifazawezk/linkcheck@latest
- run: linkcheck README.md docs/*.md
```

## What it looks at

- Links in Markdown (`[text](url)`, `[label]: url`, `<url>`) and HTML (`href`, `src`). Relative links on crawled pages are resolved.
- Markdown fenced code blocks, inline code and HTML comments are skipped, since links there are usually sample text. Indented code blocks are still read. In HTML files, comments and the code inside `script` and `style` tags are skipped.
- Relative links in files are checked against the disk, with the exact case of the file name, because a link that opens on Windows or macOS can be a 404 on GitHub. Each place a broken link appears is reported with its line number.
- Anchors: `#setup` in the same file and `docs/guide.md#setup` in another. Headings get the anchor GitHub gives them, in any language, with `-1` and `-2` added to repeats. `{#custom-id}` after a heading and HTML `id` and `name` attributes count too. Case is ignored, and links like `script.py#L10` are left alone.
- A page linked without its extension, like `./guide/setup` for `guide/setup.md`, is accepted, as VitePress and Docusaurus allow. In HTML files, `about` can mean `about.html`.
- Links that start with `/` are skipped unless `-root` is set, since they depend on where the site is served from. So are links with template code like `{{ url }}` or `%BASE_URL%`.
- A crawl stays on the host the first page ends up on, so a redirect from `example.com` to `www.example.com` is still followed.
- Web links get a `HEAD` request, then `GET` if that fails, since many servers reject `HEAD` for pages that exist. Network errors and 5xx responses are retried (twice by default, waiting 0.5s then 1s), including pages loaded for crawling; a 404 is not.
- 4xx and 5xx responses and network errors count as broken, except `429 Too Many Requests`, since that page usually exists.
- Web links are checked in parallel by a worker pool, each URL only once, so a broken URL is reported at the first place it appears.

## License

MIT
