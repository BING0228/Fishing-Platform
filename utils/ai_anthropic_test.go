package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNormalizeAnthropicBaseURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://api.anthropic.com", "https://api.anthropic.com/v1"},
		{"https://api.anthropic.com/", "https://api.anthropic.com/v1"},
		{"https://api.anthropic.com/v1", "https://api.anthropic.com/v1"},
		{"https://api.anthropic.com/v1/", "https://api.anthropic.com/v1"},
		{"https://api.anthropic.com/v1/messages", "https://api.anthropic.com/v1"},
		{"https://gateway.example.com/anthropic/v1/messages/", "https://gateway.example.com/anthropic/v1"},
	}
	for _, tc := range cases {
		if got := NormalizeAnthropicBaseURL(tc.in); got != tc.want {
			t.Fatalf("NormalizeAnthropicBaseURL(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
	if got := AnthropicMessagesEndpoint("https://api.anthropic.com"); got != "https://api.anthropic.com/v1/messages" {
		t.Fatalf("endpoint=%q", got)
	}
}

func TestAnthropicChatToolUse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path=%s", r.URL.Path)
		}
		if got := r.Header.Get("x-api-key"); got != "secret" {
			t.Errorf("x-api-key=%q", got)
		}
		if got := r.Header.Get("anthropic-version"); got != anthropicAPIVersion {
			t.Errorf("anthropic-version=%q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["system"] != "You are a test." {
			t.Errorf("system=%v", body["system"])
		}
		tools, _ := body["tools"].([]any)
		if len(tools) != 1 {
			t.Errorf("tools=%v", body["tools"])
		} else {
			tool := tools[0].(map[string]any)
			if tool["name"] != "list_agents" || tool["input_schema"] == nil {
				t.Errorf("tool=%v", tool)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"content":[{"type":"text","text":"Checking."},{"type":"tool_use","id":"toolu_1","name":"list_agents","input":{"limit":5}}],"stop_reason":"tool_use","usage":{"input_tokens":12,"output_tokens":7}}`)
	}))
	defer srv.Close()

	res, err := ChatCompletionsWithToolsFormat(
		context.Background(), AIAPIFormatAnthropic, srv.URL+"/v1", "secret", "claude-test", 5*time.Second,
		[]ChatLLMMessage{{Role: "system", Content: "You are a test."}, {Role: "user", Content: "List agents."}},
		[]ChatTool{{
			Type: "function",
			Function: ChatToolFunction{
				Name:        "list_agents",
				Description: "List agents",
				Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
			},
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if res.Content != "Checking." || res.FinishReason != "tool_calls" {
		t.Fatalf("result=%+v", res)
	}
	if len(res.ToolCalls) != 1 || res.ToolCalls[0].ID != "toolu_1" || res.ToolCalls[0].Function.Name != "list_agents" || res.ToolCalls[0].Function.Arguments != `{"limit":5}` {
		t.Fatalf("tool calls=%+v", res.ToolCalls)
	}
}

func TestAnthropicToolResultConversion(t *testing.T) {
	system, messages, err := buildAnthropicMessages([]ChatLLMMessage{
		{Role: "system", Content: "system"},
		{Role: "user", Content: "go"},
		{Role: "assistant", Content: "calling", ToolCalls: []ChatToolCall{{
			ID: "toolu_1", Type: "function",
			Function: ChatToolCallFunction{Name: "list_agents", Arguments: `{"limit":5}`},
		}}},
		{Role: "tool", ToolCallID: "toolu_1", Content: "[]"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if system != "system" {
		t.Fatalf("system=%q", system)
	}
	if len(messages) != 3 {
		t.Fatalf("messages=%+v", messages)
	}
	if messages[1].Content[1].Type != "tool_use" || string(messages[1].Content[1].Input) != `{"limit":5}` {
		t.Fatalf("tool use=%+v", messages[1].Content[1])
	}
	if messages[2].Role != "user" || messages[2].Content[0].Type != "tool_result" || messages[2].Content[0].ToolUseID != "toolu_1" {
		t.Fatalf("tool result=%+v", messages[2])
	}
}

func TestAnthropicStreamTextAndTool(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, event := range []string{
			`{"type":"message_start","message":{"usage":{"input_tokens":4}}}`,
			`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello "}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"world"}}`,
			`{"type":"content_block_stop","index":0}`,
			`{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_1","name":"list_agents"}}`,
			`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"limit\":"}}`,
			`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"5}"}}`,
			`{"type":"content_block_stop","index":1}`,
			`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":9}}`,
			`{"type":"message_stop"}`,
		} {
			fmt.Fprintf(w, "event: x\ndata: %s\n\n", event)
		}
	}))
	defer srv.Close()

	var got strings.Builder
	res, err := StreamChatCompletionsWithToolsFormat(
		context.Background(), AIAPIFormatAnthropic, srv.URL, "secret", "claude-test", 5*time.Second,
		[]ChatLLMMessage{{Role: "user", Content: "go"}}, nil,
		func(text string) { got.WriteString(text) },
	)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "Hello world" || res.Content != "Hello world" {
		t.Fatalf("content=%q res=%q", got.String(), res.Content)
	}
	if len(res.ToolCalls) != 1 || res.ToolCalls[0].Function.Arguments != `{"limit":5}` {
		t.Fatalf("tools=%+v", res.ToolCalls)
	}
	if res.FinishReason != "tool_calls" {
		t.Fatalf("finish=%q", res.FinishReason)
	}
}

func TestCompleteAITextAnthropic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path=%s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "secret" {
			t.Errorf("missing x-api-key")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["system"] != "system" {
			t.Errorf("system=%v", body["system"])
		}
		if body["max_tokens"] != float64(128) {
			t.Errorf("max_tokens=%v", body["max_tokens"])
		}
		fmt.Fprint(w, `{"content":[{"type":"text","text":"<html>ok</html>"}],"stop_reason":"end_turn","usage":{"input_tokens":5,"output_tokens":3}}`)
	}))
	defer srv.Close()

	result, err := CompleteAITextFormat(context.Background(), AIAPIFormatAnthropic, srv.URL+"/v1", "secret", "claude-test", 5*time.Second, "system", "user", 128, 0)
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "<html>ok</html>" || result.Endpoint != srv.URL+"/v1/messages" {
		t.Fatalf("result=%+v", result)
	}
}

func TestAnthropicWebSearchGather(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path=%s", r.URL.Path)
		}
		if r.Header.Get("anthropic-beta") != "web-search-2025-03-05" {
			t.Errorf("missing web search beta header: %q", r.Header.Get("anthropic-beta"))
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		tools, _ := body["tools"].([]any)
		if len(tools) != 1 {
			t.Fatalf("tools=%v", body["tools"])
		}
		if tools[0].(map[string]any)["type"] != "web_search_20250305" {
			t.Fatalf("tool=%v", tools[0])
		}
		fmt.Fprint(w, `{"content":[{"type":"text","text":"{\"findings\":[{\"kind\":\"email\",\"value\":\"security@example.com\",\"source_url\":\"https://example.com/contact\",\"snippet\":\"contact\"}]}"}],"stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":20}}`)
	}))
	defer srv.Close()

	result, err := GatherContactsWithAIFormat(context.Background(), AIAPIFormatAnthropic, srv.URL+"/v1", "secret", "claude-test", 5*time.Second, InfoGatherInput{Target: "example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) != 1 || result.Findings[0].Value != "security@example.com" {
		t.Fatalf("findings=%+v", result.Findings)
	}
}
