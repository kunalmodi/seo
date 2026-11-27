package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

// fakeAssistants serves canned replies in each API's shape and records the
// request bodies.
func fakeAssistants(t *testing.T, replies map[string]string) map[string]*[]map[string]any {
	t.Helper()
	seen := map[string]*[]map[string]any{}
	mux := http.NewServeMux()
	for path, reply := range replies {
		reply, log := reply, &[]map[string]any{}
		seen[path] = log
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("x-api-key")+r.Header.Get("Authorization")+r.Header.Get("x-goog-api-key") == "" {
				w.WriteHeader(401)
				return
			}
			var body map[string]any
			data, _ := io.ReadAll(r.Body)
			json.Unmarshal(data, &body)
			*log = append(*log, body)
			if strings.HasPrefix(reply, "!") {
				w.WriteHeader(404)
				reply = reply[1:]
			}
			io.WriteString(w, reply)
		})
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	old := []string{anthropicURL, openaiURL, xaiURL, perplexityURL, geminiURL}
	anthropicURL, openaiURL, xaiURL, perplexityURL, geminiURL = srv.URL+"/claude", srv.URL+"/openai", srv.URL+"/xai", srv.URL+"/pplx", srv.URL+"/gemini"
	t.Cleanup(func() {
		anthropicURL, openaiURL, xaiURL, perplexityURL, geminiURL = old[0], old[1], old[2], old[3], old[4]
	})
	for _, e := range askEngines {
		for _, k := range e.keyEnv {
			t.Setenv(k, "")
		}
		t.Setenv(e.modelEnv, "")
	}
	return seen
}

const claudeReply = `{"stop_reason":"end_turn","content":[
 {"type":"text","text":"I'll look."},
 {"type":"server_tool_use","name":"web_search","input":{"query":"glp-1 electrolytes"}},
 {"type":"web_search_tool_result","content":[{"type":"web_search_result","url":"https://www.shop.example/guide/","title":"G"},{"type":"web_search_result","url":"https://rival.com/a"}]},
 {"type":"server_tool_use","name":"web_search","input":{"query":"best electrolyte drink 2026"}},
 {"type":"web_search_tool_result","content":{"type":"web_search_tool_result_error","error_code":"max_uses_exceeded"}},
 {"type":"text","text":"ShopExample recommends sodium first.","citations":[{"type":"web_search_result_location","url":"https://shop.example/guide","title":"G"}]},
 {"type":"text","text":" Rival agrees.","citations":[{"url":"https://rival.com/a"}]}]}`

const openaiReply = `{"output":[
 {"type":"reasoning"},
 {"type":"web_search_call","action":{"type":"search","query":"electrolytes glp-1","sources":[{"type":"url","url":"https://rival.com/a"},{"type":"url","url":"https://blog.shop.example/x"}]}},
 {"type":"message","content":[{"type":"output_text","text":"Try Rival.","annotations":[{"type":"url_citation","url":"https://rival.com/a"},{"type":"url_citation","url":"https://other.org/z"}]}]}],
 "usage":{"cost_in_usd_ticks":2500000000}}`

const pplxReply = `{"output":[
 {"type":"search_results","queries":["glp-1 hydration"],"results":[{"id":1,"url":"https://rival.com/a"},{"id":2,"url":"https://shop.example/guide/"}]},
 {"type":"message","content":[{"type":"output_text","text":"Sodium matters [web:2]. See also https://third.net/page.","annotations":[]}]}]}`

const geminiReply = `{"steps":[
 {"type":"thought"},
 {"type":"google_search_call","arguments":{"queries":["electrolytes on glp-1","sodium glp-1"]}},
 {"type":"model_output","content":[{"type":"text","text":"Rival and others.","annotations":[
   {"type":"url_citation","url":"https://vertexaisearch.cloud.google.com/grounding-api-redirect/abc","title":"rival.com"},
   {"type":"url_citation","url":"https://vertexaisearch.cloud.google.com/grounding-api-redirect/def","title":"shop.example"}]}]}]}`

func TestAskParsesEachAPI(t *testing.T) {
	seen := fakeAssistants(t, map[string]string{"/claude": claudeReply, "/openai": openaiReply, "/xai": openaiReply, "/pplx": pplxReply, "/gemini": geminiReply})
	ctx := context.Background()
	byID := map[string]*askEngine{}
	for _, e := range askEngines {
		byID[e.id] = e
	}

	a, err := askClaude(ctx, byID["claude"], "k", "m", "q?")
	if err != nil || len(a.Queries) != 2 || len(a.Sources) != 2 || len(a.Cited) != 2 || !strings.Contains(a.Text, "Rival agrees") {
		t.Errorf("claude: %+v %v", a, err)
	}
	tool := (*seen["/claude"])[0]["tools"].([]any)[0].(map[string]any)
	if tool["type"] != "web_search_20260318" || tool["allowed_callers"].([]any)[0] != "direct" {
		t.Errorf("claude tool: %+v", tool)
	}

	a, err = askResponses(ctx, byID["chatgpt"], "k", "m", "q?")
	if err != nil || len(a.Queries) != 1 || len(a.Sources) != 2 || len(a.Cited) != 2 || a.Cost != 0.25 {
		t.Errorf("chatgpt: %+v %v", a, err)
	}
	if (*seen["/openai"])[0]["include"] == nil || (*seen["/xai"]) != nil && len(*seen["/xai"]) != 0 {
		t.Errorf("only OpenAI is asked to include sources")
	}
	if a, err = askResponses(ctx, byID["grok"], "k", "m", "q?"); err != nil || (*seen["/xai"])[0]["include"] != nil || len(a.Cited) != 2 {
		t.Errorf("grok: %+v %v", a, err)
	}

	a, err = askResponses(ctx, byID["perplexity"], "k", "m", "q?")
	if err != nil || len(a.Queries) != 1 || len(a.Cited) != 1 || a.Cited[0] != "https://shop.example/guide/" {
		t.Errorf("perplexity [web:N] markers: %+v %v", a, err)
	}

	a, err = askGemini(ctx, byID["gemini"], "k", "m", "q?")
	if err != nil || len(a.Queries) != 2 || len(a.Cited) != 2 || a.Cited[1] != "https://shop.example/" {
		t.Errorf("gemini: %+v %v", a, err)
	}
}

func TestTally(t *testing.T) {
	brands := brandNames("shop.example", "Shop Example")
	answers := []*answer{
		{Queries: []string{"a", "B"}, Sources: []string{"https://shop.example/guide"}, Cited: []string{"https://www.shop.example/guide/", "https://rival.com/x"}, Text: "Shop Example says so."},
		{Queries: []string{"b", "c"}, Sources: []string{"https://blog.shop.example/p"}, Cited: []string{"https://rival.com/y"}, Text: "See [other](https://other.org/z)."},
		{Cited: []string{"https://rival.com/y"}, Text: "The shop.example guide is good.", Cost: 0.1},
		{Err: "HTTP 500: boom"},
	}
	got := tally("X", "m", "shop.example", brands, answers)
	if got.Answers != 3 || got.Cited != 1 || got.Retrieved != 2 || got.Named != 2 || got.Cost != 0.1 || len(got.Errors) != 1 {
		t.Errorf("%+v", got)
	}
	if strings.Join(got.Searches, ",") != "a,B,c" {
		t.Errorf("searches deduped ignoring case: %v", got.Searches)
	}
	if s := topCounts(got.Others, 5); s != "rival.com (3), other.org (1)" {
		t.Errorf("cited instead: %s", s)
	}
	// A short ordinary word isn't a brand mention.
	for _, b := range brandNames("base.org", "Base") {
		if b.MatchString("our customer base grew") {
			t.Errorf("%v matched an ordinary word", b)
		}
	}
	if !brandNames("scriptroom.app", "")[1].MatchString("Try Scriptroom, it's free") {
		t.Error("a distinctive host label is a brand name")
	}
}

func TestCheckAsk(t *testing.T) {
	seen := fakeAssistants(t, map[string]string{"/claude": claudeReply, "/openai": openaiReply, "/gemini": `!{"error":{"message":"model not found, key k9secret"}}`})
	target, _ := url.Parse("https://shop.example/guide")
	pg := &page{final: target}
	d := parseDoc(`<html><head><title>Electrolytes on GLP-1</title></head></html>`)

	r := &report{}
	checkAsk(context.Background(), r, pg, d, []string{"q1"}, 2, nil, "")
	if g := findings(r, "aeo")["AI answers"]; g.Level != warn || !strings.Contains(g.Detail, "ANTHROPIC_API_KEY") {
		t.Errorf("no keys: %+v", g)
	}

	t.Setenv("ANTHROPIC_API_KEY", "k")
	t.Setenv("OPENAI_API_KEY", "k")
	t.Setenv("GEMINI_API_KEY", "k9secret")
	dir := t.TempDir()
	r = &report{}
	checkAsk(context.Background(), r, pg, d, []string{"q1", "q2"}, 2, nil, dir)
	got := findings(r, "aeo")
	if g := got["AI answers: Claude"]; g.Level != pass || !strings.Contains(g.Detail, "cited in 4 of 4 answers, retrieved in 4, named in 0") || !strings.Contains(g.Detail, "rival.com (4)") || !strings.Contains(g.Detail, `"glp-1 electrolytes"`) {
		t.Errorf("claude: %+v", g)
	}
	if g := got["AI answers: ChatGPT"]; g.Level != info || !strings.Contains(g.Detail, "cited in 0 of 4 answers, retrieved in 4") {
		t.Errorf("chatgpt (retrieved via a subdomain, not cited): %+v", g)
	}
	if g := got["AI answers: Gemini"]; g.Level != warn || !strings.Contains(g.Detail, "HTTP 404: model not found") || strings.Contains(g.Detail, "k9secret") || !strings.Contains(g.Detail, "SITECHECK_GEMINI_MODEL") {
		t.Errorf("gemini error, key redacted: %+v", g)
	}
	if n := len(*seen["/gemini"]); n != 1 {
		t.Errorf("a 4xx should stop that assistant after one call, got %d", n)
	}
	if g := got["AI answers"]; !strings.Contains(g.Detail, "$1.00") || !strings.Contains(g.Detail, "differ from the apps") {
		t.Errorf("note: %+v", g)
	}
	md, err := os.ReadFile(r.Files["ai_answers"])
	if err != nil || !strings.Contains(string(md), "## Claude (claude-opus-5-5): q1") || !strings.Contains(string(md), "Cited: rival.com, shop.example") {
		t.Errorf("answers.md: %v\n%s", err, md)
	}

	// -ask-engines narrows it; never cited or retrieved with given questions is a WARN.
	r = &report{}
	pg2 := &page{final: &url.URL{Scheme: "https", Host: "nobody.example", Path: "/"}}
	checkAsk(context.Background(), r, pg2, d, []string{"q1"}, 1, []string{"chatgpt"}, "")
	got = findings(r, "aeo")
	if _, ok := got["AI answers: Claude"]; ok || got["AI answers: ChatGPT"].Level != warn {
		t.Errorf("only chatgpt, as a warning: %+v", got)
	}
}
