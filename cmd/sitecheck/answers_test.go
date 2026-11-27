package main

import (
	"context"
	"strings"
	"testing"
)

func TestDirectAnswer(t *testing.T) {
	cases := []struct {
		q, text string
		want    bool
	}{
		{"How much protein do GLP-1 users need?", "Most adults on GLP-1 medicines should aim for 1.2 to 1.6 g of protein per kg of body weight a day, spread across meals.", true},
		{"Is SoWell sugar free?", "Yes. Each stick has 0 g of sugar and is sweetened with stevia, so it fits a low-sugar plan.", true},
		{"How should you store pens when travelling?", "Keep unopened pens refrigerated between 2 and 8°C; a travel case with an ice pack keeps them in range for a day.", true},
		{"What are the best electrolyte drinks?", "When it comes to staying healthy, there are many things to consider and everyone is different in their own way.", false},
		{"Why does nausea happen?", "As mentioned above, the first weeks can be hard for many people starting out on a new routine like this one.", false},
		{"Which travel case keeps pens cold?", "Click here to see our full list of picks below.", false},
		{"Which travel case keeps pens cold?", "", false},
	}
	for _, c := range cases {
		if got, why := directAnswer(c.q, c.text); got != c.want {
			t.Errorf("directAnswer(%q) = %v (%s), want %v", c.q, got, why, c.want)
		}
	}
}

func answersFor(html string, words int) map[string]finding {
	pg := &page{body: html}
	r := &report{}
	checkAnswers(r, pg, parseDoc(html), words)
	return findings(r, "aeo")
}

func TestCheckAnswers(t *testing.T) {
	good := `<html><body><h1>GLP-1 electrolytes</h1><p>Electrolyte drinks replace the sodium and potassium you lose when GLP-1 side effects cut how much you eat and drink.</p>
<h2>Is SoWell sugar free?</h2><p>Yes, every stick has 0 g of sugar and is sweetened with stevia instead.</p>
<h2>How many sticks a day?</h2><p>One or two sticks a day covers most people; more only on hot days or after vomiting.</p></body></html>`
	got := answersFor(good, 400)
	if g := got["Answers under questions"]; g.Level != pass {
		t.Errorf("good page: %+v", g)
	}
	if g := got["Opening paragraph"]; g.Level != info || !strings.Contains(g.Detail, "Electrolyte drinks replace") {
		t.Errorf("opening: %+v", g)
	}

	bad := `<html><body><h1>GLP-1 electrolytes</h1><p>In today's world, staying hydrated matters more than ever for everyone, whatever your goals are this year.</p>
<h2>What is the best electrolyte drink?</h2><p>When it comes to drinks, there are many things to weigh up and your needs are your own.</p>
<h2>Why do GLP-1 users get dehydrated?</h2><ul><li>See below</li></ul>
<h2>Our picks</h2><p>Lots of products.</p></body></html>`
	got = answersFor(bad, 400)
	if g := got["Answers under questions"]; g.Level != warn || !strings.Contains(g.Detail, "0 of 2") {
		t.Errorf("bad page: %+v", g)
	}
	if g := got["Opening paragraph"]; g.Level != warn || !strings.Contains(g.Detail, "In today") {
		t.Errorf("throat-clearing opening: %+v", g)
	}
}

func TestOpeningSkipsBylineLine(t *testing.T) {
	html := `<html><body><h1>How much protein per day?</h1><p>Medically reviewed by Jerlyn Jones, RDN — Written by Tom Rush — Updated July 16, 2026</p>
<p>Most adults need at least 0.8 g of protein per kg of body weight a day, and more when losing weight or training.</p>
<h2>How we reviewed this article:</h2><p>Sources.</p></body></html>`
	got := answersFor(html, 400)
	if g := got["Opening paragraph"]; !strings.Contains(g.Detail, "Most adults need") {
		t.Errorf("opening should skip the byline line: %+v", g)
	}
	if _, ok := got["Answers under questions"]; ok {
		t.Errorf("a heading ending in a colon is a label, not a question")
	}
}

func TestReviewerAndEditorialOnHealthGuide(t *testing.T) {
	// A health guide that isn't marked up as an article still gets checked.
	html := `<html><head><title>How to store GLP-1 pens when travelling</title></head><body>
<nav><a href="/about/">About us</a><a href="/editorial-policy/">Editorial policy</a></nav>
<h1>How to store GLP-1 pens when travelling</h1><p>By Ana Ruiz · Medically reviewed by Dr. Sam Lee, PharmD</p>` + longBody + `</body></html>`
	got := runByline(t, html, "")
	if g := got["Byline"]; g.Level != pass {
		t.Errorf("byline on a non-article health page: %+v", g)
	}
	if g := got["Reviewer"]; g.Level != pass || !strings.Contains(g.Detail, "Sam Lee, PharmD") {
		t.Errorf("reviewer: %+v", g)
	}
	if g := got["Editorial policy"]; g.Level != pass {
		t.Errorf("editorial: %+v", g)
	}

	bare := `<html><head><title>Best mortgage rates for first-time buyers</title></head><body><h1>Best mortgage rates</h1>` + longBody + `</body></html>`
	got = runByline(t, bare, "")
	if g := got["Byline"]; g.Level != warn {
		t.Errorf("money page without byline: %+v", g)
	}
	if g := got["Reviewer"]; g.Level != info || !strings.Contains(g.Detail, "CFP") {
		t.Errorf("money reviewer hint: %+v", g)
	}
	if g := got["Editorial policy"]; g.Level != warn {
		t.Errorf("no about or editorial link: %+v", g)
	}

	// Non-YMYL, non-article pages stay unchecked.
	got = runByline(t, `<html><head><title>Our favourite hiking boots</title></head><body><h1>Boots</h1>`+longBody+`</body></html>`, "")
	if _, ok := got["Byline"]; ok {
		t.Errorf("byline check should skip ordinary non-article pages: %+v", got)
	}
	_ = context.Background

	if got := trimRepeatedName("Michael Soon Lee Michael"); got != "Michael Soon Lee" {
		t.Errorf("trimRepeatedName = %q", got)
	}
	got = runByline(t, `<html><head><meta property="og:type" content="article"><title>T</title></head><body><h1>T</h1><p>Updated by December 6</p>`+longBody+`</body></html>`, "")
	if g := got["Byline"]; g.Level != warn {
		t.Errorf("a month is not an author: %+v", g)
	}
}
