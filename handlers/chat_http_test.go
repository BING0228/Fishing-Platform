package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"spearhook/models"

	"github.com/gin-gonic/gin"
)

func TestSendChatMessageStreamsSSE(t *testing.T) {
	db := newChatTestDB(t)
	user := models.User{Username: "http-op", Password: "x", Role: models.UserRoleOperator}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	session := models.ChatSession{Title: "t", CreatedBy: user.ID}
	if err := db.Create(&session).Error; err != nil {
		t.Fatal(err)
	}

	var calls int32
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if atomic.AddInt32(&calls, 1) == 1 {
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"c1\",\"type\":\"function\",\"function\":{\"name\":\"list_agents\",\"arguments\":\"{}\"}}]},\"finish_reason\":null}]}\n\n")
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n")
		} else {
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"all good\"},\"finish_reason\":null}]}\n\n")
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer llm.Close()

	profile := models.AIProfile{ID: "p1", Name: "p", Enabled: true, BaseURL: llm.URL, APIKey: "k", Model: "m", TimeoutSec: 60}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user", user)
		c.Next()
	})
	router.POST("/api/chat/sessions/:id/messages/", SendChatMessage)

	srv := httptest.NewServer(router)
	defer srv.Close()

	payload, _ := json.Marshal(map[string]string{"content": "list agents"})
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/chat/sessions/%d/messages/", srv.URL, session.ID), strings.NewReader(string(payload)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, raw)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{"event: thinking", "event: tool_start", "event: tool_result", "event: token", "event: done"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in SSE stream:\n%s", want, text)
		}
	}
	if !strings.Contains(text, "all good") {
		t.Fatalf("streamed assistant content missing:\n%s", text)
	}

	// The assistant turn and tool turn must be persisted for replay.
	var msgs []models.ChatMessage
	if err := db.Where("session_id = ?", session.ID).Order("id ASC").Find(&msgs).Error; err != nil {
		t.Fatal(err)
	}
	roles := map[string]bool{}
	for _, m := range msgs {
		roles[m.Role] = true
	}
	if !roles["user"] || !roles["assistant"] || !roles["tool"] {
		t.Fatalf("persisted roles = %v", roles)
	}
}
