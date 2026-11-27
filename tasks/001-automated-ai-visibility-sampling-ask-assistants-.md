---
id: 001
title: Automated AI visibility sampling: ask assistants the page's target questions, record searched/retrieved/cited
status: done
area: aeo
created: 2026-11-25
---

- 2026-11-25: Carried over from commandcenter task 032. Retrieval rank is the best proxy: ChatGPT cites mostly from Bing results, Claude from Brave's index. Brave has a Search API (done: -q); Bing's public API was retired, so Bing rank needs a manual check or a paid SERP API.
- 2026-11-27: Built as sitecheck -ask (ask.go): Claude, ChatGPT, Gemini, Perplexity and Grok through their APIs with web search on; records searches, retrieved and cited URLs per answer, tallies cited/retrieved/named and who is cited instead, saves answers.md. Only the Grok path has been run against the live API; the other four are written from the vendors' docs and tested against canned replies. Bing rank is still manual.
