# linkcheck

A fast broken-link checker for Markdown files, HTML files and websites. One binary, no dependencies.

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

The exit code is `1` when any link is broken, so it works as a CI step. It is `2` when linkcheck can't do the job: a flag is wrong, a file can't be read, or the first page of a crawl can't be loaded, so a site that is down never passes as "0 broken".

```yaml
- run: go install github.com/Ifazawezk/linkcheck@latest
- run: linkcheck README.md docs/*.md
```

## How it works

- Finds links in Markdown (`[text](url)`, `[label]: url`, `<url>`) and HTML (`href`, `src`), and resolves relative links on crawled pages.
- Skips fenced code blocks, inline code and HTML comments in Markdown files, since links there are usually sample text. Indented code blocks are still read. In HTML files it skips comments and the code inside `script` and `style` tags.
- Checks relative links in files against the disk, so a renamed or moved file is caught. Each place a broken link appears is reported, with its line number.
- Checks anchors as well: `#setup` in the same file and `docs/guide.md#setup` in another. A heading gets the anchor GitHub gives it, in any language, with `-1` and `-2` added to repeated headings. `{#custom-id}` after a heading and HTML `id` and `name` attributes count too. Case is ignored, and links to places like `script.py#L10` are left alone.
- Requires the exact case of a file name, because a link that opens on Windows or macOS can be a 404 on GitHub.
- Accepts a page linked without its extension, like `./guide/setup` for `guide/setup.md`, as docs sites such as VitePress and Docusaurus allow. In HTML files, `about` can mean `about.html`.
- Skips links that start with `/` unless `-root` is set, since they depend on where the site is served from, and links with template code in them like `{{ url }}` or `%BASE_URL%`.
- When crawling, stays on the host the first page ends up on, so a site that redirects from `example.com` to `www.example.com` is still followed.
- Tries a `HEAD` request first and falls back to `GET`, since many servers reject `HEAD` even when the page exists.
- Retries network errors and 5xx responses (twice by default, waiting 0.5s then 1s), since those are often temporary. This includes pages it loads to crawl. A 404 is not retried.
- Counts 4xx/5xx responses and network errors as broken. `429 Too Many Requests` is not counted, since the page usually exists.
- Checks web links in parallel with a worker pool, and checks each URL only once, so a broken URL is reported at the first place it appears.

## License

MIT
