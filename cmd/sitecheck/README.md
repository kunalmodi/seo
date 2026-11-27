# sitecheck

Audits one web page and prints a findings report (FAIL / WARN / INFO / PASS).
By default it lists the top fixes; `-full` lists every finding by area.
It's the engine behind the [`site-check` skill](../../skills/site-check/SKILL.md),
and works fine on its own.

```bash
sitecheck https://example.com/            # top fixes, about 30s
sitecheck -full https://example.com/      # every finding, grouped by area
sitecheck -no-psi https://example.com/    # skip performance, about 1s
sitecheck -json https://example.com/      # machine-readable
```

| Area | Checks |
| --- | --- |
| basics | status, HTTPS, http→https, redirect chain, TLS expiry, compression, HTTP/2, soft 404, favicon |
| performance | PageSpeed Insights (mobile and desktop): Lighthouse scores, real-user Core Web Vitals, top opportunities, failing accessibility, SEO and best-practice audits |
| social | OG and X card tags fetched as `facebookexternalhit` and `Twitterbot`; share image reachability, format, size, dimensions and aspect ratio (saved to disk so you can look at it) |
| seo | title, description, h1, lang, viewport, canonical, noindex (meta and header), robots.txt per crawler, sitemap, JSON-LD validity, alt text |
| security | HSTS, CSP, nosniff, frame protection, referrer policy, version-leaking headers, mixed content |
| links | broken links (the first `-links` N, same-site first) |
| content | main text via **trafilatura** (saved to `content.md`), word count, heading outline, title vs h1, vague, repeated and empty link text |
| aeo | AI search and assistants: AI crawler rules in robots.txt (answer vs training crawlers), live fetches with AI user agents (catches firewall/Cloudflare blocks robots.txt doesn't show), text without JavaScript, entity/author/date markup, answer-shaped structure, direct answers under question headings and in the opening paragraph, evidence density (figures, quotations, sources), visible byline and author page on articles and health/money pages, expert reviewer and editorial-policy link on health/money pages, stock photos, Content-Signal, llms.txt; with `BRAVE_SEARCH_API_KEY`, where the page ranks on Brave Search (Claude's search index) for up to 3 questions; with `-ask`, what Claude, ChatGPT, Gemini, Perplexity and Grok search for, retrieve and cite when asked those questions through their APIs (how often the site is cited, retrieved or named, and who is cited instead; full answers saved to `answers.md`) |
| sitewide | every sitemap plus 3 sampled pages from each of the 4 largest sections: publishing bursts, build-timestamp lastmods, sampled noindex/canonical/404s, thin sections, health pages with no named author, hreflang alternates that 404, are noindexed or canonicalise to another language |
| site | `-crawl N`: **advertools** crawls the site (sitemap first, obeys robots.txt); broken internal URLs with their linking page, duplicate titles and descriptions, missing tags, sitemap entries that redirect, error or are noindexed, orphan pages, hreflang return links, site-wide alt and link text (rows saved to `crawl.jl`) |
| search | sites you own: Google Search Console (index status, Google's canonical, last crawl, rich results, 28 days of clicks, impressions and positions, top and striking-distance queries, sitemaps) and Bing Webmaster Tools (backlinks, linking pages) |

## Your sites and other people's

Most checks use only what any visitor can see, so they work on any site. The
search area needs the site verified in your own Search Console or Bing account,
and sitecheck works this out by itself. If an account is set up, the report
starts with `your site: verified in ...` or `not verified as your site`. On
someone else's site the search section is a single INFO line, not a warning.

Performance uses the PageSpeed Insights API. Set `PAGESPEED_API_KEY` in
`~/.config/sitecheck/sitecheck.env` (see `config/sitecheck.env.example`),
since the keyless quota is shared and often exhausted. When the API fails,
`-lighthouse auto` (the default) runs Lighthouse locally in headless Chrome,
either from PATH or via `npx -y lighthouse@13`. That gives lab data only.

Search data is optional: set `GSC_SERVICE_ACCOUNT_FILE` (recommended) or
`GOOGLE_OAUTH_CLIENT_FILE` plus a one-time `sitecheck auth google`, and
`BING_WEBMASTER_API_KEY`, `BRAVE_SEARCH_API_KEY` for the Brave rank check (works on any site), and for `-ask` any of `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GEMINI_API_KEY`, `PERPLEXITY_API_KEY`, `XAI_API_KEY` (models can be overridden with `SITECHECK_CLAUDE_MODEL`, `SITECHECK_OPENAI_MODEL`, `SITECHECK_GEMINI_MODEL`, `SITECHECK_PERPLEXITY_MODEL`, `SITECHECK_GROK_MODEL`). Setup steps are in `config/sitecheck.env.example`.
Access is read-only: the Google scope is `webmasters.readonly`.

## Open-source tools underneath

sitecheck does the rules and the glue, and leaves extraction and crawling to
established tools. Each one is pinned and fetched on first use:

| Tool | Version | Run via | Used for |
| --- | --- | --- | --- |
| [trafilatura](https://github.com/adbar/trafilatura) | 2.3.1 | `uvx` | main-text extraction |
| [advertools](https://github.com/eliasdabbas/advertools) (Scrapy) | 0.18.0 | `uvx` | site crawl (`-crawl`) |
| [Lighthouse](https://github.com/GoogleChrome/lighthouse) | 13 | `npx` | fallback when PageSpeed Insights is unavailable |

If Claude Code asks before running them, these allow rules cover exactly those
commands:

```
"Bash(uvx trafilatura@2.3.1:*)", "Bash(uvx --from advertools==0.18.0:*)", "Bash(npx -y lighthouse@13:*)"
```

They need `uv` (`brew install uv`) and Node.js. Change a pin in `tools.go` and
here together.

Flags: `-full`, `-json`, `-pass`, `-no-psi`, `-no-search`, `-no-brave`, `-no-sitewide`, `-q "question"` (repeatable), `-ask` (sample AI assistants; paid, on your keys), `-ask-runs 2`, `-ask-engines claude,chatgpt,gemini,perplexity,grok`, `-crawl N`, `-strategy mobile,desktop`,
`-lighthouse auto|always|off`, `-links 40`, `-out DIR`, `-timeout 20s`.
