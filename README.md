# sitecheck: SEO and AEO audits for Claude Code

Point it at a live page and get a short, prioritized list of what to fix so the
page can be found, chosen and shared: by Google, by AI assistants (ChatGPT,
Claude, Perplexity, AI Overviews), and in link previews.

It's two things:

- **`sitecheck`**, a fast command-line auditor (one Go binary, no setup). It
  runs dozens of rule checks and prints the worst problems first.
- **A Claude Code skill** that runs it, then does what rules can't: looks at
  your share image, reads your copy, works out what search the page is trying
  to win, and tells you what to change, grounded in what the evidence actually
  says about AI search.

## Quick start (Claude Code)

In Claude Code:

```
/plugin marketplace add kunalmodi/seo
/plugin install sitecheck@seo
```

Install the binary:

```bash
curl -fsSL https://raw.githubusercontent.com/kunalmodi/seo/main/install.sh | sh
```

(or `go install github.com/kunalmodi/seo/cmd/sitecheck@latest`)

Then just ask: *"check https://mysite.com/pricing"*, *"is this page ready to
launch?"*, *"why does my link preview look wrong?"* or *"how do I show up in
ChatGPT for 'best X for Y'?"*. The skill is `/sitecheck:site-check` if you want
to call it directly.

Optional: `brew install uv` turns on the content checks (main-text extraction)
and site crawling. Everything else works without it.

## Without Claude

```bash
sitecheck https://go.dev/
```

```
sitecheck 0.2.0: https://go.dev/
0 FAIL, 5 WARN, 21 INFO, 26 PASS

## top fixes
WARN  [seo] Image alt text: 16 of 55 <img> have no alt attribute ...
WARN  [seo] Sitemap: none found (looked at https://go.dev/sitemap.xml)
WARN  [content] Title vs h1: they share no words ...
...
Run with -full for every finding by area, including 21 INFO notes, or -json.
```

`-full` shows everything by area, `-json` is for scripts, `-no-psi` skips the
~30s performance check, `-crawl 200` adds site-wide checks.

## What it checks

- **Can it be found?** Status and redirects, HTTPS, canonical, noindex,
  robots.txt, sitemap, soft 404s, broken links. With `-crawl`: broken internal
  links, duplicate titles, sitemap health, orphan pages, hreflang.
- **Can AI assistants get to it?** robots.txt rules for every AI crawler
  (split into *search* crawlers, which you want, and *training* crawlers, which
  are your call), plus live fetches as those crawlers to catch Cloudflare or
  firewall bot blocking that robots.txt doesn't show. Whether the text is in
  the HTML, since most AI crawlers don't run JavaScript.
- **Will it be chosen?** Title, description, h1 and outline; whether they agree;
  link text; evidence density (figures, quotes, cited sources); author, byline,
  dates and entity markup; stock photos. Optionally, where the page ranks on
  Brave Search, the index Claude's web search uses.
- **Does it look right when shared?** Open Graph and X tags *as Facebook and X
  fetch them*, and the share image's size, format and crop. The skill looks at
  the image itself.
- **Is it fast and sound?** PageSpeed Insights / Lighthouse scores and
  real-user Core Web Vitals, security headers, mixed content, TLS expiry.
- **Your own sites:** Google Search Console (indexed or not, queries,
  positions, "striking distance" keywords) and Bing Webmaster backlinks, when
  you connect them. Works on anyone's site without them.

## AEO: what we check, and what we deliberately don't

AEO ("answer engine optimization") advice online is mostly vendor marketing.
The skill carries a short evidence file, [aeo.md](skills/site-check/aeo.md),
with a confidence level and a source for each claim. The gist:

- **Retrieval comes first.** ChatGPT cites mostly from Bing's results, Claude
  from Brave's. If the page isn't in those results, nothing else matters. So
  the first AEO advice is usually about Bing indexing and not blocking AI
  *search* crawlers.
- **Then quotability.** Direct answers, self-contained passages, and above all
  statistics, quotations and cited sources (up to ~40% more visibility in the
  best study there is).
- **Off-site mentions** (Reddit, reviews, "alternatives" lists) matter more
  than backlinks for AI visibility.

What it won't push, and why:

- **llms.txt:** 97% of published files got zero requests in Ahrefs' data;
  Google doesn't use it. sitecheck reports whether you have one, as INFO.
- **"AI schema" or rewriting into chunks:** Google says neither helps its AI
  features. Ordinary structured data still helps ordinary search.
- **Blocking GPTBot or ClaudeBot isn't an error.** Those are training crawlers;
  blocking them doesn't hide you from AI search.
- **No promised citations.** Answers vary run to run. The skill explains how to
  sample assistants properly (clean sessions, several runs, log what was
  searched, retrieved and cited).

## Optional keys

All optional. Put them in `~/.config/sitecheck/sitecheck.env` (outside any
repo); see [config/sitecheck.env.example](config/sitecheck.env.example) for
step-by-step setup.

| Key | Adds |
| --- | --- |
| `PAGESPEED_API_KEY` | reliable PageSpeed results (the free shared quota often runs out; sitecheck then falls back to local Lighthouse if Node.js is installed) |
| `GSC_SERVICE_ACCOUNT_FILE` or `GOOGLE_OAUTH_CLIENT_FILE` | Search Console data for your sites (read-only) |
| `BING_WEBMASTER_API_KEY` | Bing backlinks for your sites |
| `BRAVE_SEARCH_API_KEY` | the page's Brave Search rank (free plan works) |

## Being polite

One run makes about 50 light requests. Fine for any site, but don't loop it
over hundreds of pages of a site you don't own.

## Development

Go 1.21+, standard library only.

```bash
make test      # go vet + unit tests
make install   # build into ~/.local/bin
make skill     # symlink the skill into ~/.claude/skills for live editing
```

Releases: push a `v*` tag and GitHub Actions publishes the binaries that
`install.sh` downloads. More detail on the CLI is in
[cmd/sitecheck/README.md](cmd/sitecheck/README.md).

MIT licensed.
