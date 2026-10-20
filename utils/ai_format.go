package utils

import "strings"

const (
	// AIAPIFormatOpenAI is the OpenAI-compatible chat/completions wire format.
	AIAPIFormatOpenAI = "openai"
	// AIAPIFormatAnthropic is the Anthropic Messages wire format.
	AIAPIFormatAnthropic = "anthropic"
)

// NormalizeAIAPIFormat maps aliases to a supported format and returns "" for
// an unknown value. An empty value means the caller did not choose a format.
func NormalizeAIAPIFormat(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "openai", "openai-compatible", "openai_compatible":
		return AIAPIFormatOpenAI
	case "anthropic", "claude", "anthropic-messages", "anthropic_messages":
		return AIAPIFormatAnthropic
	default:
		return ""
	}
}

// AIAPIFormatOrDefault returns OpenAI-compatible for an empty/legacy value.
func AIAPIFormatOrDefault(value string) string {
	if format := NormalizeAIAPIFormat(value); format != "" {
		return format
	}
	return AIAPIFormatOpenAI
}

// NormalizeAIBaseURL strips the endpoint path for the selected wire format.
func NormalizeAIBaseURL(format, baseURL string) string {
	if AIAPIFormatOrDefault(format) == AIAPIFormatAnthropic {
		return NormalizeAnthropicBaseURL(baseURL)
	}
	return NormalizeOpenAICompatibleBaseURL(baseURL)
}

// NormalizeAnthropicBaseURL accepts an Anthropic API root or a full
// /messages endpoint and returns the API root (usually ending in /v1).
func NormalizeAnthropicBaseURL(baseURL string) string {
	u := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	for u != "" {
		lower := strings.ToLower(u)
		var stripped bool
		for _, suffix := range []string{"/messages", "/chat/completions", "/responses", "/completions"} {
			if strings.HasSuffix(lower, suffix) {
				u = strings.TrimRight(u[:len(u)-len(suffix)], "/")
				stripped = true
				break
			}
		}
		if !stripped {
			break
		}
	}
	// The canonical Anthropic host needs its version root. Custom proxies are
	// left exactly as entered so self-hosted gateways keep their own paths.
	if strings.EqualFold(u, "https://api.anthropic.com") || strings.EqualFold(u, "http://api.anthropic.com") {
		u += "/v1"
	}
	return u
}

// AnthropicMessagesEndpoint appends /messages to a normalized root.
func AnthropicMessagesEndpoint(baseURL string) string {
	return NormalizeAnthropicBaseURL(baseURL) + "/messages"
}
