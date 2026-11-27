---
name: site-check
description: Audit a web page for SEO, AEO (getting cited by ChatGPT, Claude, Perplexity and AI Overviews) and launch quality. Covers PageSpeed / Core Web Vitals, how its link preview looks when shared (Facebook, X, LinkedIn, iMessage, Slack), SEO basics, AI crawler access, HTTPS, security headers and broken links. For the user's own sites it adds Google Search Console data (indexing, queries and positions) and Bing backlinks. Use when the user asks to check, audit or review a URL or website, asks "is this page ready to launch/share", asks why a link preview looks wrong, asks how to show up in ChatGPT or AI search, or wants SEO or performance feedback on a live page.
---

# Site check

Audit one live URL and give the user a short, prioritized list of what to fix.
The `sitecheck` command does the fetching and rule checks. Your job is to run
it, look at what a person would see (the share image and the text), and turn
the findings into advice.

## 0. Make sure sitecheck is installed

Run `sitecheck -h`. If it isn't found, ask the user to install it with one of
these (don't run them without a yes):

```bash
curl -fsSL https://raw.githubusercontent.com/kunalmodi/seo/main/install.sh | sh
```

```bash
go install github.com/kunalmodi/seo/cmd/sitecheck@latest
```

The installer puts it in `~/.local/bin`. Optional extras: `uv`
(`brew install uv`) for the content checks, Node.js for local Lighthouse when
PageSpeed's free quota runs out.

## 1. Run the check

```bash
sitecheck -full https://example.com/page
```

Use `-full` so you see every finding grouped by area (the default prints only
the top fixes, which is meant for people). About 30 seconds; add `-no-psi` to
skip performance (about 1 second). If you know the questions the page should
win in AI search, pass up to three with `-q "question"`. For a whole site, see
"Crawling a site" in [reference.md](reference.md).

Findings are marked FAIL, WARN, INFO or PASS, in areas: basics, performance,
search, sitewide, seo, content, aeo, site, social, security, links. What each one covers,
the other flags, and how to treat the user's own site versus someone else's are
in [reference.md](reference.md). Read its "Your site or someone else's" section
the first time the header says `your site` or `not verified as your site`.

### When traffic or indexed pages dropped

If the user says the site lost traffic or dropped out of Google, the cause is
usually site-wide, and every single page can look fine. Read the `sitewide`
area first:

- **Sampled pages noindexed, or canonicals pointing elsewhere:** a deploy or
  template change told Google to drop them. That's the answer; fix it first.
- **Publishing burst:** hundreds of pages in one section went up on one day.
  On a young site, and especially on health or money topics without a named
  author, this is the pattern Google's scaled-content-abuse policy targets.
  Read one or two of those pages yourself: do they add anything beyond the
  sources they restate?
- **Thin section** and **Health pages without an author** strengthen that
  reading.

Then ask for what only the owner can see, in this order: Search Console →
Manual actions; the Pages report's reasons pages aren't indexed ("Crawled -
currently not indexed" across the site points to quality, "Excluded by
noindex" or "Blocked" to something technical); Crawl stats, since sitecheck
only spoofs Googlebot's user agent and a firewall can still block the real
one. Say plainly what is confirmed and what is a likely cause.

For spam-policy or quality causes, the fix is to remove (404/410) or noindex
the mass-produced pages and take them out of the sitemap, keep the pages that
earn their place, put named people on health content, and expect recovery to
take weeks to months. With a manual action, the owner files a reconsideration
request after the cleanup.

## 2. Look at the share image

The `files` section has the share image path (`share_image`, and
`x_share_image` if X gets a different one). **Open it with the Read tool.**
The checks can't judge:

- Is it the right image for this page, not a placeholder, default logo or blank?
- Is text readable at thumbnail size (about 500px wide, smaller in iMessage)?
- Does it survive cropping to 1.91:1, and to a square (WhatsApp, X `summary`)?
- Does it work on light and dark backgrounds? Transparent PNGs often don't.

## 3. Read the content

`content.md` (in `files`) holds the heading outline and main text. **Read it.**

- **What search is this page trying to win?** Say it as a query a person would
  type. If you can't tell, neither can Google. That's the finding.
- **Do the title, h1, first paragraph and subheadings aim at that query**, in
  searchers' words? A clever tagline h1 is weaker than a plain one.
- **Coverage:** what would that searcher expect that's missing? Prices,
  comparisons, who it's for, how to use it, sources.
- **Trust** for health, money and safety topics: author or reviewer, dates,
  sources, disclosures.
- On listing pages (home, categories) extraction may drop names; judge from the
  outline.

## 4. AEO: AI search and assistants

**Read [aeo.md](aeo.md) before giving AEO advice.** It says what the evidence
supports, how to weigh the `aeo` findings, and how to test AI visibility by
hand. The short version:

1. **Retrieval decides everything.** ChatGPT cites mostly from Bing's results,
   Claude from Brave's. A page outside those results can't be cited. So: get
   into Bing (Webmaster Tools, sitemap, IndexNow), stay crawlable, and don't
   block the AI *search* crawlers. A search crawler refused by the firewall
   while robots.txt allows it is the most important AEO finding.
2. **Then be quotable:** a direct answer up top, passages that stand alone,
   and statistics, quotations and cited sources.
3. **Keep it fresh**, and get mentioned where the questions are already
   answered (Reddit, review sites, "alternatives" lists).
4. **Skip the myths:** don't recommend llms.txt or AI-specific schema as fixes,
   and don't call blocking *training* crawlers an error. Never promise
   citations.

### Fan-out coverage

When the user cares about AI search, or the page sells or compares something,
do this yourself. It's the most AEO-specific advice you can give. Follow
"Fan-out coverage" in [aeo.md](aeo.md):

1. Write the page's target question, plus the 5-8 searches an assistant would
   run to answer it. Mostly they're fact checks: free or not, limits, price,
   compatibility, "vs <rival>", "<topic> <current year>".
2. Run them with your own web search tool if you have one, and note where the
   site ranks and who's ahead. Pass the three that matter most to
   `sitecheck -q "..."` for exact Brave ranks.
3. Check `content.md` for whether each fact is stated in plain text.
4. Report it as a table: search, whether the site is retrieved (and who is
   instead), whether the page answers it, and what to add.

### Sampling the assistants (`-ask`)

`sitecheck -ask -q "..." URL` asks Claude, ChatGPT, Gemini, Perplexity and
Grok (whichever the user has API keys for) the questions, with web search on,
and adds an `AI answers: <assistant>` line for each. It bills the user's keys,
roughly a few cents to 50 cents per answer, so **ask before running it** and
say how many answers it will make (questions x runs x assistants). Use real
buyer questions, not the page title. Then:

- **Read `answers.md`** (`ai_answers` in `files`). Check what each assistant
  says about the site against the page: wrong prices, a missing feature, or
  a different product with the same name. "Named" only means the name
  appeared, so confirm it's this site they mean.
- **"It searched"** is the real fan-out. Use those queries in the fan-out
  table in place of your guesses, and pass the important ones back to `-q`.
- **Never retrieved** means a discoverability problem (Bing, Brave, crawler
  access, the page's words). **Retrieved but not cited** means the page didn't
  answer as plainly as the ones cited.
- **"Cited instead"** lists the pages to learn from and, for listicles,
  review sites and Reddit threads, the places to get mentioned.
- Say that these are API answers, which differ from the apps, and that counts
  from a few runs are a sample, not a score.

## 5. Report back

Lead with the three to five fixes that matter most, in plain language, each
with where to make it. Order: can't be found (not indexed, blocked crawlers,
broken status) → can't be chosen (title, content, evidence) → polish (previews,
performance, headers). Then two or three concrete content suggestions, not a
generic checklist. Suggest the relevant manual checks from
[reference.md](reference.md) last.

If the user wants to *see* the link previews, sketch them as a small HTML page
with the real title, description, site name and image, labelled as an
approximation.
