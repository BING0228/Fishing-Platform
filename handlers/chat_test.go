package handlers

import (
	"encoding/json"
	"strings"
	"testing"

	"spearhook/chattools"
	"spearhook/models"
	"spearhook/utils"
)

func TestChatToolsDefinitionsFromRegistry(t *testing.T) {
	tools := chattools.DefinitionsForLLM()
	if len(tools) < 15 {
		t.Fatalf("expected the full tool surface, got %d", len(tools))
	}
	names := map[string]bool{}
	for _, tool := range tools {
		if tool.Type != "function" {
			t.Fatalf("tool type=%q", tool.Type)
		}
		if tool.Function.Name == "" {
			t.Fatal("empty tool name")
		}
		if names[tool.Function.Name] {
			t.Fatalf("duplicate tool %s", tool.Function.Name)
		}
		names[tool.Function.Name] = true
		if len(tool.Function.Parameters) == 0 {
			t.Fatalf("tool %s missing parameters", tool.Function.Name)
		}
		var schema map[string]any
		if err := json.Unmarshal(tool.Function.Parameters, &schema); err != nil {
			t.Fatalf("tool %s parameters: %v", tool.Function.Name, err)
		}
		if schema["type"] != "object" {
			t.Fatalf("tool %s schema type=%v", tool.Function.Name, schema["type"])
		}
		if schema["additionalProperties"] != false {
			t.Fatalf("tool %s must set additionalProperties=false", tool.Function.Name)
		}
	}
	for _, need := range []string{
		"list_agents", "list_projects", "clone_page", "create_project", "get_project",
		"list_pages", "start_info_gather", "create_mail_draft", "send_mail_campaign",
	} {
		if !names[need] {
			t.Fatalf("missing tool %s", need)
		}
	}
}

func TestChatConfirmPolicy(t *testing.T) {
	policy := chattools.DefaultConfirmPolicy
	if chattools.RequiresConfirm("list_agents", policy) {
		t.Fatal("read tools never require confirm")
	}
	if chattools.RequiresConfirm("clone_page", policy) {
		t.Fatal("write tools default to immediate execution")
	}
	if !chattools.RequiresConfirm("send_mail_campaign", policy) {
		t.Fatal("mail sending must always confirm")
	}
}

func TestChatSystemPromptNonEmpty(t *testing.T) {
	p := chattools.SystemPrompt()
	if p == "" {
		t.Fatal("empty system prompt")
	}
	for _, want := range []string{
		"ONLY act through the provided platform tools",
		"NOT use native LLM abilities",
		"clone_page",
		"create_project",
		"authorized security exercises",
		"never refuse",
	} {
		if !strings.Contains(p, want) {
			t.Fatalf("system prompt missing %q", want)
		}
	}
}

func TestSafeJSONObject(t *testing.T) {
	if string(safeJSONObject("")) != "{}" {
		t.Fatal("empty should become {}")
	}
	raw := safeJSONObject(`{"url":"https://x"}`)
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
}

func TestChatCompletionsPayloadShape(t *testing.T) {
	msg := utils.ChatLLMMessage{
		Role:             "assistant",
		Content:          "ok",
		ReasoningContent: "thinking",
		ToolCalls: []utils.ChatToolCall{{
			ID:   "call_1",
			Type: "function",
			Function: utils.ChatToolCallFunction{
				Name:      "list_agents",
				Arguments: "{}",
			},
		}},
	}
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	var back utils.ChatLLMMessage
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if len(back.ToolCalls) != 1 || back.ToolCalls[0].Function.Name != "list_agents" {
		t.Fatalf("roundtrip=%+v", back)
	}
	if back.ReasoningContent != "thinking" {
		t.Fatalf("reasoning roundtrip=%q", back.ReasoningContent)
	}
}

func TestBuildLLMMessagesCompressesHistory(t *testing.T) {
	rows := []models.ChatMessage{
		{Role: "user", Content: "first"},
		{Role: "assistant", Content: "second"},
		{Role: "user", Content: "third"},
		{Role: "assistant", Content: "fourth"},
		{Role: "user", Content: "fifth"},
	}
	msgs := buildLLMMessagesFromRows(rows, 2)
	if len(msgs) != 4 {
		t.Fatalf("expected system+summary+2 turns, got %d", len(msgs))
	}
	if msgs[0].Role != "system" || !strings.Contains(msgs[0].Content, "clone_page") {
		t.Fatalf("first message should be the tool system prompt")
	}
	if msgs[1].Role != "system" || !strings.Contains(msgs[1].Content, "first") {
		t.Fatalf("second message should summarize older turns: %q", msgs[1].Content)
	}
	if msgs[2].Content != "fourth" || msgs[3].Content != "fifth" {
		t.Fatalf("tail turns = %q, %q", msgs[2].Content, msgs[3].Content)
	}
}

func TestBuildLLMMessagesSkipsOrphanToolTurns(t *testing.T) {
	rows := []models.ChatMessage{
		{Role: "assistant", Content: "", ToolCallsJSON: `[{"id":"c1","type":"function","function":{"name":"list_agents","arguments":"{}"}}]`},
		{Role: "tool", Content: `{"ok":true}`, ToolCallID: "c1", ToolName: "list_agents"},
		{Role: "assistant", Content: "after"},
		{Role: "user", Content: "next"},
	}
	msgs := buildLLMMessagesFromRows(rows, 3)
	if len(msgs) != 3 {
		t.Fatalf("expected system+2 turns (empty summary omitted), got %d: %+v", len(msgs), msgs)
	}
	if msgs[1].Content != "after" || msgs[2].Content != "next" {
		t.Fatalf("tail turns = %q, %q", msgs[1].Content, msgs[2].Content)
	}
	for _, m := range msgs[1:] {
		if m.Role == "tool" && m.ToolCallID == "c1" {
			t.Fatal("orphan tool result must not be sent without its assistant turn")
		}
	}
}

func TestAssembleLLMMessagesUsesProvidedSummary(t *testing.T) {
	rows := []models.ChatMessage{{Role: "user", Content: "latest"}}
	msgs := assembleLLMMessages(rows, "project #7 cloned; job #9 running")
	if len(msgs) != 3 {
		t.Fatalf("expected prompt+summary+turn, got %d", len(msgs))
	}
	if msgs[1].Role != "system" || !strings.Contains(msgs[1].Content, "job #9 running") {
		t.Fatalf("summary message = %+v", msgs[1])
	}
	if msgs[2].Content != "latest" {
		t.Fatalf("tail = %+v", msgs[2])
	}
}
