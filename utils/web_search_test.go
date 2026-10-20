package utils

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNormalizeSearchProvider(t *testing.T) {
	cases := map[string]string{
		"":             SearchProviderAuto,
		"auto":         SearchProviderAuto,
		"LLM":          SearchProviderAuto,
		"native":       SearchProviderAuto,
		"deepseek":     SearchProviderDeepSeek,
		"Brave":        SearchProviderBrave,
		"brave_search": SearchProviderBrave,
		"tavily":       SearchProviderTavily,
		"serper":       SearchProviderSerper,
		"serperdev":    SearchProviderSerper,
		"bocha":        SearchProviderBocha,
		"bochaai":      SearchProviderBocha,
	}
	for in, want := range cases {
		if got := NormalizeSearchProvider(in); got != want {
			t.Fatalf("NormalizeSearchProvider(%q) = %q, want %q", in, got, want)
		}
	}
	if !IsStandaloneSearchProvider("deepseek") || IsStandaloneSearchProvider("auto") {
		t.Fatal("standalone provider classification is wrong")
	}
}

func TestSearchEndpoint(t *testing.T) {
	cases := []struct{ provider, base, want string }{
		{"deepseek", "https://api.deepseek.com", "https://api.deepseek.com/v1/tools/search"},
		{"deepseek", "https://api.deepseek.com/v1", "https://api.deepseek.com/v1/tools/search"},
		{"deepseek", "https://proxy.example/v1/tools/search", "https://proxy.example/v1/tools/search"},
		{"brave", "https://api.search.brave.com/res/v1", "https://api.search.brave.com/res/v1/web/search"},
		{"brave", "https://api.search.brave.com", "https://api.search.brave.com/res/v1/web/search"},
		{"tavily", "https://api.tavily.com", "https://api.tavily.com/search"},
		{"serper", "https://google.serper.dev", "https://google.serper.dev/search"},
		{"bocha", "https://api.bochaai.com/v1", "https://api.bochaai.com/v1/web-search"},
		{"bocha", "https://api.bochaai.com", "https://api.bochaai.com/v1/web-search"},
	}
	for _, tc := range cases {
		if got := SearchEndpoint(tc.provider, tc.base); got != tc.want {
			t.Fatalf("SearchEndpoint(%q,%q) = %q, want %q", tc.provider, tc.base, got, tc.want)
		}
	}
}

func TestSearchDeepSeekParsesResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tools/search" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer key" {
			t.Errorf("auth = %q", got)
		}
		fmt.Fprint(w, `{"results":[{"title":"T","url":"https://a.example","snippet":"S","summary":"Sum","content":"Body","date":"2026-01-01T00:00:00Z"}]}`)
	}))
	defer srv.Close()

	hits, err := SearchWeb(context.Background(), SearchProviderConfig{Provider: "deepseek", BaseURL: srv.URL, APIKey: "key", Count: 5}, "q", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].URL != "https://a.example" || hits[0].Content != "Body" {
		t.Fatalf("hits = %+v", hits)
	}
}

func TestSearchBraveParsesResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/res/v1/web/search" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("q") != "q" {
			t.Errorf("q = %q", r.URL.Query().Get("q"))
		}
		if got := r.Header.Get("X-Subscription-Token"); got != "key" {
			t.Errorf("token = %q", got)
		}
		fmt.Fprint(w, `{"web":{"results":[{"title":"T","url":"https://b.example","description":"Desc"}]}}`)
	}))
	defer srv.Close()

	hits, err := SearchWeb(context.Background(), SearchProviderConfig{Provider: "brave", BaseURL: srv.URL, APIKey: "key"}, "q", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Content != "Desc" {
		t.Fatalf("hits = %+v", hits)
	}
}

func TestSearchTavilyParsesResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Errorf("path = %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"results":[{"title":"T","url":"https://t.example","content":"C","raw_content":"Raw"}]}`)
	}))
	defer srv.Close()

	hits, err := SearchWeb(context.Background(), SearchProviderConfig{Provider: "tavily", BaseURL: srv.URL, APIKey: "key"}, "q", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Content != "Raw" {
		t.Fatalf("hits = %+v", hits)
	}
}

func TestSearchSerperParsesResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-KEY") != "key" {
			t.Errorf("api key header missing")
		}
		fmt.Fprint(w, `{"organic":[{"title":"T","link":"https://s.example","snippet":"Snip"}]}`)
	}))
	defer srv.Close()

	hits, err := SearchWeb(context.Background(), SearchProviderConfig{Provider: "serper", BaseURL: srv.URL, APIKey: "key"}, "q", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Snippet != "Snip" {
		t.Fatalf("hits = %+v", hits)
	}
}

func TestSearchBochaParsesResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/web-search" {
			t.Errorf("path = %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"code":200,"data":{"webPages":{"value":[{"name":"T","url":"https://c.example","snippet":"Snip","summary":"Sum"}]}}}`)
	}))
	defer srv.Close()

	hits, err := SearchWeb(context.Background(), SearchProviderConfig{Provider: "bocha", BaseURL: srv.URL, APIKey: "key"}, "q", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Content != "Sum" {
		t.Fatalf("hits = %+v", hits)
	}
}

func TestSearchWebRejectsNative(t *testing.T) {
	if _, err := SearchWeb(context.Background(), SearchProviderConfig{Provider: "auto"}, "q", time.Second); err == nil {
		t.Fatal("auto must not be treated as a standalone provider")
	}
}

func TestSearchWebRequiresKey(t *testing.T) {
	if _, err := SearchWeb(context.Background(), SearchProviderConfig{Provider: "brave"}, "q", time.Second); err == nil {
		t.Fatal("missing key should fail")
	}
}

func TestGatherContactsViaSearchProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/tools/search":
			fmt.Fprint(w, `{"results":[{"title":"Contact","url":"https://target.example/contact","snippet":"mail: a@example.com","summary":"a@example.com"}]}`)
		case "/chat/completions":
			fmt.Fprint(w, `{"choices":[{"message":{"content":"{\"findings\":[{\"kind\":\"email\",\"value\":\"a@example.com\",\"source_url\":\"https://target.example/contact\"}],\"notes\":\"ok\"}"}}]}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	res, err := GatherContactsWithAI(context.Background(), srv.URL, "key", "m", 30*time.Second, InfoGatherInput{
		Target:   "target.example",
		Searches: []SearchProviderConfig{{Provider: "deepseek", BaseURL: srv.URL, APIKey: "key", Count: 5}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 1 || res.Findings[0].Value != "a@example.com" {
		t.Fatalf("findings = %+v", res.Findings)
	}
	if !strings.Contains(res.RawResponse, "a@example.com") {
		t.Fatalf("raw = %q", res.RawResponse)
	}
}

func TestSearchWebAllMergesAndDedupes(t *testing.T) {
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"results":[
			{"title":"A","url":"https://dup.example","snippet":"one"},
			{"title":"B","url":"https://b.example","snippet":"bee"}
		]}`)
	}))
	defer first.Close()
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"results":[
			{"title":"A2","url":"https://dup.example","snippet":"two"},
			{"title":"C","url":"https://c.example","snippet":"see"}
		]}`)
	}))
	defer second.Close()

	hits, errs := SearchWebAll(context.Background(), []SearchProviderConfig{
		{Provider: "deepseek", BaseURL: first.URL, APIKey: "k", Count: 5},
		{Provider: "deepseek", BaseURL: second.URL, APIKey: "k", Count: 5},
	}, "q", 5*time.Second)
	if len(errs) != 0 {
		t.Fatalf("errs = %v", errs)
	}
	if len(hits) != 3 {
		t.Fatalf("hits = %+v", hits)
	}
	if hits[0].URL != "https://dup.example" || hits[0].Title != "A" || hits[0].Snippet != "one" {
		t.Fatalf("first hit should win dedupe: %+v", hits[0])
	}
}

func TestSearchWebAllReturnsPartialHitsOnProviderError(t *testing.T) {
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"results":[{"title":"A","url":"https://a.example","snippet":"a"}]}`)
	}))
	defer good.Close()
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer bad.Close()

	hits, errs := SearchWebAll(context.Background(), []SearchProviderConfig{
		{Provider: "deepseek", BaseURL: good.URL, APIKey: "k"},
		{Provider: "deepseek", BaseURL: bad.URL, APIKey: "k"},
	}, "q", 5*time.Second)
	if len(hits) != 1 || hits[0].URL != "https://a.example" {
		t.Fatalf("hits = %+v", hits)
	}
	if len(errs) != 1 {
		t.Fatalf("errs = %v, want 1", errs)
	}
}
