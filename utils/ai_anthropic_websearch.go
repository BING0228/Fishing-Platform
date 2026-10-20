package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	anthropicWebSearchTool  = "web_search_20250305"
	infoGatherModeAnthropic = "anthropic_web_search"
)

func anthropicWebSearchRequest(model string, in InfoGatherInput) anthropicMessagesRequest {
	system, messages, _ := buildAnthropicMessages([]ChatLLMMessage{
		{Role: "system", Content: InfoGatherSystemPrompt},
		{Role: "user", Content: BuildInfoGatherUserPrompt(in) + "\n\nUse the web_search tool. Follow the JSON-only output contract."},
	})
	payload := newAnthropicRequest(model, system, messages, nil, 4096, 0, false)
	payload.Tools = []anthropicTool{{
		Type:    anthropicWebSearchTool,
		Name:    "web_search",
		MaxUses: 5,
	}}
	return payload
}

// gatherContactsViaAnthropicWebSearch uses Anthropic's server-side web_search
// tool and parses the final JSON text block.
func gatherContactsViaAnthropicWebSearch(ctx context.Context, baseURL, apiKey, model string, timeout time.Duration, in InfoGatherInput) (InfoGatherResult, error) {
	var empty InfoGatherResult
	reportInfoGatherProgress(in, "searching", 20, "Searching the web with Anthropic")
	payload := anthropicWebSearchRequest(model, in)
	resp, endpoint, err := sendAnthropicRequest(ctx, baseURL, apiKey, payload, timeout, false)
	if err != nil {
		return empty, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 12<<20))
	if err != nil {
		return empty, err
	}
	reportInfoGatherProgress(in, "synthesizing", 75, "Extracting emails and phones")
	result, err := parseAnthropicResponse(body, resp.StatusCode, endpoint, ctx)
	if err != nil {
		return empty, err
	}
	text := strings.TrimSpace(result.Content)
	if text == "" {
		return empty, fmt.Errorf("Anthropic returned no text from %s", endpoint)
	}
	reportInfoGatherProgress(in, "parsing", 90, "Validating email/phone findings")
	parsed, err := ParseInfoGatherJSON(text)
	parsed.RawResponse = truncateRaw(text, infoGatherRawMax)
	return parsed, err
}

func probeAnthropicWebSearch(ctx context.Context, baseURL, apiKey, model string, timeout time.Duration) (ProbeWebSearchResult, error) {
	out := ProbeWebSearchResult{Model: strings.TrimSpace(model), Mode: infoGatherModeAnthropic}
	if strings.TrimSpace(apiKey) == "" {
		return out, fmt.Errorf("API key is required")
	}
	if strings.TrimSpace(model) == "" {
		return out, fmt.Errorf("model is required")
	}
	baseURL = NormalizeAnthropicBaseURL(baseURL)
	if baseURL == "" {
		return out, fmt.Errorf("base URL is required")
	}
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	system, messages, err := buildAnthropicMessages([]ChatLLMMessage{
		{Role: "user", Content: "Use web_search and reply with one short sentence confirming web search works."},
	})
	if err != nil {
		return out, err
	}
	payload := newAnthropicRequest(model, system, messages, nil, 64, 0, false)
	payload.Tools = []anthropicTool{{
		Type:    anthropicWebSearchTool,
		Name:    "web_search",
		MaxUses: 1,
	}}
	started := time.Now()
	resp, endpoint, err := sendAnthropicRequest(ctx, baseURL, apiKey, payload, timeout, false)
	out.Endpoint = endpoint
	out.LatencyMs = time.Since(started).Milliseconds()
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	out.StatusCode = resp.StatusCode
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return out, err
	}
	var parsed anthropicMessagesResponse
	_ = json.Unmarshal(body, &parsed)
	if errMsg := extractAIErrorMessage(parsed.Error); errMsg != "" {
		return out, formatAIHTTPError(resp.StatusCode, endpoint, errMsg)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return out, formatAIHTTPError(resp.StatusCode, endpoint, truncateForErr(string(body), 300))
	}
	recordAIUsage(ctx, parsed.Usage.normalized())
	return out, nil
}
