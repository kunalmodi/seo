package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Site-wide shape: problems no single page shows. A site can pass every page
// check and still drop out of Google's index, most often because it published
// hundreds of templated pages at once (Google's "scaled content abuse"
// policy), or because a deploy noindexed a whole template. This area reads the
// sitemaps (no crawl) and fetches a few pages from each big section.

const (
	sampleSections    = 4 // largest sections sampled
	samplesPerSection = 3
	maxAlternates     = 3 // hreflang alternates checked
)

var healthRe = regexp.MustCompile(`(?i)\b(stud(y|ies)|trials?|clinical|disease|cancer|kidney|liver|heart|cardio\w*|diabet\w*|obesity|doses?|dosage|medications?|medicines?|drugs?|treatments?|symptoms?|patients?|therap\w*|semaglutide|tirzepatide|ozempic|wegovy|mounjaro|insulin|blood|pregnan\w*|supplements?|side effects?|weight loss|mental health|anxiety|depression|vaccines?)\b`)

// section is a sitemap URL's first path segment ("/research/"), or "/" for
// top-level pages.
func section(loc string) string {
	u, err := url.Parse(loc)
	if err != nil {
		return "/"
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" {
		return "/"
	}
	return "/" + parts[0] + "/"
}

// lastmodDay is the YYYY-MM-DD part of a sitemap lastmod, or "".
func lastmodDay(s string) string {
	if len(s) >= 10 {
		// Ignore impossible years (typos like 0023) rather than call them the oldest page.
		if t, err := time.Parse("2006-01-02", s[:10]); err == nil && t.Year() >= 1995 {
			return s[:10]
		}
	}
	return ""
}

type sectionStat struct {
	name     string
	urls     []string
	topDay   string // the most common lastmod day
	topCount int    // URLs on topDay
	dated    int    // URLs with a lastmod
}

type shape struct {
	total    int
	sections []*sectionStat // largest first
	oldest   string         // earliest lastmod day
	siteDay  string         // most common day across the whole sitemap
	siteDayN int
	dated    int
}

func analyzeShape(entries []sitemapEntry) shape {
	by := map[string]*sectionStat{}
	days := map[string]int{}
	secDays := map[string]map[string]int{}
	var sh shape
	for _, e := range entries {
		s := section(e.loc)
		st := by[s]
		if st == nil {
			st = &sectionStat{name: s}
			by[s] = st
			secDays[s] = map[string]int{}
		}
		st.urls = append(st.urls, e.loc)
		if d := lastmodDay(e.lastmod); d != "" {
			st.dated++
			sh.dated++
			days[d]++
			secDays[s][d]++
			if sh.oldest == "" || d < sh.oldest {
				sh.oldest = d
			}
		}
	}
	sh.total = len(entries)
	sh.siteDay, sh.siteDayN = maxDay(days)
	for name, st := range by {
		st.topDay, st.topCount = maxDay(secDays[name])
		sh.sections = append(sh.sections, st)
	}
	sort.Slice(sh.sections, func(i, j int) bool {
		if len(sh.sections[i].urls) != len(sh.sections[j].urls) {
			return len(sh.sections[i].urls) > len(sh.sections[j].urls)
		}
		return sh.sections[i].name < sh.sections[j].name
	})
	return sh
}

func maxDay(m map[string]int) (string, int) {
	best, n := "", 0
	for d, c := range m {
		if c > n || (c == n && d > best) {
			best, n = d, c
		}
	}
	return best, n
}

// burst is a section where most URLs share one lastmod day, when the rest of
// the site doesn't (a site-wide shared date is a build timestamp instead).
func (sh shape) bursts(now time.Time) []*sectionStat {
	if sh.dated == 0 || float64(sh.siteDayN) >= 0.9*float64(sh.dated) {
		return nil
	}
	var out []*sectionStat
	for _, st := range sh.sections {
		// A burst has to be a real share of the site, not one small section.
		if st.topCount < 50 || float64(st.topCount) < 0.8*float64(st.dated) || pct(len(st.urls), sh.total) < 15 {
			continue
		}
		if d, err := time.Parse("2006-01-02", st.topDay); err == nil && now.Sub(d) <= 365*24*time.Hour {
			out = append(out, st)
		}
	}
	return out
}

func pct(n, of int) int {
	if of == 0 {
		return 0
	}
	return (100*n + of/2) / of
}

// sample is what one fetched page from a section tells us.
type sample struct {
	url       string
	status    int
	noindex   string // where the noindex came from, if any
	canonical string // set when the canonical points elsewhere
	words     int
	health    bool
	person    string // a named author, if any
	hreflang  []string
}

func checkSitewide(ctx context.Context, r *report, f *fetcher, pg *page, now time.Time) {
	const a = "sitewide"
	entries := sitemapEntries(ctx, f, pg.final, 50000)
	var same []sitemapEntry
	for _, e := range entries {
		if u, err := url.Parse(e.loc); err == nil && sameSite(u, pg.final) {
			same = append(same, e)
		}
	}
	if len(same) < 20 {
		r.add(a, info, "Site shape", "skipped: the sitemap lists %d pages on this site (needs 20 or more)", len(same))
		return
	}
	sh := analyzeShape(same)
	r.fact("sitemap_sections", sectionSummary(sh, 6))

	var targets []*sectionStat
	for _, st := range sh.sections {
		if len(targets) < sampleSections && len(st.urls) >= 10 {
			targets = append(targets, st)
		}
	}
	samples := map[string][]sample{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for _, st := range targets {
		for _, u := range spread(st.urls, samplesPerSection) {
			st, u := st, u
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				s := samplePage(ctx, f, u)
				mu.Lock()
				samples[st.name] = append(samples[st.name], s)
				mu.Unlock()
			}()
		}
	}
	wg.Wait()
	reportSitewide(r, sh, samples, now)
	checkAlternates(ctx, r, f, samples)
}

// reportSitewide turns the sitemap shape and page samples into findings. It's
// separate from the fetching so it can be tested.
func reportSitewide(r *report, sh shape, samples map[string][]sample, now time.Time) {
	const a = "sitewide"
	young := sh.oldest != "" && now.Sub(mustDay(sh.oldest)) < 120*24*time.Hour
	r.add(a, info, "Sitemap shape", "%d URLs: %s%s", sh.total, sectionSummary(sh, 6), func() string {
		if sh.oldest != "" {
			return fmt.Sprintf("; oldest lastmod %s", sh.oldest)
		}
		return ""
	}())
	if sh.dated > 0 && float64(sh.siteDayN) >= 0.9*float64(sh.dated) && sh.total >= 50 {
		r.add(a, info, "Sitemap lastmod", "%d%% of URLs share lastmod %s, which looks like a build timestamp; Google ignores lastmod it can't trust, so set it to when each page really changed", pct(sh.siteDayN, sh.dated), sh.siteDay)
	}

	// Which sampled sections are health pages with nobody named on them?
	unsigned := map[string]bool{}
	for name, ss := range samples {
		health, named := 0, 0
		for _, s := range ss {
			if s.status != 200 {
				continue
			}
			if s.health {
				health++
			}
			if s.person != "" {
				named++
			}
		}
		if health > 0 && 2*health >= len(ss) && named == 0 {
			unsigned[name] = true
		}
	}

	burst := sh.bursts(now)
	for _, st := range burst {
		share := pct(len(st.urls), sh.total)
		lvl := warn
		if st.topCount >= 200 && share >= 40 {
			lvl = fail
		}
		var why []string
		if young {
			why = append(why, fmt.Sprintf("on a site whose oldest lastmod is %s", sh.oldest))
		}
		if unsigned[st.name] {
			why = append(why, "on health topics with no named author on the pages sampled")
		}
		ctx := ""
		if len(why) > 0 {
			ctx = ", " + strings.Join(why, " and ")
		}
		r.add(a, lvl, "Publishing burst", "%d of %d %s URLs (%d%% of the sitemap) were last modified on %s%s. Hundreds of templated pages at once is what Google's scaled-content-abuse policy targets, and it can pull the whole site out of the index. If indexing dropped, check Search Console's Manual actions, then remove or noindex the pages that only restate other sources. (A template change that reset lastmod looks the same; check which it was.)",
			st.topCount, len(st.urls), st.name, share, st.topDay, ctx)
	}
	// On an established site a big section is normal; on a new one it's a risk.
	if len(burst) == 0 && young && len(sh.sections) > 1 {
		if top := sh.sections[0]; len(top.urls) >= 200 && pct(len(top.urls), sh.total) >= 60 && top.name != "/" {
			r.add(a, info, "Dominant section", "%s holds %d%% of the sitemap (%d URLs); make sure those pages each earn their place, since a large thin section can drag down the whole site", top.name, pct(len(top.urls), sh.total), len(top.urls))
		}
	}
	if young && sh.total >= 300 && len(burst) == 0 {
		r.add(a, info, "Young site", "%d URLs, all last modified since %s; Google is cautious with large new sites, so publish in batches it can index and judge", sh.total, sh.oldest)
	}

	names := make([]string, 0, len(samples))
	for n := range samples {
		names = append(names, n)
	}
	sort.Strings(names)
	checked, bad := 0, 0
	for _, name := range names {
		ss := samples[name]
		var noidx, canon, broken, thin []string
		for _, s := range ss {
			checked++
			switch {
			case s.status != 200:
				broken = append(broken, fmt.Sprintf("%s (%d)", pathOf(s.url), s.status))
			case s.noindex != "":
				noidx = append(noidx, fmt.Sprintf("%s (%s)", pathOf(s.url), s.noindex))
			case s.canonical != "":
				canon = append(canon, fmt.Sprintf("%s → %s", pathOf(s.url), s.canonical))
			}
			if s.status == 200 && s.words < 250 {
				thin = append(thin, pathOf(s.url))
			}
		}
		if len(noidx) > 0 {
			bad++
			r.add(a, fail, "Sampled pages noindexed", "%d of %d sampled %s pages tell Google not to index them: %s", len(noidx), len(ss), name, strings.Join(noidx, ", "))
		}
		if len(canon) > 0 {
			bad++
			r.add(a, warn, "Sampled canonicals", "%d of %d sampled %s pages name a different URL as canonical, so Google indexes that one instead: %s", len(canon), len(ss), name, strings.Join(canon, ", "))
		}
		if len(broken) > 0 {
			bad++
			r.add(a, warn, "Sampled pages broken", "sitemap URLs in %s don't return 200: %s", name, strings.Join(broken, ", "))
		}
		if len(thin) == len(ss) && len(ss) > 0 {
			r.add(a, warn, "Thin section", "every sampled %s page has under 250 words of text (%s); thin templated pages in bulk are a common reason Google stops indexing a site", name, strings.Join(thin, ", "))
		}
		if unsigned[name] {
			lvl := warn
			for _, b := range burst {
				if b.name == name {
					lvl = fail
				}
			}
			r.add(a, lvl, "Health pages without an author", "none of the sampled %s pages, which cover health topics, names a person who wrote or reviewed them. Google holds health content to the highest bar (\"Your Money or Your Life\"): show a named writer and a qualified reviewer, each with a page about them", name)
		}
	}
	if checked > 0 && bad == 0 {
		r.add(a, pass, "Sampled pages indexable", "%d pages from %d sections return 200, allow indexing and are their own canonical", checked, len(samples))
	}
}

func mustDay(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

func sectionSummary(sh shape, n int) string {
	var parts []string
	for i, st := range sh.sections {
		if i == n {
			parts = append(parts, fmt.Sprintf("%d more sections", len(sh.sections)-n))
			break
		}
		parts = append(parts, fmt.Sprintf("%s %d", st.name, len(st.urls)))
	}
	return strings.Join(parts, ", ")
}

// spread picks n URLs evenly through a list (sitemaps are often ordered by
// date, so this samples old and new pages).
func spread(urls []string, n int) []string {
	if len(urls) <= n {
		return urls
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, urls[(2*i+1)*len(urls)/(2*n)])
	}
	return out
}

func samplePage(ctx context.Context, f *fetcher, u string) sample {
	s := sample{url: u}
	p, err := f.get(ctx, u, "")
	if err != nil {
		return s
	}
	s.status = p.status
	if p.status != 200 {
		return s
	}
	d := parseDoc(p.body)
	if xr := p.header.Get("X-Robots-Tag"); strings.Contains(strings.ToLower(xr), "noindex") {
		s.noindex = "X-Robots-Tag header"
	} else if m, ok := d.meta("robots"); ok && strings.Contains(strings.ToLower(m), "noindex") {
		s.noindex = "meta robots"
	} else if m, ok := d.meta("googlebot"); ok && strings.Contains(strings.ToLower(m), "noindex") {
		s.noindex = "meta googlebot"
	}
	if c := d.links("canonical"); len(c) > 0 {
		if cu, err := p.final.Parse(c[0].attr("href")); err == nil && !sameURL(cu, p.final) {
			s.canonical = cu.String()
		}
	}
	for _, l := range d.links("alternate") {
		if hl := l.attr("hreflang"); hl != "" && !strings.EqualFold(hl, "x-default") {
			if au, err := p.final.Parse(l.attr("href")); err == nil && !sameURL(au, p.final) {
				s.hreflang = append(s.hreflang, au.String())
			}
		}
	}
	text := visibleText(p.body)
	s.words = len(wordRe.FindAllString(text, -1))
	title, _ := d.title()
	h1 := ""
	if h, ok := d.first("h1"); ok {
		h1 = h.text
	}
	desc, _ := d.meta("description")
	s.health = len(healthRe.FindAllString(title+" "+h1+" "+desc, -1)) > 0
	s.person = pagePerson(d, text)
	return s
}

// pagePerson returns a person named as the page's author or reviewer: in
// JSON-LD (a Person, not the site's Organization), a meta author tag, or a
// visible "By Jane Doe" line.
func pagePerson(d *doc, text string) string {
	orgs := map[string]bool{}
	var objs []map[string]any
	for _, s := range d.all("script") {
		if !strings.EqualFold(strings.TrimSpace(s.attr("type")), "application/ld+json") {
			continue
		}
		var v any
		if json.Unmarshal([]byte(strings.TrimSpace(s.text)), &v) == nil {
			objs = append(objs, ldObjects(v)...)
		}
	}
	for _, o := range objs {
		if ldHasType(o, "Organization", "WebSite", "Corporation", "NewsMediaOrganization") {
			if n, _ := o["name"].(string); n != "" {
				orgs[strings.ToLower(n)] = true
			}
		}
	}
	for _, o := range objs {
		for _, k := range []string{"author", "reviewedBy", "editor"} {
			for _, au := range ldAuthors(o[k]) {
				if au.name != "" && !orgs[strings.ToLower(au.name)] {
					if m, ok := o[k].(map[string]any); ok && ldHasType(m, "Organization") {
						continue
					}
					return au.name
				}
			}
		}
	}
	if m, ok := d.meta("author"); ok && m != "" && !orgs[strings.ToLower(m)] {
		return m
	}
	for _, m := range byRe.FindAllStringSubmatch(text, -1) {
		if !byStopword[strings.Fields(m[1])[0]] {
			return m[1]
		}
	}
	return ""
}

// checkAlternates fetches a few hreflang alternates of one sampled page: a
// translation that is noindexed, broken or canonicalised to another language
// won't be indexed.
func checkAlternates(ctx context.Context, r *report, f *fetcher, samples map[string][]sample) {
	const a = "sitewide"
	var from sample
	var langs []string
	for _, ss := range samples {
		for _, s := range ss {
			if len(s.hreflang) > len(langs) {
				from, langs = s, s.hreflang
			}
		}
	}
	if len(langs) == 0 {
		return
	}
	if len(langs) > maxAlternates {
		langs = langs[:maxAlternates]
	}
	var problems []string
	for _, u := range langs {
		s := samplePage(ctx, f, u)
		switch {
		case s.status != 200:
			problems = append(problems, fmt.Sprintf("%s returns %d", pathOf(u), s.status))
		case s.noindex != "":
			problems = append(problems, fmt.Sprintf("%s is noindex (%s)", pathOf(u), s.noindex))
		case s.canonical != "":
			problems = append(problems, fmt.Sprintf("%s names %s as canonical, so Google won't index the translation", pathOf(u), s.canonical))
		}
	}
	if len(problems) > 0 {
		r.add(a, warn, "Translations", "hreflang alternates of %s: %s", pathOf(from.url), strings.Join(problems, "; "))
		return
	}
	r.add(a, pass, "Translations", "%d hreflang alternates of %s return 200, allow indexing and are their own canonical", len(langs), pathOf(from.url))
}
