package handlers

import (
	"strings"
	"testing"

	"spearhook/chattools"
	"spearhook/models"
)

func TestFindChatToolCall(t *testing.T) {
	db := newChatTestDB(t)
	session := models.ChatSession{Title: "t", CreatedBy: 1}
	if err := db.Create(&session).Error; err != nil {
		t.Fatal(err)
	}
	asst := models.ChatMessage{
		SessionID:     session.ID,
		Role:          "assistant",
		ToolCallsJSON: `[{"id":"call_1","type":"function","function":{"name":"clone_page","arguments":"{\"url\":\"https://x\"}"}}]`,
	}
	if err := db.Create(&asst).Error; err != nil {
		t.Fatal(err)
	}

	call, ok := findChatToolCall(session.ID, "call_1")
	if !ok || call.Name != "clone_page" || !strings.Contains(call.Args, "https://x") {
		t.Fatalf("call = %+v ok=%v", call, ok)
	}
	if _, ok := findChatToolCall(session.ID, "missing"); ok {
		t.Fatal("unknown tool_call must not be found")
	}
}

func TestUpsertToolMessageUpdatesInPlace(t *testing.T) {
	db := newChatTestDB(t)
	session := models.ChatSession{Title: "t", CreatedBy: 1}
	if err := db.Create(&session).Error; err != nil {
		t.Fatal(err)
	}

	upsertToolMessage(session.ID, "call_1", "list_agents", chattools.Result{OK: false, Error: "boom"})
	var rows []models.ChatMessage
	if err := db.Where("session_id = ? AND tool_call_id = ?", session.ID, "call_1").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Status != "error" {
		t.Fatalf("rows = %+v", rows)
	}

	upsertToolMessage(session.ID, "call_1", "list_agents", chattools.Result{OK: true, Summary: "ok"})
	rows = nil
	if err := db.Where("session_id = ? AND tool_call_id = ?", session.ID, "call_1").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Status != "done" {
		t.Fatalf("retry must update in place, rows = %+v", rows)
	}
}
