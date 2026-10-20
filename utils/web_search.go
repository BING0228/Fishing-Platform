package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// SearchProviderConfig is the resolved web-search provider configuration.
type SearchProviderConfig struct {
	Provider string
	BaseURL  string
	APIKey   string
	Count    int
}

// SearchHit is one normalized web result from any provider.
type SearchHit struct {
	Title   string
	URL     string
	Snippet string
	Content string
	Date    string
}

// SearchProviderInfo describes one selectable provider for the settings UI.
type SearchProviderInfo struct {
	ID             string `json:"id"`
	Label          string `json:"label"`
	DefaultBaseURL string `json:"default_base_url"`
	RequiresKey    bool   `json:"requires_key"`
	Native         bool   `json:"native,omitempty"`
}

const (
	SearchProviderAuto     = "auto"
	SearchProviderDeepSeek = "deepseek"
	SearchProviderBrave    = "brave"
	SearchProviderTavily   = "tavily"
	SearchProviderSerper   = "serper"
	SearchProviderBocha    = "bocha"
)

// SearchProviders is the provider catalog returned to the settings UI.
func SearchProviders() []SearchProviderInfo {
	return []SearchProviderInfo{
		{ID: SearchProviderAuto, Label: "AI Profile (native web_search)", DefaultBaseURL: "", RequiresKey: false, Native: true},
		{ID: SearchProviderDeepSeek, Label: "DeepSeek Search", DefaultBaseURL: "https://api.deepseek.com", RequiresKey: true},
		{ID: SearchProviderBrave, Label: "Brave Search", DefaultBaseURL: "https://api.search.brave.com/res/v1", RequiresKey: true},
		{ID: SearchProviderTavily, Label: "Tavily", DefaultBaseURL: "https://api.tavily.com", RequiresKey: true},
		{ID: SearchProviderSerper, Label: "Serper (Google)", DefaultBaseURL: "https://google.serper.dev", RequiresKey: true},
		{ID: SearchProviderBocha, Label: "Bocha 博查", DefaultBaseURL: "https://api.bochaai.com/v1", RequiresKey: true},
	}
}

// NormalizeSearchProvider canonicalizes aliases and casing.
func NormalizeSearchProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "", "auto", "llm", "native", "ai", "profile":
		return SearchProviderAuto
	case SearchProviderDeepSeek:
		return SearchProviderDeepSeek
	case SearchProviderBrave, "brave_search":
		return SearchProviderBrave
	case SearchProviderTavily:
		return SearchProviderTavily
	case SearchProviderSerper, "serperdev":
		return SearchProviderSerper
	case SearchProviderBocha, "bochaai":
		return SearchProviderBocha
	default:
		return strings.ToLower(strings.TrimSpace(provider))
	}
}

// IsKnownSearchProvider reports whether the provider id is supported.
func IsKnownSearchProvider(provider string) bool {
	for _, p := range SearchProviders() {
		if p.ID == NormalizeSearchProvider(provider) {
			return true
		}
	}
	return false
}

// IsStandaloneSearchProvider reports whether the provider calls its own API
// (i.e. not the AI profile's native web_search).
func IsStandaloneSearchProvider(provider string) bool {
	switch NormalizeSearchProvider(provider) {
	case SearchProviderDeepSeek, SearchProviderBrave, SearchProviderTavily, SearchProviderSerper, SearchProviderBocha:
		return true
	default:
		return false
	}
}

// DefaultSearchBaseURL returns the documented base URL for a provider.
func DefaultSearchBaseURL(provider string) string {
	for _, p := range SearchProviders() {
		if p.ID == NormalizeSearchProvider(provider) {
			return p.DefaultBaseURL
		}
	}
	return ""
}

// SearchCountOrDefault clamps a configured result count.
func SearchCountOrDefault(n int) int {
	if n <= 0 {
		return 10
	}
	if n > 50 {
		return 50
	}
	return n
}

// SearchEndpoint returns the concrete endpoint SearchWeb will call.
func SearchEndpoint(provider, baseURL string) string {
	provider = NormalizeSearchProvider(provider)
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = DefaultSearchBaseURL(provider)
	}
	lower := strings.ToLower(base)
	switch provider {
	case SearchProviderDeepSeek:
		if strings.HasSuffix(lower, "/tools/search") {
			return base
		}
		if strings.HasSuffix(lower, "/v1") {
			return base + "/tools/search"
		}
		return base + "/v1/tools/search"
	case SearchProviderBrave:
		if strings.HasSuffix(lower, "/web/search") {
			return base
		}
		if strings.HasSuffix(lower, "/res/v1") {
			return base + "/web/search"
		}
		return base + "/res/v1/web/search"
	case SearchProviderTavily, SearchProviderSerper:
		if strings.HasSuffix(lower, "/search") {
			return base
		}
		return base + "/search"
	case SearchProviderBocha:
		if strings.HasSuffix(lower, "/web-search") {
			return base
		}
		if strings.HasSuffix(lower, "/v1") {
			return base + "/web-search"
		}
		return base + "/v1/web-search"
	default:
		return base
	}
}

// SearchWeb queries one standalone provider and returns normalized hits.
func SearchWeb(ctx context.Context, cfg SearchProviderConfig, query string, timeout time.Duration) ([]SearchHit, error) {
	provider := NormalizeSearchProvider(cfg.Provider)
	if !IsStandaloneSearchProvider(provider) {
		return nil, fmt.Errorf("search provider %q is not a standalone API", cfg.Provider)
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("%s API key is required", provider)
	}
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	count := SearchCountOrDefault(cfg.Count)
	switch provider {
	case SearchProviderDeepSeek:
		return searchDeepSeek(ctx, cfg, query, count, timeout)
	case SearchProviderBrave:
		return searchBrave(ctx, cfg, query, count, timeout)
	case SearchProviderTavily:
		return searchTavily(ctx, cfg, query, count, timeout)
	case SearchProviderSerper:
		return searchSerper(ctx, cfg, query, count, timeout)
	case SearchProviderBocha:
		return searchBocha(ctx, cfg, query, count, timeout)
	default:
		return nil, fmt.Errorf("unsupported search provider %q", provider)
	}
}

// SearchWebAll queries every standalone provider in parallel and merges the
// hits, deduplicating by URL while preserving provider order. Provider errors
// are returned with any partial results so callers can fall back or report.
func SearchWebAll(ctx context.Context, cfgs []SearchProviderConfig, query string, timeout time.Duration) ([]SearchHit, []error) {
	if len(cfgs) == 0 {
		return nil, nil
	}
	type result struct {
		hits []SearchHit
		err  error
	}
	results := make([]result, len(cfgs))
	var wg sync.WaitGroup
	for i, cfg := range cfgs {
		wg.Add(1)
		go func(i int, cfg SearchProviderConfig) {
			defer wg.Done()
			hits, err := SearchWeb(ctx, cfg, query, timeout)
			results[i] = result{hits: hits, err: err}
		}(i, cfg)
	}
	wg.Wait()

	out := make([]SearchHit, 0)
	byURL := map[string]int{}
	var errs []error
	for i, res := range results {
		if res.err != nil {
			provider := NormalizeSearchProvider(cfgs[i].Provider)
			if provider == "" {
				provider = "search"
			}
			errs = append(errs, fmt.Errorf("%s: %w", provider, res.err))
			continue
		}
		for _, hit := range res.hits {
			link := strings.TrimSpace(hit.URL)
			if link == "" {
				continue
			}
			if idx, ok := byURL[link]; ok {
				if out[idx].Title == "" {
					out[idx].Title = hit.Title
				}
				if out[idx].Snippet == "" {
					out[idx].Snippet = hit.Snippet
				}
				if out[idx].Content == "" {
					out[idx].Content = hit.Content
				}
				if out[idx].Date == "" {
					out[idx].Date = hit.Date
				}
				continue
			}
			byURL[link] = len(out)
			out = append(out, hit)
		}
	}
	return out, errs
}

// ProbeSearchResult is returned by ProbeSearchProvider.
type ProbeSearchResult struct {
	Provider    string `json:"provider"`
	Endpoint    string `json:"endpoint"`
	LatencyMs   int64  `json:"latency_ms"`
	StatusCode  int    `json:"status"`
	ResultCount int    `json:"result_count"`
}

// ProbeSearchProvider runs a small real query to verify a provider config.
func ProbeSearchProvider(ctx context.Context, cfg SearchProviderConfig, timeout time.Duration) (ProbeSearchResult, error) {
	start := time.Now()
	out := ProbeSearchResult{
		Provider: NormalizeSearchProvider(cfg.Provider),
		Endpoint: SearchEndpoint(cfg.Provider, cfg.BaseURL),
	}
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	hits, err := SearchWeb(ctx, cfg, "example.com", timeout)
	out.LatencyMs = time.Since(start).Milliseconds()
	out.ResultCount = len(hits)
	if err != nil {
		return out, err
	}
	if len(hits) == 0 {
		return out, fmt.Errorf("search returned no results")
	}
	out.StatusCode = http.StatusOK
	return out, nil
}

func doSearchRequest(req *http.Request, timeout time.Duration, provider string) ([]byte, int, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("%s search request failed: %w", provider, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("%s search read failed: %w", provider, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return body, resp.StatusCode, fmt.Errorf("%s search failed (HTTP %d): %s", provider, resp.StatusCode, truncateForErr(string(body), 300))
	}
	return body, resp.StatusCode, nil
}

func newSearchJSONRequest(ctx context.Context, endpoint string, payload any) (*http.Request, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return req, nil
}

func searchDeepSeek(ctx context.Context, cfg SearchProviderConfig, query string, count int, timeout time.Duration) ([]SearchHit, error) {
	endpoint := SearchEndpoint(SearchProviderDeepSeek, cfg.BaseURL)
	req, err := newSearchJSONRequest(ctx, endpoint, map[string]any{
		"query":        query,
		"count":        count,
		"need_content": true,
	})
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	body, _, err := doSearchRequest(req, timeout, SearchProviderDeepSeek)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Snippet string `json:"snippet"`
			Summary string `json:"summary"`
			Content string `json:"content"`
			Date    string `json:"date"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("invalid DeepSeek search response: %w", err)
	}
	out := make([]SearchHit, 0, len(parsed.Results))
	for _, r := range parsed.Results {
		content := firstNonEmpty(r.Content, r.Summary)
		out = append(out, SearchHit{
			Title:   strings.TrimSpace(r.Title),
			URL:     strings.TrimSpace(r.URL),
			Snippet: firstNonEmpty(r.Snippet, r.Summary),
			Content: strings.TrimSpace(content),
			Date:    strings.TrimSpace(r.Date),
		})
	}
	return out, nil
}

func searchBrave(ctx context.Context, cfg SearchProviderConfig, query string, count int, timeout time.Duration) ([]SearchHit, error) {
	endpoint := SearchEndpoint(SearchProviderBrave, cfg.BaseURL)
	values := url.Values{}
	values.Set("q", query)
	values.Set("count", fmt.Sprint(minInt(count, 20)))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", cfg.APIKey)
	body, _, err := doSearchRequest(req, timeout, SearchProviderBrave)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
				Age         string `json:"age"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("invalid Brave search response: %w", err)
	}
	out := make([]SearchHit, 0, len(parsed.Web.Results))
	for _, r := range parsed.Web.Results {
		desc := strings.TrimSpace(r.Description)
		out = append(out, SearchHit{
			Title:   strings.TrimSpace(r.Title),
			URL:     strings.TrimSpace(r.URL),
			Snippet: desc,
			Content: desc,
			Date:    strings.TrimSpace(r.Age),
		})
	}
	return out, nil
}

func searchTavily(ctx context.Context, cfg SearchProviderConfig, query string, count int, timeout time.Duration) ([]SearchHit, error) {
	endpoint := SearchEndpoint(SearchProviderTavily, cfg.BaseURL)
	req, err := newSearchJSONRequest(ctx, endpoint, map[string]any{
		"api_key":             cfg.APIKey,
		"query":               query,
		"max_results":         count,
		"include_raw_content": true,
		"search_depth":        "basic",
	})
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	body, _, err := doSearchRequest(req, timeout, SearchProviderTavily)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Results []struct {
			Title      string `json:"title"`
			URL        string `json:"url"`
			Content    string `json:"content"`
			RawContent string `json:"raw_content"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("invalid Tavily search response: %w", err)
	}
	out := make([]SearchHit, 0, len(parsed.Results))
	for _, r := range parsed.Results {
		out = append(out, SearchHit{
			Title:   strings.TrimSpace(r.Title),
			URL:     strings.TrimSpace(r.URL),
			Snippet: strings.TrimSpace(r.Content),
			Content: strings.TrimSpace(firstNonEmpty(r.RawContent, r.Content)),
		})
	}
	return out, nil
}

func searchSerper(ctx context.Context, cfg SearchProviderConfig, query string, count int, timeout time.Duration) ([]SearchHit, error) {
	endpoint := SearchEndpoint(SearchProviderSerper, cfg.BaseURL)
	req, err := newSearchJSONRequest(ctx, endpoint, map[string]any{
		"q":   query,
		"num": count,
	})
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-KEY", cfg.APIKey)
	body, _, err := doSearchRequest(req, timeout, SearchProviderSerper)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Organic []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
			Date    string `json:"date"`
		} `json:"organic"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("invalid Serper search response: %w", err)
	}
	out := make([]SearchHit, 0, len(parsed.Organic))
	for _, r := range parsed.Organic {
		out = append(out, SearchHit{
			Title:   strings.TrimSpace(r.Title),
			URL:     strings.TrimSpace(r.Link),
			Snippet: strings.TrimSpace(r.Snippet),
			Content: strings.TrimSpace(r.Snippet),
			Date:    strings.TrimSpace(r.Date),
		})
	}
	return out, nil
}

func searchBocha(ctx context.Context, cfg SearchProviderConfig, query string, count int, timeout time.Duration) ([]SearchHit, error) {
	endpoint := SearchEndpoint(SearchProviderBocha, cfg.BaseURL)
	req, err := newSearchJSONRequest(ctx, endpoint, map[string]any{
		"query":     query,
		"count":     count,
		"summary":   true,
		"freshness": "noLimit",
	})
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	body, _, err := doSearchRequest(req, timeout, SearchProviderBocha)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			WebPages struct {
				Value []struct {
					Name          string `json:"name"`
					URL           string `json:"url"`
					Snippet       string `json:"snippet"`
					Summary       string `json:"summary"`
					DatePublished string `json:"datePublished"`
				} `json:"value"`
			} `json:"webPages"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("invalid Bocha search response: %w", err)
	}
	if parsed.Code != 0 && parsed.Code != http.StatusOK {
		return nil, fmt.Errorf("Bocha search failed (code %d): %s", parsed.Code, truncateForErr(parsed.Message, 200))
	}
	values := parsed.Data.WebPages.Value
	out := make([]SearchHit, 0, len(values))
	for _, r := range values {
		out = append(out, SearchHit{
			Title:   strings.TrimSpace(r.Name),
			URL:     strings.TrimSpace(r.URL),
			Snippet: firstNonEmpty(r.Snippet, r.Summary),
			Content: firstNonEmpty(r.Summary, r.Snippet),
			Date:    strings.TrimSpace(r.DatePublished),
		})
	}
	return out, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
