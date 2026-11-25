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
search, seo, content, aeo, site, social, security, links. What each one covers,
the other flags, and how to treat the user's own site versus someone else's are
in [reference.md](reference.md). Read its "Your site or someone else's" section
the first time the header says `your site` or `not verified as your site`.

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
