package main

import (
	"fmt"
	"regexp"
	"strings"
)

// Does the page answer its own questions? Assistants quote the passage under
// a heading, so a question heading followed by a direct, self-contained
// answer is quotable, and one followed by throat-clearing isn't. The GEO
// results fit this, but no study isolates it, and Google says no AI-specific
// writing is needed: findings stay WARN at most.

var (
	headingBlockRe = regexp.MustCompile(`(?is)<h([1-6])\b[^>]*>(.*?)</h[1-6]\s*>`)
	// Openers that delay the answer or lean on text elsewhere on the page.
	throatClearRe = regexp.MustCompile(`(?i)^(as (we |i )?(mentioned|noted|discussed|said)|as you (may )?know|in today'?s|in this (day and age|section|article|post|guide)|great question|good question|before (we|you) (dive|get|start|begin)|let'?s (take a (closer )?look|dive|explore|talk|start|break)|when it comes to|there are (many|several|a lot of|lots of|a few) (things|factors|reasons|ways)|we'?ll (cover|explore|look|walk|discuss)|you (might|may) (be wondering|have heard)|have you ever|it'?s no secret|if you'?re like (most|many)|whether you'?re|navigating|in the (ever|world of))`)
	directStartRe = regexp.MustCompile(`(?i)^(yes|no|not|usually|generally|typically|mostly|most|it depends on|about|around|roughly|under|over|up to|at least|\d|\$|€|£)`)
	answerStop    = map[string]bool{"what": true, "which": true, "when": true, "where": true, "does": true, "should": true, "could": true, "would": true, "with": true, "your": true, "this": true, "that": true, "have": true, "from": true, "they": true, "them": true, "there": true, "their": true, "much": true, "many": true, "best": true, "need": true, "about": true, "into": true, "work": true, "good": true, "will": true, "make": true, "more": true, "than": true, "really": true}
)

type headedBlock struct {
	level int
	head  string
	text  string // text between this heading and the next one
	para  string // the first paragraph of 12+ words in it, if any
}

var (
	paraRe = regexp.MustCompile(`(?is)<p\b[^>]*>(.*?)</p\s*>`)
	// Navigation and controls aren't answers: menus, forms and sidebars.
	// Buttons stay, since FAQ accordions put the question inside one.
	chromeRe   = regexp.MustCompile(`(?is)<(nav|form|select|aside|footer)\b.*?</(nav|form|select|aside|footer)\s*>`)
	sentenceRe = regexp.MustCompile(`[\p{L}\p{N})”"'’][.!?:](\s|$)`)
)

// lead is what a reader meets first under the heading: the first real
// paragraph (skipping bylines and dates), else the text as a whole when it
// reads as prose (has a sentence in its first 40 words), else nothing.
func (b headedBlock) lead() string {
	if b.para != "" {
		return b.para
	}
	if sentenceRe.MatchString(firstWords(b.text, 40)) {
		return b.text
	}
	return ""
}

// headedBlocks splits the server HTML at headings.
func headedBlocks(html string) []headedBlock {
	if m := bodyRe.FindStringSubmatch(html); m != nil {
		html = m[1]
	}
	html = scriptStyleRe.ReplaceAllString(html, " ")
	html = chromeRe.ReplaceAllString(html, " ")
	ms := headingBlockRe.FindAllStringSubmatchIndex(html, -1)
	var out []headedBlock
	for i, m := range ms {
		end := len(html)
		if i+1 < len(ms) {
			end = ms[i+1][0]
		}
		b := headedBlock{
			level: int(html[m[2]] - '0'),
			head:  cleanText(stripTags(html[m[4]:m[5]])),
			text:  cleanText(stripTags(html[m[1]:end])),
		}
		for _, p := range paraRe.FindAllStringSubmatch(html[m[1]:end], 8) {
			t := cleanText(stripTags(p[1]))
			if len(wordRe.FindAllString(t, -1)) >= 12 && !reviewerRe.MatchString(t) && !byRe.MatchString(firstWords(t, 3)) {
				b.para = t
				break
			}
		}
		out = append(out, b)
	}
	return out
}

func firstWords(s string, n int) string {
	w := strings.Fields(s)
	if len(w) > n {
		w = w[:n]
	}
	return strings.Join(w, " ")
}

// directAnswer says whether text answers question in its opening, and if not,
// why not.
func directAnswer(question, text string) (bool, string) {
	open := firstWords(text, 50)
	if len(wordRe.FindAllString(open, -1)) < 8 {
		return false, "almost no text under it"
	}
	if m := throatClearRe.FindString(open); m != "" {
		return false, fmt.Sprintf("opens with %q", m)
	}
	if directStartRe.MatchString(open) {
		return true, ""
	}
	// The answer should name what the question asks about.
	lower := strings.ToLower(open)
	var keys []string
	for _, w := range wordRe.FindAllString(strings.ToLower(question), -1) {
		if len([]rune(w)) < 4 || answerStop[w] || questionStart.MatchString(w) {
			continue
		}
		keys = append(keys, w)
		stem := w
		if r := []rune(w); len(r) > 5 {
			stem = string(r[:5])
		}
		if strings.Contains(lower, stem) {
			return true, ""
		}
	}
	if len(keys) == 0 {
		return true, ""
	}
	return false, fmt.Sprintf("the first 50 words don't mention %q", strings.Join(keys[:min(2, len(keys))], " "))
}

func isQuestion(t string) bool {
	t = strings.TrimSpace(t)
	if strings.HasSuffix(t, ":") || strings.HasSuffix(t, ".") || strings.HasSuffix(t, "!") {
		return false // a label or a statement ("Where the world transacts onchain.")
	}
	return strings.HasSuffix(t, "?") || questionStart.MatchString(t)
}

func checkAnswers(r *report, pg *page, d *doc, words int) {
	const a = "aeo"
	blocks := headedBlocks(pg.body)

	// Opening: the text between the h1 and the first subheading.
	for _, b := range blocks {
		if b.level != 1 {
			continue
		}
		open := firstWords(b.lead(), 30)
		if words >= 300 {
			switch {
			case len(wordRe.FindAllString(open, -1)) < 8:
				r.add(a, info, "Opening paragraph", "almost no text between the headline and the first subheading; a two-sentence answer there is what assistants and featured snippets quote")
			case throatClearRe.MatchString(open):
				r.add(a, warn, "Opening paragraph", "opens with %q; lead with the answer to the headline instead (\"%s…\")", throatClearRe.FindString(open), open)
			default:
				r.add(a, info, "Opening paragraph", "first words under the headline: \"%s…\" (does this answer the headline on its own?)", open)
			}
		}
		break
	}

	var asked, answered int
	var misses []string
	for _, b := range blocks {
		if b.level == 1 || !isQuestion(b.head) {
			continue
		}
		asked++
		if ok, why := directAnswer(b.head, b.lead()); ok {
			answered++
		} else if len(misses) < 3 {
			misses = append(misses, fmt.Sprintf("%q: %s", truncate(b.head, 50), why))
		}
	}
	switch {
	case asked == 0:
		return
	case answered == asked:
		r.add(a, pass, "Answers under questions", "all %d question headings are followed by a direct answer", asked)
	case asked >= 2 && 2*answered < asked:
		r.add(a, warn, "Answers under questions", "%d of %d question headings are followed by a direct answer. Assistants quote the passage under a heading, so answer in its first sentence; e.g. %s", answered, asked, strings.Join(misses, "; "))
	default:
		r.add(a, info, "Answers under questions", "%d of %d question headings are followed by a direct answer; e.g. %s", answered, asked, strings.Join(misses, "; "))
	}
}
