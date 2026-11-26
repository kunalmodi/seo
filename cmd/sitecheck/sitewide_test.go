package main

import (
	"fmt"
	"strings"
	"testing"
)

func entriesFor(sec string, n int, day func(i int) string) []sitemapEntry {
	var out []sitemapEntry
	for i := 0; i < n; i++ {
		out = append(out, sitemapEntry{loc: fmt.Sprintf("https://ex.com%s%d/", sec, i), lastmod: day(i)})
	}
	return out
}

func spreadDays(start string) func(int) string {
	t := mustDay(start)
	return func(i int) string { return t.AddDate(0, 0, i%40).Format("2006-01-02") }
}

func findingsOf(r *report, check string) []finding {
	var out []finding
	for _, f := range r.Findings {
		if f.Check == check {
			out = append(out, f)
		}
	}
	return out
}

func TestSection(t *testing.T) {
	for in, want := range map[string]string{
		"https://ex.com/":               "/",
		"https://ex.com/about/":         "/",
		"https://ex.com/research/a-b/":  "/research/",
		"https://ex.com/es/guides/x/":   "/es/",
		"https://ex.com/products/x?y=1": "/products/",
	} {
		if got := section(in); got != want {
			t.Errorf("section(%s) = %s, want %s", in, got, want)
		}
	}
}

func TestBurstFlagsOneSectionNotBuildStamps(t *testing.T) {
	now := mustDay("2026-11-26")
	// glpfinds-like: a young site, then 500 research pages on one day.
	var es []sitemapEntry
	es = append(es, entriesFor("/products/", 130, spreadDays("2026-10-29"))...)
	es = append(es, entriesFor("/research/", 500, func(int) string { return "2026-11-21T10:00:00Z" })...)
	sh := analyzeShape(es)
	b := sh.bursts(now)
	if len(b) != 1 || b[0].name != "/research/" || b[0].topCount != 500 {
		t.Fatalf("want one /research/ burst, got %+v", b)
	}
	r := &report{}
	reportSitewide(r, sh, map[string][]sample{
		"/research/": {{url: "https://ex.com/research/1/", status: 200, words: 1500, health: true}},
	}, now)
	got := findingsOf(r, "Publishing burst")
	if len(got) != 1 || got[0].Level != fail || !strings.Contains(got[0].Detail, "oldest lastmod is 2026-10-29") || !strings.Contains(got[0].Detail, "no named author") {
		t.Fatalf("burst finding: %+v", got)
	}
	if h := findingsOf(r, "Health pages without an author"); len(h) != 1 || h[0].Level != fail {
		t.Fatalf("authorless health pages in a burst should FAIL: %+v", h)
	}

	// Every URL with the same lastmod is a build timestamp, not a burst.
	same := entriesFor("/blog/", 300, func(int) string { return "2026-11-20" })
	same = append(same, entriesFor("/docs/", 100, func(int) string { return "2026-11-20" })...)
	sh = analyzeShape(same)
	if b := sh.bursts(now); len(b) != 0 {
		t.Fatalf("build stamps should not count as a burst: %+v", b)
	}
	r = &report{}
	reportSitewide(r, sh, nil, now)
	if len(findingsOf(r, "Sitemap lastmod")) != 1 {
		t.Fatalf("expected the build-timestamp note: %+v", r.Findings)
	}

	// An old burst is history, not news.
	sh = analyzeShape(append(entriesFor("/a/", 100, spreadDays("2023-01-01")), entriesFor("/b/", 300, func(int) string { return "2024-01-01" })...))
	if b := sh.bursts(now); len(b) != 0 {
		t.Fatalf("a burst over a year old should be ignored: %+v", b)
	}
}

func TestSampledIndexability(t *testing.T) {
	now := mustDay("2026-11-26")
	sh := analyzeShape(entriesFor("/p/", 40, spreadDays("2024-01-01")))
	r := &report{}
	reportSitewide(r, sh, map[string][]sample{
		"/p/": {
			{url: "https://ex.com/p/1/", status: 200, words: 600, noindex: "X-Robots-Tag header"},
			{url: "https://ex.com/p/2/", status: 200, words: 600, canonical: "https://ex.com/"},
			{url: "https://ex.com/p/3/", status: 404},
		},
	}, now)
	for check, lvl := range map[string]level{"Sampled pages noindexed": fail, "Sampled canonicals": warn, "Sampled pages broken": warn} {
		if f := findingsOf(r, check); len(f) != 1 || f[0].Level != lvl {
			t.Errorf("%s: %+v", check, f)
		}
	}
	r = &report{}
	reportSitewide(r, sh, map[string][]sample{"/p/": {{url: "https://ex.com/p/1/", status: 200, words: 600, person: "Ana Ruiz", health: true}}}, now)
	if len(findingsOf(r, "Sampled pages indexable")) != 1 || len(findingsOf(r, "Health pages without an author")) != 0 {
		t.Fatalf("clean sample: %+v", r.Findings)
	}
}

func TestSpread(t *testing.T) {
	u := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8"}
	if got := strings.Join(spread(u, 3), ","); got != "1,4,7" {
		t.Fatalf("spread = %s", got)
	}
	if len(spread(u[:2], 3)) != 2 {
		t.Fatal("short lists are returned whole")
	}
}

func TestPagePerson(t *testing.T) {
	org := `<script type="application/ld+json">{"@graph":[{"@type":"Organization","@id":"https://ex.com/#org","name":"GLP Finds"},{"@type":"Article","author":{"@id":"https://ex.com/#org"}}]}</script>`
	if p := pagePerson(parseDoc(org), "Updated November"); p != "" {
		t.Fatalf("an organization is not a person: %q", p)
	}
	orgNamed := `<script type="application/ld+json">{"@graph":[{"@type":"Organization","name":"GLP Finds"},{"@type":"Article","author":{"@type":"Organization","name":"GLP Finds"}}]}</script>`
	if p := pagePerson(parseDoc(orgNamed), ""); p != "" {
		t.Fatalf("an organization author is not a person: %q", p)
	}
	if p := pagePerson(parseDoc(articleLD), ""); p != "Ana Ruiz" {
		t.Fatalf("Person author: %q", p)
	}
	if p := pagePerson(parseDoc(`<p>x</p>`), "Kidney research. Reviewed by Dr. Sam Lee, MD"); p != "Dr. Sam Lee" {
		t.Fatalf("visible byline: %q", p)
	}
}

func TestBurstIgnoresSmallSectionsAndBadYears(t *testing.T) {
	now := mustDay("2026-11-26")
	es := entriesFor("/a/", 900, spreadDays("2020-01-01"))
	es = append(es, entriesFor("/uk/", 114, func(int) string { return "2026-03-03" })...)
	es = append(es, sitemapEntry{loc: "https://ex.com/x/y/", lastmod: "0023-04-10"})
	sh := analyzeShape(es)
	if b := sh.bursts(now); len(b) != 0 {
		t.Fatalf("an 11%% section is not a site-wide burst: %+v", b)
	}
	if sh.oldest != "2020-01-01" {
		t.Fatalf("oldest = %s; year 0023 should be ignored", sh.oldest)
	}
	r := &report{}
	reportSitewide(r, sh, nil, now)
	if len(findingsOf(r, "Dominant section")) != 0 {
		t.Fatal("a large section on an established site is normal")
	}
}
