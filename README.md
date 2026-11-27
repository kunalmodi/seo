# sitecheck: SEO and AEO analysis for any website

Give it a URL. It checks the page and the site the way Google, Bing and the AI
assistants (ChatGPT, Claude, Perplexity, Google's AI Overviews) actually see
them, and returns a short, prioritized list of what to fix.

It comes in two parts:

- **An agent skill** (`site-check`) that turns the results into advice. It
  works out which search the page is trying to win, reads the copy like an
  editor, looks at the share image, and weighs AEO claims against the evidence
  instead of the hype.
- **`sitecheck`**, the command-line engine underneath. It's a single binary
  with no setup, makes about 120 checks, and puts the worst problems first.
  It's useful on its own, in scripts or CI.

## What makes it different

Most free SEO checkers read your HTML once, with a browser user agent, and
grade it against a checklist. sitecheck does that too, and adds:

- **It fetches as the crawlers do.** It requests the page as Facebook's and
  X's preview bots, and as 15 AI crawlers: OAI-SearchBot, GPTBot,
  ChatGPT-User, ClaudeBot, Claude-SearchBot, PerplexityBot and others.
  Cloudflare's bot blocking or a firewall rule often turns these away while
  robots.txt says they're welcome, and the owner rarely knows. Being blocked
  from AI *search* crawlers is the most common reason a site never shows up in
  ChatGPT or Perplexity answers.
- **It looks at the whole site, not only the page.** It reads every sitemap
  and samples pages from each big section. That catches the problems that sink
  sites whose pages each look fine:
  - hundreds of templated pages published on one day: the pattern Google's
    scaled-content-abuse policy targets, and a common reason a new site drops
    out of the index overnight
  - a whole template accidentally set to `noindex`, or with its canonical
    pointing somewhere else
  - health pages with no named author or reviewer
  - translations that 404, are noindexed, or point their canonical back at
    the original language
- **It checks the page against the searches assistants actually run.** When
  ChatGPT or Claude answers, it splits the question into several searches,
  mostly fact checks: is it free, what does it cost, does it work with X, how
  does it compare, what's current this year. The skill writes those searches
  for your page, runs them to see whether you're retrieved and who ranks
  ahead, and checks whether the page answers each one in plain text. The
  result is a table of which searches you'd win, which you're missing, and the
  page you'd need to build.
- **It separates the two kinds of AI crawler.** Search crawlers decide whether
  you can be cited. Training crawlers don't affect that, so blocking GPTBot or
  ClaudeBot is your call, never an error.
- **Its AEO advice is graded by evidence.** Every claim in
  [aeo.md](skills/site-check/aeo.md) carries a source and a confidence level,
  including a field test of what ChatGPT and Claude really search for and
  cite.
- **It uses real data for your own sites.** Connect Search Console and it
  shows whether Google indexed the page, which canonical Google picked, and
  the queries where you rank 5th to 20th with real demand: the cheapest wins
  there are. It also adds Bing backlinks, and the page's rank on Brave Search,
  the index Claude's web search uses.
- **It handles any site politely.** It works on competitors' and clients' sites
  with no accounts, knows which sites are yours, and keeps its requests light.

## What it checks

**Indexing and crawling**
- HTTP status, soft 404s, redirect chains, http→https redirects
- `noindex` in the page or its headers, canonical tags, robots.txt rules for
  each crawler, sitemaps, sitemap entries that redirect, 404 or are
  noindexed
- With `-crawl N`: broken internal links (with the page linking to them),
  orphan pages, hreflang return links, and duplicate titles and descriptions
  across the site

**Site-wide patterns** (every run)
- Publishing bursts, sitemap `lastmod` dates that are just build timestamps,
  oversized sections on a young site
- Sampled pages from each section: indexable, self-canonical, not thin, and a
  named author on health topics
- hreflang alternates: reachable, indexable, and not canonicalised away

**On-page SEO**
- Title and meta description length (Google cuts them off), h1, heading
  levels, whether the title and h1 agree
- Language, viewport, charset, structured data (valid JSON-LD; which rich
  results it could earn)
- Image alt text, including product photos wrongly marked decorative
- Vague, repeated or empty link text, internal `nofollow`

**Content** (main text extracted with [trafilatura](https://github.com/adbar/trafilatura))
- Word count and heading outline, saved for the agent to read
- The agent judges which query the page targets, whether the copy answers it,
  what a searcher would expect that's missing, and trust signals for health
  and money topics

**AEO: AI search and assistants**
- Robots.txt and live fetches for each AI search and training crawler
- How much of the text is in the server HTML (most AI crawlers don't run
  JavaScript)
- **Does it answer its questions?** Whether each question heading is followed
  by a direct answer in its first sentence, and whether the opening paragraph
  answers the headline or clears its throat ("In today's world…")
- **Who stands behind it?** A visible byline, and an author page with a photo,
  Person markup and `sameAs` profiles. On health and money pages, even ones
  not marked up as articles: a named expert reviewer with credentials, and a
  link to an editorial or review policy
- Entity and date markup, stock photos, and how many figures, quotations and
  cited sources the page has (the strongest lever in the research)
- Freshness, Content-Signal, llms.txt (reported, but not pushed: there's no
  evidence it helps)
- Brave Search rank for the questions you name (`-q`)

**Link previews**
- Open Graph and X card tags as Facebook and X actually fetch them, duplicate
  tags
- The share image: reachable, format, file size, dimensions, aspect ratio. The
  agent looks at it to check whether text is readable at thumbnail size and
  whether it survives cropping

**Performance**
- PageSpeed Insights for mobile and desktop: Lighthouse scores, real-user Core
  Web Vitals (LCP, INP, CLS), the top opportunities, and failing accessibility
  and best-practice audits. It falls back to a local Lighthouse run when the
  free quota runs out
- Response time, HTML size, compression, HTTP/2

**Security and hygiene**
- HTTPS, TLS certificate expiry, HSTS, CSP, nosniff, clickjacking protection,
  referrer policy, leaked server versions, mixed content
- Broken links on the page

**Your own sites** (with Search Console and Bing connected)
- Index status, Google's chosen canonical, last crawl, rich results
- 28 days of clicks, impressions and positions; top, striking-distance and
  low click-through queries; submitted sitemaps
- Bing backlinks and linking pages

## Install

**As a Claude Code plugin** (bundles the skill):

```
/plugin marketplace add kunalmodi/seo
/plugin install sitecheck@seo
```

**The binary** (macOS and Linux):

```bash
curl -fsSL https://raw.githubusercontent.com/kunalmodi/seo/main/install.sh | sh
```

or `go install github.com/kunalmodi/seo/cmd/sitecheck@latest`.

The skill is a plain [Agent Skill](https://agentskills.io) folder
([skills/site-check](skills/site-check)), so any agent that can run shell
commands can use it. Optional: `brew install uv` turns on main-text extraction
and site crawling, and Node.js enables the local Lighthouse fallback.

## Use

Ask your agent things like *"check https://mysite.com/pricing"*, *"why did our
traffic drop?"*, *"is this page ready to launch?"*, *"why does my link preview
look wrong?"* or *"how do I show up in ChatGPT for 'best X for Y'?"*

Or run it directly:

```bash
sitecheck https://go.dev/
```

```
sitecheck 0.4.1: https://go.dev/
0 FAIL, 5 WARN, 23 INFO, 25 PASS

## top fixes
WARN  [seo] Image alt text: 16 of 55 <img> have no alt attribute ...
WARN  [seo] Sitemap: none found (looked at https://go.dev/sitemap.xml)
WARN  [content] Title vs h1: they share no words ...
...
Run with -full for every finding by area, including 23 INFO notes, or -json.
```

`-full` shows every finding by area, `-json` gives machine-readable output,
`-no-psi` skips the ~30s performance check, and `-crawl 200` adds a site crawl.
The [CLI README](cmd/sitecheck/README.md) has every flag.

## AEO: what it pushes, and what it deliberately doesn't

The short version of [aeo.md](skills/site-check/aeo.md):

- **Retrieval comes first.** ChatGPT cites mostly from Bing's results, and
  Claude from Brave's. A page that isn't in those results can't be cited, so
  the first advice is usually about Bing indexing and not blocking the AI
  search crawlers.
- **Then quotability:** direct answers, passages that stand on their own, and
  above all statistics, quotations and cited sources (up to ~40% more
  visibility in the best study there is).
- **Off-site mentions** (Reddit, reviews, "alternatives" lists) correlate with
  AI visibility more than backlinks do.

What it won't push, and why:

- **llms.txt:** 97% of published files got no requests in Ahrefs' data, and
  Google doesn't use them.
- **"AI schema" or rewriting content into chunks:** Google says neither helps
  its AI features. Ordinary structured data still helps ordinary search.
- **Promised citations:** answers vary from run to run. Instead, the skill
  explains how to sample assistants properly: clean sessions, several runs,
  and a log of what they searched for, retrieved and cited.

## Optional keys

All optional. Put them in `~/.config/sitecheck/sitecheck.env`, outside any
repo. [config/sitecheck.env.example](config/sitecheck.env.example) walks
through the setup.

| Key | Adds |
| --- | --- |
| `PAGESPEED_API_KEY` | reliable PageSpeed results (the free shared quota often runs out) |
| `GSC_SERVICE_ACCOUNT_FILE` or `GOOGLE_OAUTH_CLIENT_FILE` | Search Console data for your sites (read-only) |
| `BING_WEBMASTER_API_KEY` | Bing backlinks for your sites |
| `BRAVE_SEARCH_API_KEY` | the page's Brave Search rank (the free plan works) |

## Being polite

A run makes about 60 to 120 light requests, depending on how many sitemaps the
site has. That's fine for any site, but don't loop it over hundreds of pages
of a site you don't own.

## Development

Go 1.21+, standard library only.

```bash
make test      # go vet + unit tests
make install   # build into ~/.local/bin
make skill     # symlink the skill into ~/.claude/skills for live editing
```

To release, push a `v*` tag. GitHub Actions then publishes the binaries that
`install.sh` downloads.

MIT licensed.
