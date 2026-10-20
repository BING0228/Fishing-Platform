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

func sseServer(t *testing.T, chunks []string, status int, errBody string) *httptest.Server {
	t.Helper()
	if status == 0 {
		status = http.StatusOK
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(status)
		if errBody != "" {
			_, _ = w.Write([]byte(errBody))
			return
		}
		for _, c := range chunks {
			fmt.Fprintf(w, "data: %s\n\n", c)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
}

func TestStreamContentDeltas(t *testing.T) {
	chunks := []string{
		`{"choices":[{"delta":{"content":"Hello "},"finish_reason":null}]}`,
		`{"choices":[{"delta":{"content":"world"},"finish_reason":null}]}`,
		`{"choices":[{"delta":{},"finish_reason":"stop"}]}`,
	}
	srv := sseServer(t, chunks, 0, "")
	defer srv.Close()

	var got strings.Builder
	res, err := StreamChatCompletionsWithTools(
		context.Background(), srv.URL, "k", "m", 5*time.Second,
		[]ChatLLMMessage{{Role: "user", Content: "hi"}}, nil,
		func(text string) { got.WriteString(text) },
	)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "Hello world" {
		t.Fatalf("deltas = %q", got.String())
	}
	if res.Content != "Hello world" {
		t.Fatalf("content = %q", res.Content)
	}
	if res.FinishReason != "stop" {
		t.Fatalf("finish_reason = %q", res.FinishReason)
	}
}

func TestStreamToolCallFragmentsMerge(t *testing.T) {
	chunks := []string{
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"list_agents","arguments":"{\"a\":"}}]},"finish_reason":null}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"1}"}}]},"finish_reason":null}]}`,
		`{"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
	}
	srv := sseServer(t, chunks, 0, "")
	defer srv.Close()

	res, err := StreamChatCompletionsWithTools(
		context.Background(), srv.URL, "k", "m", 5*time.Second, nil, nil, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d", len(res.ToolCalls))
	}
	call := res.ToolCalls[0]
	if call.ID != "call_1" || call.Function.Name != "list_agents" {
		t.Fatalf("call = %+v", call)
	}
	if call.Function.Arguments != `{"a":1}` {
		t.Fatalf("arguments = %q", call.Function.Arguments)
	}
	if res.FinishReason != "tool_calls" {
		t.Fatalf("finish_reason = %q", res.FinishReason)
	}
}

func TestStreamReasoningContentCaptured(t *testing.T) {
	chunks := []string{
		`{"choices":[{"delta":{"reasoning_content":"thinking..."},"finish_reason":null}]}`,
		`{"choices":[{"delta":{"content":"done"},"finish_reason":"stop"}]}`,
	}
	srv := sseServer(t, chunks, 0, "")
	defer srv.Close()

	res, err := StreamChatCompletionsWithTools(
		context.Background(), srv.URL, "k", "m", 5*time.Second, nil, nil, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if res.Content != "done" {
		t.Fatalf("content = %q", res.Content)
	}
	if res.ReasoningContent != "thinking..." {
		t.Fatalf("reasoning = %q", res.ReasoningContent)
	}
}

func TestStreamHTTPError(t *testing.T) {
	srv := sseServer(t, nil, http.StatusBadRequest, `{"error":{"message":"bad key"}}`)
	defer srv.Close()

	_, err := StreamChatCompletionsWithTools(
		context.Background(), srv.URL, "k", "m", 5*time.Second, nil, nil, nil,
	)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestStreamChatRecordsUsage(t *testing.T) {
	chunks := []string{
		`{"choices":[{"delta":{"content":"hi"},"finish_reason":null}]}`,
		`{"choices":[],"usage":{"prompt_tokens":4,"completion_tokens":2,"total_tokens":6}}`,
	}
	srv := sseServer(t, chunks, 0, "")
	defer srv.Close()

	prev := aiUsageRecorder
	defer SetAIUsageRecorder(prev)
	var gotProfile string
	var gotUsage AITokenUsage
	SetAIUsageRecorder(func(profileID string, usage AITokenUsage) {
		gotProfile = profileID
		gotUsage = usage
	})

	ctx := WithAIProfileID(context.Background(), "profile-1")
	res, err := StreamChatCompletionsWithTools(
		ctx, srv.URL, "k", "m", 5*time.Second,
		[]ChatLLMMessage{{Role: "user", Content: "hi"}}, nil, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if res.Content != "hi" {
		t.Fatalf("content = %q", res.Content)
	}
	if gotProfile != "profile-1" {
		t.Fatalf("profile = %q", gotProfile)
	}
	if gotUsage.PromptTokens != 4 || gotUsage.CompletionTokens != 2 || gotUsage.TotalTokens != 6 {
		t.Fatalf("usage = %+v", gotUsage)
	}
}
