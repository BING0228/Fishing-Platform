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

const anthropicAPIVersion = "2023-06-01"

type anthropicContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   any             `json:"content,omitempty"`
}

type anthropicMessage struct {
	Role    string                  `json:"role"`
	Content []anthropicContentBlock `json:"content"`
}

type anthropicTool struct {
	Type        string          `json:"type,omitempty"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
	MaxUses     int             `json:"max_uses,omitempty"`
}

type anthropicMessagesRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
	Tools       []anthropicTool    `json:"tools,omitempty"`
	Temperature float64            `json:"temperature,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
}

type anthropicMessagesResponse struct {
	Content    []anthropicContentBlock `json:"content"`
	StopReason string                  `json:"stop_reason"`
	Usage      aiUsageJSON             `json:"usage"`
	Error      json.RawMessage         `json:"error"`
}

type anthropicStreamEvent struct {
	Type         string `json:"type"`
	Index        int    `json:"index"`
	ContentBlock struct {
		Type  string          `json:"type"`
		ID    string          `json:"id"`
		Name  string          `json:"name"`
		Text  string          `json:"text"`
		Input json.RawMessage `json:"input"`
	} `json:"content_block"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		PartialJSON string `json:"partial_json"`
		StopReason  string `json:"stop_reason"`
		Thinking    string `json:"thinking"`
	} `json:"delta"`
	Message struct {
		Usage      aiUsageJSON `json:"usage"`
		StopReason string      `json:"stop_reason"`
	} `json:"message"`
	Usage aiUsageJSON     `json:"usage"`
	Error json.RawMessage `json:"error"`
}

// AITextResult is a one-turn text completion from either wire format.
type AITextResult struct {
	Text     string
	Endpoint string
	Raw      json.RawMessage
}

// ChatCompletionsWithToolsFormat dispatches to the selected API wire format.
func ChatCompletionsWithToolsFormat(
	ctx context.Context,
	apiFormat, baseURL, apiKey, model string,
	timeout time.Duration,
	messages []ChatLLMMessage,
	tools []ChatTool,
) (ChatLLMResult, error) {
	if AIAPIFormatOrDefault(apiFormat) == AIAPIFormatAnthropic {
		return chatCompletionsAnthropic(ctx, baseURL, apiKey, model, timeout, messages, tools)
	}
	return chatCompletionsOpenAI(ctx, baseURL, apiKey, model, timeout, messages, tools)
}

// StreamChatCompletionsWithToolsFormat dispatches streaming to the selected
// API wire format. Callers may fall back to the non-streaming call.
func StreamChatCompletionsWithToolsFormat(
	ctx context.Context,
	apiFormat, baseURL, apiKey, model string,
	timeout time.Duration,
	messages []ChatLLMMessage,
	tools []ChatTool,
	onDelta func(text string),
) (ChatLLMResult, error) {
	if AIAPIFormatOrDefault(apiFormat) == AIAPIFormatAnthropic {
		return streamChatCompletionsAnthropic(ctx, baseURL, apiKey, model, timeout, messages, tools, onDelta)
	}
	return streamChatCompletionsOpenAI(ctx, baseURL, apiKey, model, timeout, messages, tools, onDelta)
}

func buildAnthropicMessages(messages []ChatLLMMessage) (string, []anthropicMessage, error) {
	var system []string
	out := make([]anthropicMessage, 0, len(messages))
	for i := 0; i < len(messages); {
		msg := messages[i]
		switch strings.ToLower(strings.TrimSpace(msg.Role)) {
		case "system":
			if text := strings.TrimSpace(msg.Content); text != "" {
				system = append(system, text)
			}
			i++
		case "tool":
			var blocks []anthropicContentBlock
			for i < len(messages) && strings.EqualFold(strings.TrimSpace(messages[i].Role), "tool") {
				toolResult := messages[i]
				id := strings.TrimSpace(toolResult.ToolCallID)
				if id == "" {
					return "", nil, fmt.Errorf("tool result is missing tool_call_id")
				}
				content := toolResult.Content
				if strings.TrimSpace(content) == "" {
					content = "(tool returned no output)"
				}
				blocks = append(blocks, anthropicContentBlock{
					Type:      "tool_result",
					ToolUseID: id,
					Content:   content,
				})
				i++
			}
			out = appendAnthropicMessage(out, "user", blocks)
		case "assistant":
			blocks := make([]anthropicContentBlock, 0, 1+len(msg.ToolCalls))
			if text := strings.TrimSpace(msg.Content); text != "" {
				blocks = append(blocks, anthropicContentBlock{Type: "text", Text: text})
			}
			for _, call := range msg.ToolCalls {
				if strings.TrimSpace(call.ID) == "" || strings.TrimSpace(call.Function.Name) == "" {
					continue
				}
				blocks = append(blocks, anthropicContentBlock{
					Type:  "tool_use",
					ID:    call.ID,
					Name:  call.Function.Name,
					Input: anthropicToolInput(call.Function.Arguments),
				})
			}
			if len(blocks) > 0 {
				out = appendAnthropicMessage(out, "assistant", blocks)
			}
			i++
		default:
			text := msg.Content
			if strings.TrimSpace(text) == "" {
				text = "(empty message)"
			}
			out = appendAnthropicMessage(out, "user", []anthropicContentBlock{{Type: "text", Text: text}})
			i++
		}
	}
	return strings.Join(system, "\n\n"), out, nil
}

func appendAnthropicMessage(out []anthropicMessage, role string, blocks []anthropicContentBlock) []anthropicMessage {
	if len(blocks) == 0 {
		return out
	}
	if len(out) > 0 && out[len(out)-1].Role == role {
		out[len(out)-1].Content = append(out[len(out)-1].Content, blocks...)
		return out
	}
	return append(out, anthropicMessage{Role: role, Content: blocks})
}

func anthropicToolInput(raw string) json.RawMessage {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return json.RawMessage(`{}`)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil || obj == nil {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(raw)
}

func buildAnthropicTools(tools []ChatTool) []anthropicTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]anthropicTool, 0, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(tool.Function.Name)
		if name == "" {
			continue
		}
		schema := bytes.TrimSpace(tool.Function.Parameters)
		if len(schema) == 0 || !json.Valid(schema) {
			schema = []byte(`{"type":"object","properties":{}}`)
		}
		out = append(out, anthropicTool{
			Name:        name,
			Description: strings.TrimSpace(tool.Function.Description),
			InputSchema: json.RawMessage(schema),
		})
	}
	return out
}

func newAnthropicRequest(model, system string, messages []anthropicMessage, tools []ChatTool, maxTokens int, temperature float64, stream bool) anthropicMessagesRequest {
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	return anthropicMessagesRequest{
		Model:       strings.TrimSpace(model),
		MaxTokens:   maxTokens,
		System:      strings.TrimSpace(system),
		Messages:    messages,
		Tools:       buildAnthropicTools(tools),
		Temperature: temperature,
		Stream:      stream,
	}
}

func sendAnthropicRequest(ctx context.Context, baseURL, apiKey string, payload anthropicMessagesRequest, timeout time.Duration, stream bool) (*http.Response, string, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, "", fmt.Errorf("API key is required")
	}
	if strings.TrimSpace(payload.Model) == "" {
		return nil, "", fmt.Errorf("model is required")
	}
	baseURL = NormalizeAnthropicBaseURL(baseURL)
	if baseURL == "" {
		return nil, "", fmt.Errorf("base URL is required")
	}
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	endpoint := AnthropicMessagesEndpoint(baseURL)
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, endpoint, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, endpoint, err
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", anthropicAPIVersion)
	req.Header.Set("Content-Type", "application/json")
	for _, tool := range payload.Tools {
		if strings.HasPrefix(strings.ToLower(tool.Type), "web_search") {
			req.Header.Set("anthropic-beta", "web-search-2025-03-05")
			break
		}
	}
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, endpoint, fmt.Errorf("request to %s failed: %w", endpoint, err)
	}
	return resp, endpoint, nil
}

func parseAnthropicResponse(body []byte, status int, endpoint string, ctx context.Context) (ChatLLMResult, error) {
	out := ChatLLMResult{Raw: append(json.RawMessage(nil), body...)}
	var parsed anthropicMessagesResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		if status < 200 || status >= 300 {
			return out, formatAIHTTPError(status, endpoint, truncateForErr(string(body), 300))
		}
		return out, fmt.Errorf("invalid Anthropic response from %s: %w", endpoint, err)
	}
	if errMsg := extractAIErrorMessage(parsed.Error); errMsg != "" {
		return out, formatAIHTTPError(status, endpoint, errMsg)
	}
	if status < 200 || status >= 300 {
		return out, formatAIHTTPError(status, endpoint, truncateForErr(string(body), 300))
	}
	recordAIUsage(ctx, parsed.Usage.normalized())
	var text strings.Builder
	for _, block := range parsed.Content {
		switch block.Type {
		case "text":
			text.WriteString(block.Text)
		case "tool_use":
			args := strings.TrimSpace(string(block.Input))
			if args == "" || args == "null" {
				args = "{}"
			}
			out.ToolCalls = append(out.ToolCalls, ChatToolCall{
				ID:   block.ID,
				Type: "function",
				Function: ChatToolCallFunction{
					Name:      block.Name,
					Arguments: args,
				},
			})
		}
	}
	out.Content = strings.TrimSpace(text.String())
	out.FinishReason = mapAnthropicStopReason(parsed.StopReason)
	return out, nil
}

func mapAnthropicStopReason(reason string) string {
	switch strings.TrimSpace(reason) {
	case "tool_use":
		return "tool_calls"
	case "end_turn", "stop_sequence":
		return "stop"
	case "max_tokens":
		return "length"
	default:
		return reason
	}
}

func chatCompletionsAnthropic(ctx context.Context, baseURL, apiKey, model string, timeout time.Duration, messages []ChatLLMMessage, tools []ChatTool) (ChatLLMResult, error) {
	var out ChatLLMResult
	if strings.TrimSpace(apiKey) == "" {
		return out, fmt.Errorf("API key is required")
	}
	if strings.TrimSpace(model) == "" {
		return out, fmt.Errorf("model is required")
	}
	system, msgs, err := buildAnthropicMessages(messages)
	if err != nil {
		return out, err
	}
	payload := newAnthropicRequest(model, system, msgs, tools, 4096, 0.2, false)
	resp, endpoint, err := sendAnthropicRequest(ctx, baseURL, apiKey, payload, timeout, false)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return out, err
	}
	return parseAnthropicResponse(body, resp.StatusCode, endpoint, ctx)
}

func streamChatCompletionsAnthropic(ctx context.Context, baseURL, apiKey, model string, timeout time.Duration, messages []ChatLLMMessage, tools []ChatTool, onDelta func(string)) (ChatLLMResult, error) {
	var out ChatLLMResult
	if strings.TrimSpace(apiKey) == "" {
		return out, fmt.Errorf("API key is required")
	}
	if strings.TrimSpace(model) == "" {
		return out, fmt.Errorf("model is required")
	}
	system, msgs, err := buildAnthropicMessages(messages)
	if err != nil {
		return out, err
	}
	payload := newAnthropicRequest(model, system, msgs, tools, 4096, 0.2, true)
	resp, endpoint, err := sendAnthropicRequest(ctx, baseURL, apiKey, payload, timeout, true)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		var parsed anthropicMessagesResponse
		_ = json.Unmarshal(body, &parsed)
		if errMsg := extractAIErrorMessage(parsed.Error); errMsg != "" {
			return out, formatAIHTTPError(resp.StatusCode, endpoint, errMsg)
		}
		return out, formatAIHTTPError(resp.StatusCode, endpoint, truncateForErr(string(body), 400))
	}

	type toolBuffer struct {
		call ChatToolCall
		args strings.Builder
	}
	var content strings.Builder
	var reasoning strings.Builder
	toolsByIndex := map[int]*toolBuffer{}
	finishReason := ""
	usage := AITokenUsage{}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var event anthropicStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			if strings.Contains(data, `"error"`) {
				return out, fmt.Errorf("invalid Anthropic stream event from %s: %w", endpoint, err)
			}
			continue
		}
		if errMsg := extractAIErrorMessage(event.Error); errMsg != "" {
			return out, formatAIHTTPError(resp.StatusCode, endpoint, errMsg)
		}
		switch event.Type {
		case "message_start":
			mergeAIUsage(&usage, event.Message.Usage.normalized())
			if event.Message.StopReason != "" {
				finishReason = mapAnthropicStopReason(event.Message.StopReason)
			}
		case "content_block_start":
			switch event.ContentBlock.Type {
			case "tool_use":
				toolsByIndex[event.Index] = &toolBuffer{call: ChatToolCall{
					ID:   event.ContentBlock.ID,
					Type: "function",
					Function: ChatToolCallFunction{
						Name: event.ContentBlock.Name,
					},
				}}
			case "text":
				if event.ContentBlock.Text != "" {
					content.WriteString(event.ContentBlock.Text)
					if onDelta != nil {
						onDelta(event.ContentBlock.Text)
					}
				}
			}
		case "content_block_delta":
			switch event.Delta.Type {
			case "text_delta":
				content.WriteString(event.Delta.Text)
				if onDelta != nil && event.Delta.Text != "" {
					onDelta(event.Delta.Text)
				}
			case "input_json_delta":
				if buf := toolsByIndex[event.Index]; buf != nil {
					buf.args.WriteString(event.Delta.PartialJSON)
				}
			case "thinking_delta":
				reasoning.WriteString(event.Delta.Thinking)
			}
		case "message_delta":
			if event.Delta.StopReason != "" {
				finishReason = mapAnthropicStopReason(event.Delta.StopReason)
			}
			mergeAIUsage(&usage, event.Usage.normalized())
		case "message_stop":
			// End of stream.
		case "error":
			if errMsg := extractAIErrorMessage(event.Error); errMsg != "" {
				return out, formatAIHTTPError(resp.StatusCode, endpoint, errMsg)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return out, fmt.Errorf("Anthropic stream read failed: %w", err)
	}
	indexes := make([]int, 0, len(toolsByIndex))
	for index := range toolsByIndex {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	for _, index := range indexes {
		buf := toolsByIndex[index]
		args := strings.TrimSpace(buf.args.String())
		if args == "" || args == "null" {
			args = "{}"
		}
		buf.call.Function.Arguments = args
		out.ToolCalls = append(out.ToolCalls, buf.call)
	}
	out.Content = strings.TrimSpace(content.String())
	out.ReasoningContent = reasoning.String()
	out.FinishReason = finishReason
	recordAIUsage(ctx, usage)
	return out, nil
}

func mergeAIUsage(dst *AITokenUsage, src AITokenUsage) {
	if src.PromptTokens > 0 {
		dst.PromptTokens = src.PromptTokens
	}
	if src.CompletionTokens > 0 {
		dst.CompletionTokens = src.CompletionTokens
	}
	dst.TotalTokens = dst.PromptTokens + dst.CompletionTokens
}

// CompleteAITextFormat performs a single-turn text completion using the
// selected wire format. It is used by page mirroring and info-gather synthesis.
func CompleteAITextFormat(ctx context.Context, apiFormat, baseURL, apiKey, model string, timeout time.Duration, systemPrompt, userPrompt string, maxTokens int, temperature float64) (AITextResult, error) {
	out := AITextResult{}
	if strings.TrimSpace(apiKey) == "" {
		return out, fmt.Errorf("API key is required")
	}
	if strings.TrimSpace(model) == "" {
		return out, fmt.Errorf("model is required")
	}
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	if AIAPIFormatOrDefault(apiFormat) == AIAPIFormatAnthropic {
		system, msgs, err := buildAnthropicMessages([]ChatLLMMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		})
		if err != nil {
			return out, err
		}
		payload := newAnthropicRequest(model, system, msgs, nil, maxTokens, temperature, false)
		resp, endpoint, err := sendAnthropicRequest(ctx, baseURL, apiKey, payload, timeout, false)
		if err != nil {
			return out, err
		}
		defer resp.Body.Close()
		out.Endpoint = endpoint
		body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		if err != nil {
			return out, err
		}
		out.Raw = append(json.RawMessage(nil), body...)
		result, err := parseAnthropicResponse(body, resp.StatusCode, endpoint, ctx)
		if err != nil {
			return out, err
		}
		out.Text = result.Content
		return out, nil
	}

	baseURL = NormalizeOpenAICompatibleBaseURL(baseURL)
	if baseURL == "" {
		return out, fmt.Errorf("base URL is required")
	}
	out.Endpoint = baseURL + "/chat/completions"
	payload := chatCompletionRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: temperature,
		MaxTokens:   maxTokens,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return out, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, out.Endpoint, bytes.NewReader(raw))
	if err != nil {
		return out, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return out, fmt.Errorf("request to %s failed: %w", out.Endpoint, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return out, err
	}
	out.Raw = append(json.RawMessage(nil), body...)
	var parsed chatCompletionResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return out, formatAIHTTPError(resp.StatusCode, out.Endpoint, truncateForErr(string(body), 300))
		}
		return out, fmt.Errorf("invalid AI response from %s: %w", out.Endpoint, err)
	}
	if errMsg := extractAIErrorMessage(parsed.Error); errMsg != "" {
		return out, formatAIHTTPError(resp.StatusCode, out.Endpoint, errMsg)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return out, formatAIHTTPError(resp.StatusCode, out.Endpoint, truncateForErr(string(body), 300))
	}
	recordAIUsage(ctx, parsed.Usage.normalized())
	if len(parsed.Choices) == 0 {
		return out, fmt.Errorf("empty choices from %s", out.Endpoint)
	}
	out.Text = strings.TrimSpace(parsed.Choices[0].Message.Content)
	return out, nil
}

// ProbeChatCompletionsFormat verifies the selected wire format.
func ProbeChatCompletionsFormat(ctx context.Context, apiFormat, baseURL, apiKey, model string, timeout time.Duration) (ProbeChatCompletionsResult, error) {
	if AIAPIFormatOrDefault(apiFormat) == AIAPIFormatAnthropic {
		return probeChatCompletionsAnthropic(ctx, baseURL, apiKey, model, timeout)
	}
	return probeChatCompletionsOpenAI(ctx, baseURL, apiKey, model, timeout)
}

func probeChatCompletionsAnthropic(ctx context.Context, baseURL, apiKey, model string, timeout time.Duration) (ProbeChatCompletionsResult, error) {
	out := ProbeChatCompletionsResult{Model: strings.TrimSpace(model)}
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
		timeout = 30 * time.Second
	}
	system, msgs, err := buildAnthropicMessages([]ChatLLMMessage{{Role: "user", Content: "ping"}})
	if err != nil {
		return out, err
	}
	payload := newAnthropicRequest(model, system, msgs, nil, 8, 0, false)
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
	if _, err := parseAnthropicResponse(body, resp.StatusCode, endpoint, ctx); err != nil {
		return out, err
	}
	return out, nil
}
