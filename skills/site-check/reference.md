# sitecheck reference

Details the main skill file leaves out. Read the section you need.

## What each area checks

| Area | What it checks |
| --- | --- |
| basics | status, HTTPS and the http→https redirect, redirect chains, TLS expiry, compression, HTTP/2, soft 404s, favicon |
| performance | PageSpeed Insights mobile and desktop: Lighthouse scores, real-user Core Web Vitals (LCP, INP, CLS), top opportunities, failing accessibility, SEO and best-practice audits |
| social | Open Graph and X card tags *as the crawlers see them* (the page is refetched with the facebookexternalhit and Twitterbot user agents), plus the share image's reachability, format, size, dimensions and aspect ratio |
| seo | title, meta description, h1, lang, viewport, canonical, noindex, robots.txt (also for the preview crawlers), sitemap, JSON-LD, image alt text (including content photos marked `alt=""`) |
| content | main text extracted by **trafilatura** (saved to `content.md`), word count, heading outline, title vs h1, vague or repeated link text, links with no text |
| aeo | AI search and assistants (see [aeo.md](aeo.md)): robots.txt rules for each AI crawler, split into ones that answer or cite and ones that only collect training data; live fetches with AI crawlers' user agents to catch firewall or Cloudflare bot blocking; whether the text is in the server HTML; entity, author and date markup; FAQ-style structure; evidence density; byline and author page on articles; stock photos; Content-Signal; llms.txt; and, with a Brave key, the page's rank on Brave Search (Claude's index) |
| site | only with `-crawl N`: **advertools** crawls the site (sitemap first, obeys robots.txt). Broken internal URLs with the page linking to them, duplicate titles and descriptions, missing tags, sitemap entries that redirect, error, are noindexed or canonicalise elsewhere, orphan pages, hreflang return links, site-wide alt text and link text. Rows saved to `crawl.jl` |
| security | HSTS, CSP, nosniff, clickjacking, referrer policy, leaked server versions, mixed content |
| links | broken links on the page (the first 40, same-site first) |
| search | **your sites only:** Google Search Console (indexed or not, Google's chosen canonical, last crawl, rich results, 28 days of clicks, impressions and positions, top and "striking-distance" queries, sitemaps) and Bing Webmaster Tools (backlinks and linking pages) |

Flags: `-full`, `-json`, `-pass`, `-no-psi`, `-no-search`, `-no-brave`,
`-q "question"` (repeatable, up to 3), `-crawl N`, `-strategy mobile,desktop`,
`-lighthouse auto|always|off`, `-links 40`, `-out DIR`, `-timeout 20s`.

## Your site or someone else's

The report's header decides it from the accounts that are set up:

- `your site: verified in Google Search Console (...)`: the site is in the
  user's Search Console or Bing account, so the search section has real data.
- `not verified as your site`: accounts are set up, but this site isn't in
  them. A competitor, a client, or curiosity. Every other area still works.
- No such line: no account is set up. Mention once that setting one up adds
  index and query data for the user's own sites (see the README). Don't repeat
  it every run.

Adjust the report:

- **Your site:** lead with the search data. A page that isn't indexed, or where
  Google picked a different canonical, outranks anything a public check found.
  Striking-distance queries (ranking 5-20 with real demand) are the cheapest
  wins. Name them, and say where on the page to use those words.
- **Someone else's site:** don't present missing search data as a problem.
  Phrase fixes as observations ("their previews would break because …"). For
  a client, say they can add the user's Google account (or the service
  account's email) in Search Console, and verify the site in Bing. Their
  backlinks and keyword rankings need a paid data API (DataForSEO is the
  cheapest). Say so if asked.
- **Be polite to sites you don't own.** One run makes about 50 light requests.
  Don't loop sitecheck over many pages of someone else's site. Use `-links 0`
  for repeat runs.

## Crawling a site

On the user's own site, crawl it all: `-crawl` a bit above the sitemap size.
It runs about 70 pages a second at 4 connections. Orphan pages are only judged
when the crawl covers 90% of the sitemap. On someone else's site, keep `-crawl`
to 50 or less, and only when asked for a site-level look. After a crawl, run
sitecheck (without `-crawl`) on one URL per template (a blog post, a product
page) to read their content too.

## Performance source

It uses the PageSpeed Insights API when it can. The keyless quota is shared and
often exhausted (HTTP 429). It then falls back to local Lighthouse in headless
Chrome (`npx lighthouse@13`) and says so in a `source` line. Local runs have
**no real-user data**, and scores depend on this machine's network: present
them as rough lab numbers. If PageSpeed keeps failing, suggest a free key in
`~/.config/sitecheck/sitecheck.env`:

```
PAGESPEED_API_KEY=...   # Google Cloud console: enable "PageSpeed Insights API", then Credentials → API key
```

## Tools underneath

sitecheck calls pinned open-source tools: **trafilatura 2.3.1** (`uvx`),
**advertools 0.18.0** (`uvx`), and **Lighthouse 13** (`npx`, only when
PageSpeed is unavailable). The first run downloads each one. Without `uv` the
content checks are skipped; without Node the Lighthouse fallback is. If Claude
Code blocks one, the user needs an allow rule for that exact command (see the
README). Don't look for a way around the block.

## Manual checks to suggest

These need the user's own login, or a person looking:

- **Facebook Sharing Debugger** (developers.facebook.com/tools/debug): after
  fixing tags, click **Scrape Again**. Facebook caches previews for about 30
  days.
- **LinkedIn Post Inspector** (linkedin.com/post-inspector): also refreshes
  LinkedIn's cache.
- **X**: the Card Validator no longer shows previews. Paste the URL into a new
  post (don't send it) to see the card.
- **Google Rich Results Test** (search.google.com/test/rich-results) when the
  page has JSON-LD, and **Search Console → URL Inspection** for indexing.
- **bing.com** for the page's target questions: ChatGPT's main source, with no
  public search API.
