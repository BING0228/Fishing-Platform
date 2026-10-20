package utils

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// chatStreamChunk is one OpenAI-compatible SSE chunk for stream=true.
type chatStreamChunk struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Delta        struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
			ToolCalls        []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
	Usage aiUsageJSON     `json:"usage"`
	Error json.RawMessage `json:"error"`
}

// StreamChatCompletionsWithTools is the OpenAI-compatible compatibility wrapper.
func StreamChatCompletionsWithTools(
	ctx context.Context,
	baseURL, apiKey, model string,
	timeout time.Duration,
	messages []ChatLLMMessage,
	tools []ChatTool,
	onDelta func(text string),
) (ChatLLMResult, error) {
	return StreamChatCompletionsWithToolsFormat(ctx, AIAPIFormatOpenAI, baseURL, apiKey, model, timeout, messages, tools, onDelta)
}

// streamChatCompletionsOpenAI streams POST {base}/chat/completions (stream=true).
// Content deltas are forwarded to onDelta as they arrive; tool_call fragments are
// merged by index into complete calls. The caller may fall back to the
// non-streaming function on error.
func streamChatCompletionsOpenAI(
	ctx context.Context,
	baseURL, apiKey, model string,
	timeout time.Duration,
	messages []ChatLLMMessage,
	tools []ChatTool,
	onDelta func(text string),
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
		Model:         model,
		Messages:      messages,
		Temperature:   0.2,
		MaxTokens:     4096,
		Stream:        true,
		StreamOptions: &chatStreamOptions{IncludeUsage: true},
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
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return out, fmt.Errorf("request to %s failed: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		var parsed chatCompletionsAPIResponse
		_ = json.Unmarshal(body, &parsed)
		if errMsg := extractAIErrorMessage(parsed.Error); errMsg != "" {
			return out, formatAIHTTPError(resp.StatusCode, endpoint, errMsg)
		}
		return out, formatAIHTTPError(resp.StatusCode, endpoint, truncateForErr(string(body), 400))
	}

	var content strings.Builder
	var reasoning strings.Builder
	merged := map[int]*ChatToolCall{}
	finishReason := ""
	var usage AITokenUsage

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			break
		}
		var chunk chatStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			// Ignore keepalive/comment payloads; surface genuine JSON errors only
			// when they look like an API error object.
			if strings.Contains(data, `"error"`) {
				return out, fmt.Errorf("invalid stream chunk from %s: %w", endpoint, err)
			}
			continue
		}
		if errMsg := extractAIErrorMessage(chunk.Error); errMsg != "" {
			return out, formatAIHTTPError(resp.StatusCode, endpoint, errMsg)
		}
		if normalized := chunk.Usage.normalized(); normalized.TotalTokens > 0 {
			usage = normalized
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		choice := chunk.Choices[0]
		if choice.FinishReason != "" {
			finishReason = choice.FinishReason
		}
		if choice.Delta.ReasoningContent != "" {
			reasoning.WriteString(choice.Delta.ReasoningContent)
		}
		if choice.Delta.Content != "" {
			content.WriteString(choice.Delta.Content)
			if onDelta != nil {
				onDelta(choice.Delta.Content)
			}
		}
		for _, tc := range choice.Delta.ToolCalls {
			call, ok := merged[tc.Index]
			if !ok {
				call = &ChatToolCall{Type: "function"}
				merged[tc.Index] = call
			}
			if tc.ID != "" && call.ID == "" {
				call.ID = tc.ID
			}
			if tc.Type != "" {
				call.Type = tc.Type
			}
			if tc.Function.Name != "" && call.Function.Name == "" {
				call.Function.Name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				call.Function.Arguments += tc.Function.Arguments
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return out, fmt.Errorf("stream read failed: %w", err)
	}

	indexes := make([]int, 0, len(merged))
	for idx := range merged {
		indexes = append(indexes, idx)
	}
	sort.Ints(indexes)
	for _, idx := range indexes {
		call := merged[idx]
		if call.Type == "" {
			call.Type = "function"
		}
		if strings.TrimSpace(call.Function.Arguments) == "" {
			call.Function.Arguments = "{}"
		}
		out.ToolCalls = append(out.ToolCalls, *call)
	}
	out.Content = strings.TrimSpace(content.String())
	out.ReasoningContent = reasoning.String()
	out.FinishReason = finishReason
	recordAIUsage(ctx, usage)
	return out, nil
}
