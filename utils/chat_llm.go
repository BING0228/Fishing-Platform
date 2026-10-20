package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ChatToolFunction describes one OpenAI-compatible function tool.
type ChatToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// ChatTool is an OpenAI tools[] entry.
type ChatTool struct {
	Type     string           `json:"type"` // "function"
	Function ChatToolFunction `json:"function"`
}

// ChatToolCallFunction holds name + JSON arguments string.
type ChatToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatToolCall is one tool invocation requested by the model.
type ChatToolCall struct {
	ID       string               `json:"id"`
	Type     string               `json:"type"`
	Function ChatToolCallFunction `json:"function"`
}

// ChatLLMMessage is one message in an OpenAI-compatible conversation.
type ChatLLMMessage struct {
	Role             string         `json:"role"`
	ReasoningContent string         `json:"reasoning_content,omitempty"`
	Content          string         `json:"content,omitempty"`
	ToolCalls        []ChatToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string         `json:"tool_call_id,omitempty"`
	Name             string         `json:"name,omitempty"`
}

// ChatLLMResult is the assistant turn returned by ChatCompletionsWithTools.
type ChatLLMResult struct {
	Content          string
	ReasoningContent string
	ToolCalls        []ChatToolCall
	FinishReason     string
	Raw              json.RawMessage
}

type chatCompletionsPayload struct {
	Model         string             `json:"model"`
	Messages      []ChatLLMMessage   `json:"messages"`
	Tools         []ChatTool         `json:"tools,omitempty"`
	ToolChoice    string             `json:"tool_choice,omitempty"`
	Temperature   float64            `json:"temperature,omitempty"`
	MaxTokens     int                `json:"max_tokens,omitempty"`
	Stream        bool               `json:"stream,omitempty"`
	StreamOptions *chatStreamOptions `json:"stream_options,omitempty"`
}

type chatStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type chatCompletionsAPIResponse struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Role      string         `json:"role"`
			Content   string         `json:"content"`
			ToolCalls []ChatToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage aiUsageJSON     `json:"usage"`
	Error json.RawMessage `json:"error"`
}

// ChatCompletionsWithTools is the OpenAI-compatible compatibility wrapper.
func ChatCompletionsWithTools(
	ctx context.Context,
	baseURL, apiKey, model string,
	timeout time.Duration,
	messages []ChatLLMMessage,
	tools []ChatTool,
) (ChatLLMResult, error) {
	return ChatCompletionsWithToolsFormat(ctx, AIAPIFormatOpenAI, baseURL, apiKey, model, timeout, messages, tools)
}

// chatCompletionsOpenAI calls POST {base}/chat/completions with optional tools.
// Non-streaming; the console streams its own SSE events to the browser.
func chatCompletionsOpenAI(
	ctx context.Context,
	baseURL, apiKey, model string,
	timeout time.Duration,
	messages []ChatLLMMessage,
	tools []ChatTool,
) (ChatLLMResult, error) {
	out := ChatLLMResult{}
	if strings.TrimSpace(apiKey) == "" {
		return out, fmt.Errorf("API key is required")
	}
	if strings.TrimSpace(model) == "" {
		return out, fmt.Errorf("model is required")
	}
	baseURL = NormalizeOpenAICompatibleBaseURL(baseURL)
	if baseURL == "" {
		return out, fmt.Errorf("base URL is required")
	}
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	endpoint := baseURL + "/chat/completions"

	payload := chatCompletionsPayload{
		Model:       model,
		Messages:    messages,
		Temperature: 0.2,
		MaxTokens:   4096,
	}
	if len(tools) > 0 {
		payload.Tools = tools
		payload.ToolChoice = "auto"
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return out, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return out, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return out, fmt.Errorf("request to %s failed: %w", endpoint, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return out, err
	}
	out.Raw = append(json.RawMessage(nil), body...)

	var parsed chatCompletionsAPIResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return out, fmt.Errorf("invalid chat completions JSON from %s: %w", endpoint, err)
	}
	if errMsg := extractAIErrorMessage(parsed.Error); errMsg != "" {
		return out, formatAIHTTPError(resp.StatusCode, endpoint, errMsg)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return out, formatAIHTTPError(resp.StatusCode, endpoint, truncateForErr(string(body), 400))
	}
	recordAIUsage(ctx, parsed.Usage.normalized())
	if len(parsed.Choices) == 0 {
		return out, fmt.Errorf("empty choices from %s", endpoint)
	}
	msg := parsed.Choices[0].Message
	out.Content = strings.TrimSpace(msg.Content)
	out.ToolCalls = msg.ToolCalls
	out.FinishReason = parsed.Choices[0].FinishReason
	return out, nil
}
