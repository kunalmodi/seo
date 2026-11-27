package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// AI answer sampling (-ask): put the page's questions to the assistants'
// APIs with web search on, a few times each, and record what each one
// searched for, which pages it retrieved, which it cited, and whether the
// site was among them.
//
// Two limits to keep in mind when reading the results. Answers vary from run
// to run, so one answer proves little: the counts matter, not any single
// reply. And the APIs aren't the consumer apps: the apps add their own system
// prompts, personalisation and sometimes a different search backend, so this
// is a proxy, the same way Brave rank is.
//
// Every call costs money on the user's own keys, so nothing here runs
// without -ask.

const (
	askMaxQuestions = 3
	askMaxSearches  = 5 // per answer, where the API lets us cap it
)

var askTimeout = 4 * time.Minute

// An answer is one reply from one assistant to one question.
type answer struct {
	Engine   string   `json:"engine"`
	Model    string   `json:"model"`
	Question string   `json:"question"`
	Queries  []string `json:"queries,omitempty"`   // searches it ran
	Sources  []string `json:"retrieved,omitempty"` // URLs its searches returned
	Cited    []string `json:"cited,omitempty"`     // URLs it cited in the answer
	Text     string   `json:"-"`
	Cost     float64  `json:"cost_usd,omitempty"` // when the API reports it
	Err      string   `json:"error,omitempty"`
}

type askEngine struct {
	id, name string   // "claude", "Claude"
	keyEnv   []string // first one set wins
	modelEnv string
	model    string
	ask      func(ctx context.Context, e *askEngine, key, model, q string) (*answer, error)
}

// Endpoints are variables so tests can point them at a fake.
var (
	anthropicURL  = "https://api.anthropic.com/v1/messages"
	openaiURL     = "https://api.openai.com/v1/responses"
	xaiURL        = "https://api.x.ai/v1/responses"
	perplexityURL = "https://api.perplexity.ai/v1/agent"
	geminiURL     = "https://generativelanguage.googleapis.com/v1beta/interactions"
)

var askEngines = []*askEngine{
	{id: "claude", name: "Claude", keyEnv: []string{"ANTHROPIC_API_KEY"}, modelEnv: "SITECHECK_CLAUDE_MODEL", model: "claude-opus-5-5", ask: askClaude},
	{id: "chatgpt", name: "ChatGPT", keyEnv: []string{"OPENAI_API_KEY"}, modelEnv: "SITECHECK_OPENAI_MODEL", model: "gpt-6.2-astra", ask: askResponses},
	{id: "gemini", name: "Gemini", keyEnv: []string{"GEMINI_API_KEY", "GOOGLE_API_KEY"}, modelEnv: "SITECHECK_GEMINI_MODEL", model: "gemini-4-hydrogen", ask: askGemini},
	{id: "perplexity", name: "Perplexity", keyEnv: []string{"PERPLEXITY_API_KEY"}, modelEnv: "SITECHECK_PERPLEXITY_MODEL", model: "perplexity/sonar", ask: askResponses},
	{id: "grok", name: "Grok", keyEnv: []string{"XAI_API_KEY", "GROK_API_KEY"}, modelEnv: "SITECHECK_GROK_MODEL", model: "grok-4.7", ask: askResponses},
}

func (e *askEngine) key() string {
	for _, k := range e.keyEnv {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

// postJSON sends body and returns the response body, turning a non-200 into
// an error that carries the API's own message and never the key.
func postJSON(ctx context.Context, url, key string, headers map[string]string, body any) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, askTimeout)
	defer cancel()
	buf, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.New(strings.ReplaceAll(err.Error(), key, "REDACTED"))
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if resp.StatusCode != 200 {
		var e struct {
			Error json.RawMessage `json:"error"`
		}
		json.Unmarshal(data, &e)
		var obj struct {
			Message string `json:"message"`
		}
		var msg string
		if json.Unmarshal(e.Error, &obj) == nil && obj.Message != "" {
			msg = obj.Message
		} else if json.Unmarshal(e.Error, &msg) != nil || msg == "" {
			msg = resp.Status
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(strings.ReplaceAll(msg, key, "REDACTED"), 200))
	}
	return data, nil
}

// askClaude uses the Messages API's web search tool, called directly (no
// dynamic filtering) so every search and its results come back as blocks.
func askClaude(ctx context.Context, e *askEngine, key, model, q string) (*answer, error) {
	a := &answer{}
	type msg struct {
		Role    string `json:"role"`
		Content any    `json:"content"`
	}
	msgs := []msg{{"user", q}}
	// A long search turn can pause; sending the paused reply back resumes it.
	for round := 0; round < 4; round++ {
		data, err := postJSON(ctx, anthropicURL, key, map[string]string{"x-api-key": key, "anthropic-version": "2026-10-21"}, map[string]any{
			"model":      model,
			"max_tokens": 16000,
			"messages":   msgs,
			"tools": []map[string]any{{
				"type": "web_search_20260318", "name": "web_search",
				"max_uses": askMaxSearches, "allowed_callers": []string{"direct"},
			}},
		})
		if err != nil {
			return nil, err
		}
		var out struct {
			StopReason string          `json:"stop_reason"`
			Content    json.RawMessage `json:"content"`
		}
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, err
		}
		var blocks []struct {
			Type  string `json:"type"`
			Name  string `json:"name"`
			Text  string `json:"text"`
			Input struct {
				Query string `json:"query"`
			} `json:"input"`
			Content   json.RawMessage `json:"content"`
			Citations []struct {
				URL string `json:"url"`
			} `json:"citations"`
		}
		json.Unmarshal(out.Content, &blocks)
		for _, b := range blocks {
			switch b.Type {
			case "server_tool_use":
				if b.Name == "web_search" && b.Input.Query != "" {
					a.Queries = append(a.Queries, b.Input.Query)
				}
			case "web_search_tool_result":
				// A list of results, or a single error object.
				var res []struct {
					URL string `json:"url"`
				}
				if json.Unmarshal(b.Content, &res) == nil {
					for _, x := range res {
						a.Sources = append(a.Sources, x.URL)
					}
				}
			case "text":
				a.Text += b.Text
				for _, c := range b.Citations {
					a.Cited = append(a.Cited, c.URL)
				}
			}
		}
		switch out.StopReason {
		case "pause_turn":
			msgs = append(msgs, msg{"assistant", out.Content})
			continue
		case "refusal":
			return nil, errors.New("the model declined to answer")
		}
		break
	}
	return a, nil
}

// askResponses covers the three APIs shaped like OpenAI's Responses API:
// OpenAI itself, xAI, and Perplexity's Agent API.
func askResponses(ctx context.Context, e *askEngine, key, model, q string) (*answer, error) {
	body := map[string]any{
		"model": model,
		"input": []map[string]string{{"role": "user", "content": q}},
		"tools": []map[string]string{{"type": "web_search"}},
	}
	url := xaiURL
	switch e.id {
	case "chatgpt":
		url = openaiURL
		body["include"] = []string{"web_search_call.action.sources"}
		body["max_tool_calls"] = askMaxSearches
	case "perplexity":
		url = perplexityURL
	default:
		body["max_tool_calls"] = askMaxSearches
	}
	data, err := postJSON(ctx, url, key, map[string]string{"Authorization": "Bearer " + key}, body)
	if err != nil {
		return nil, err
	}
	var out struct {
		Output []struct {
			Type   string `json:"type"`
			Action struct {
				Query   string   `json:"query"`
				Queries []string `json:"queries"`
				Sources []struct {
					URL string `json:"url"`
				} `json:"sources"`
			} `json:"action"`
			// Perplexity lists searches as their own item.
			Queries []string `json:"queries"`
			Results []struct {
				ID  int    `json:"id"`
				URL string `json:"url"`
			} `json:"results"`
			Content []struct {
				Text        string `json:"text"`
				Annotations []struct {
					URL string `json:"url"`
				} `json:"annotations"`
			} `json:"content"`
		} `json:"output"`
		Citations []string `json:"citations"`
		Usage     struct {
			Ticks float64 `json:"cost_in_usd_ticks"` // xAI: 1e-10 USD
			Cost  struct {
				Total float64 `json:"total_cost"`
			} `json:"cost"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	a := &answer{Sources: out.Citations, Cost: out.Usage.Ticks/1e10 + out.Usage.Cost.Total}
	byID := map[int]string{}
	for _, o := range out.Output {
		switch o.Type {
		case "web_search_call":
			if o.Action.Query != "" {
				a.Queries = append(a.Queries, o.Action.Query)
			}
			a.Queries = append(a.Queries, o.Action.Queries...)
			for _, s := range o.Action.Sources {
				a.Sources = append(a.Sources, s.URL)
			}
		case "search_results":
			a.Queries = append(a.Queries, o.Queries...)
			for _, x := range o.Results {
				a.Sources = append(a.Sources, x.URL)
				byID[x.ID] = x.URL
			}
		case "message":
			for _, c := range o.Content {
				a.Text += c.Text
				for _, an := range c.Annotations {
					if an.URL != "" {
						a.Cited = append(a.Cited, an.URL)
					}
				}
			}
		}
	}
	// Perplexity cites with [web:3] markers that point at result ids.
	for _, m := range webMarkerRe.FindAllStringSubmatch(a.Text, -1) {
		var id int
		fmt.Sscan(m[1], &id)
		if u := byID[id]; u != "" {
			a.Cited = append(a.Cited, u)
		}
	}
	return a, nil
}

var webMarkerRe = regexp.MustCompile(`\[web:(\d+)\]`)

// askGemini uses the Interactions API with Google Search grounding.
func askGemini(ctx context.Context, e *askEngine, key, model, q string) (*answer, error) {
	data, err := postJSON(ctx, geminiURL, key, map[string]string{"x-goog-api-key": key, "Api-Revision": "2026-05-20"}, map[string]any{
		"model": model, "input": q, "tools": []map[string]string{{"type": "google_search"}},
	})
	if err != nil {
		return nil, err
	}
	type textPart struct {
		Text        string `json:"text"`
		Annotations []struct {
			URL   string `json:"url"`
			Title string `json:"title"`
		} `json:"annotations"`
	}
	var out struct {
		Steps []struct {
			Type      string `json:"type"`
			Arguments struct {
				Queries []string `json:"queries"`
			} `json:"arguments"`
			Content json.RawMessage `json:"content"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	a := &answer{}
	for _, s := range out.Steps {
		switch s.Type {
		case "google_search_call":
			a.Queries = append(a.Queries, s.Arguments.Queries...)
		case "model_output":
			var parts []textPart
			json.Unmarshal(s.Content, &parts)
			for _, p := range parts {
				a.Text += p.Text
				for _, an := range p.Annotations {
					a.Cited = append(a.Cited, geminiSource(an.URL, an.Title))
				}
			}
		}
	}
	return a, nil
}

// geminiSource undoes Google's grounding redirects: the URL is an opaque
// vertexaisearch link and the title holds the real domain.
func geminiSource(u, title string) string {
	if strings.Contains(u, "vertexaisearch.cloud.google.com") && domainRe.MatchString(title) {
		return "https://" + title + "/"
	}
	return u
}

var (
	domainRe = regexp.MustCompile(`^[a-z0-9-]+(\.[a-z0-9-]+)+$`)
	urlRe    = regexp.MustCompile(`https?://[^\s<>()\[\]"'` + "`" + `]+`)
)

// brandNames are what an answer would call the site: its host, its site
// name, and the host's first label when that's distinctive enough not to be
// an ordinary word ("base" is not, "scriptroom" is).
func brandNames(host, siteName string) []*regexp.Regexp {
	var names []string
	add := func(s string) {
		if s = strings.TrimSpace(s); s != "" {
			names = append(names, s)
		}
	}
	add(host)
	if len([]rune(siteName)) >= 5 || strings.Contains(siteName, " ") {
		add(siteName)
	}
	if label, _, _ := strings.Cut(host, "."); len(label) >= 6 {
		add(label)
	}
	var out []*regexp.Regexp
	for _, n := range names {
		out = append(out, regexp.MustCompile(`(?i)(^|[^\pL\pN])`+regexp.QuoteMeta(n)+`($|[^\pL\pN])`))
	}
	return out
}

func hostsOf(urls []string) map[string]bool {
	m := map[string]bool{}
	for _, u := range urls {
		if h, _ := rankKey(u); h != "" {
			m[h] = true
		}
	}
	return m
}

// onSite reports whether host is the site or one of its subdomains.
func onSite(hosts map[string]bool, site string) bool {
	for h := range hosts {
		if h == site || strings.HasSuffix(h, "."+site) {
			return true
		}
	}
	return false
}

type askTally struct {
	Engine    string         `json:"engine"`
	Model     string         `json:"model"`
	Answers   int            `json:"answers"`
	Cited     int            `json:"cited"`
	Retrieved int            `json:"retrieved"`
	Named     int            `json:"named"`
	Searches  []string       `json:"searches,omitempty"`
	Others    map[string]int `json:"cited_instead,omitempty"`
	Cost      float64        `json:"cost_usd,omitempty"`
	Errors    []string       `json:"errors,omitempty"`
}

// tally scores one assistant's answers against the site.
func tally(name, model, site string, brands []*regexp.Regexp, answers []*answer) askTally {
	t := askTally{Engine: name, Model: model, Others: map[string]int{}}
	seenQ := map[string]bool{}
	for _, a := range answers {
		if a.Err != "" {
			if !contains(t.Errors, a.Err) {
				t.Errors = append(t.Errors, a.Err)
			}
			continue
		}
		t.Answers++
		t.Cost += a.Cost
		// Links written into the answer count as citations too: not every
		// API annotates them.
		cited := hostsOf(append(append([]string{}, a.Cited...), urlRe.FindAllString(a.Text, -1)...))
		retrieved := hostsOf(a.Sources)
		isCited := onSite(cited, site)
		if isCited {
			t.Cited++
		}
		if isCited || onSite(retrieved, site) {
			t.Retrieved++
		}
		for _, b := range brands {
			if b.MatchString(a.Text) {
				t.Named++
				break
			}
		}
		for h := range cited {
			if h != site && !strings.HasSuffix(h, "."+site) {
				t.Others[h]++
			}
		}
		for _, q := range a.Queries {
			if k := strings.ToLower(q); !seenQ[k] {
				seenQ[k] = true
				t.Searches = append(t.Searches, q)
			}
		}
	}
	return t
}

// topCounts formats the n most frequent keys as "a.com (3), b.com (2)".
func topCounts(m map[string]int, n int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if m[keys[i]] != m[keys[j]] {
			return m[keys[i]] > m[keys[j]]
		}
		return keys[i] < keys[j]
	})
	var out []string
	for _, k := range keys[:min(n, len(keys))] {
		out = append(out, fmt.Sprintf("%s (%d)", k, m[k]))
	}
	return strings.Join(out, ", ")
}

func quoteList(s []string, n int) string {
	var out []string
	for _, x := range s[:min(n, len(s))] {
		out = append(out, fmt.Sprintf("%q", x))
	}
	if len(s) > n {
		out = append(out, fmt.Sprintf("and %d more", len(s)-n))
	}
	return strings.Join(out, ", ")
}

func checkAsk(ctx context.Context, r *report, pg *page, d *doc, given []string, runs int, only []string, outDir string) {
	const a = "aeo"
	var engines []*askEngine
	var missing []string
	for _, e := range askEngines {
		if len(only) > 0 && !contains(only, e.id) {
			continue
		}
		if e.key() == "" {
			missing = append(missing, e.keyEnv[0])
			continue
		}
		engines = append(engines, e)
	}
	if len(engines) == 0 {
		r.add(a, warn, "AI answers", "-ask needs at least one assistant API key: set %s (see config/sitecheck.env.example)", strings.Join(missing, ", "))
		return
	}
	qs, source := braveQueries(r, d, given)
	qs = qs[:min(len(qs), askMaxQuestions)]
	if len(qs) == 0 {
		r.add(a, info, "AI answers", "no questions to ask: pass -q \"the question this page answers\"")
		return
	}
	runs = max(1, min(runs, 5))
	site, _ := rankKey(pg.final.String())
	siteName, _ := d.meta("og:site_name")
	brands := brandNames(site, siteName)

	results := make([][]*answer, len(engines))
	var wg sync.WaitGroup
	for i, e := range engines {
		i, e := i, e
		wg.Add(1)
		go func() {
			defer wg.Done()
			model := e.model
			if m := strings.TrimSpace(os.Getenv(e.modelEnv)); m != "" {
				model = m
			}
			for _, q := range qs {
				for n := 0; n < runs; n++ {
					ans, err := e.ask(ctx, e, e.key(), model, q)
					if err != nil {
						ans = &answer{Err: err.Error()}
					}
					ans.Engine, ans.Model, ans.Question = e.name, model, q
					results[i] = append(results[i], ans)
					if err != nil && strings.HasPrefix(err.Error(), "HTTP 4") {
						return // a bad key or model won't get better by repeating
					}
				}
			}
		}()
	}
	wg.Wait()

	var tallies []askTally
	var all []*answer
	var cost float64
	for i, e := range engines {
		if len(results[i]) == 0 {
			continue
		}
		t := tally(e.name, results[i][0].Model, site, brands, results[i])
		tallies = append(tallies, t)
		all = append(all, results[i]...)
		cost += t.Cost
		check := "AI answers: " + e.name
		for _, msg := range t.Errors {
			r.add(a, warn, check, "%s failed: %s (model %s; override with %s)", e.name, msg, t.Model, e.modelEnv)
		}
		if t.Answers == 0 {
			continue
		}
		detail := fmt.Sprintf("cited in %d of %d answers, retrieved in %d, named in %d", t.Cited, t.Answers, t.Retrieved, t.Named)
		if len(t.Searches) > 0 {
			detail += ". It searched: " + quoteList(t.Searches, 6)
		}
		if len(t.Others) > 0 {
			detail += ". Cited instead: " + topCounts(t.Others, 5)
		}
		lv := info
		switch {
		case 2*t.Cited >= t.Answers:
			lv = pass
		case t.Cited == 0 && t.Retrieved == 0 && t.Named == 0 && source != "title/h1":
			lv = warn // with guessed questions a miss may just be a bad guess
		}
		r.add(a, lv, check, "%s", detail)
	}
	note := fmt.Sprintf("asked %s, %d time(s) each (questions from %s). These are API answers, which differ from the apps, and answers vary run to run: read the counts, not one reply", quoteList(qs, 3), runs, source)
	if cost > 0 {
		note += fmt.Sprintf(". Reported cost: $%.2f (only some APIs report it)", cost)
	}
	r.add(a, info, "AI answers", "%s", note)
	r.fact("ai_answers", tallies)

	if outDir != "" {
		path := filepath.Join(outDir, "answers.md")
		if err := os.WriteFile(path, []byte(answersMarkdown(all)), 0o644); err == nil {
			r.file("ai_answers", path)
		}
	}
}

// answersMarkdown is the full transcript, for reading what each assistant
// actually said about the site.
func answersMarkdown(all []*answer) string {
	var b strings.Builder
	b.WriteString("# AI answers\n\nWhat each assistant's API answered, with web search on. Check what they say about the site against the facts.\n")
	for _, a := range all {
		fmt.Fprintf(&b, "\n## %s (%s): %s\n\n", a.Engine, a.Model, a.Question)
		if a.Err != "" {
			fmt.Fprintf(&b, "Failed: %s\n", a.Err)
			continue
		}
		if len(a.Queries) > 0 {
			fmt.Fprintf(&b, "Searched: %s\n\n", quoteList(a.Queries, 20))
		}
		if hosts := hostsOf(a.Cited); len(hosts) > 0 {
			keys := make([]string, 0, len(hosts))
			for h := range hosts {
				keys = append(keys, h)
			}
			sort.Strings(keys)
			fmt.Fprintf(&b, "Cited: %s\n\n", strings.Join(keys, ", "))
		}
		b.WriteString(strings.TrimSpace(a.Text))
		b.WriteString("\n")
	}
	return b.String()
}
