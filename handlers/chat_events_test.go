package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"spearhook/chatbus"
	"spearhook/models"

	"github.com/gin-gonic/gin"
)

func TestStreamChatEventsDeliversJobProgress(t *testing.T) {
	db := newChatTestDB(t)
	user := models.User{Username: "events-op", Password: "x", Role: models.UserRoleOperator}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	session := models.ChatSession{Title: "t", CreatedBy: user.ID}
	if err := db.Create(&session).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user", user)
		c.Next()
	})
	router.GET("/api/chat/sessions/:id/events/", StreamChatEvents)
	srv := httptest.NewServer(router)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/chat/sessions/%d/events/", srv.URL, session.ID), nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(line, "event: ready") {
			break
		}
	}

	chatbus.Publish(session.ID, "job_progress", map[string]any{
		"job_id":  uint(3),
		"percent": 42,
		"message": "searching",
	})

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(line, "event: job_progress") {
			continue
		}
		data, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(strings.TrimPrefix(strings.TrimSpace(data), "data: ")), &payload); err != nil {
			t.Fatalf("bad payload %q: %v", data, err)
		}
		if payload["job_id"] != float64(3) || payload["percent"] != float64(42) {
			t.Fatalf("payload = %+v", payload)
		}
		return
	}
}

func TestRetryChatToolHTTP(t *testing.T) {
	db := newChatTestDB(t)
	user := models.User{Username: "retry-op", Password: "x", Role: models.UserRoleOperator}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	session := models.ChatSession{Title: "t", CreatedBy: user.ID}
	if err := db.Create(&session).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ChatMessage{
		SessionID:     session.ID,
		Role:          "assistant",
		ToolCallsJSON: `[{"id":"c1","type":"function","function":{"name":"list_agents","arguments":"{}"}}]`,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ChatMessage{
		SessionID:  session.ID,
		Role:       "tool",
		Content:    `{"ok":false,"error":"boom"}`,
		ToolCallID: "c1",
		ToolName:   "list_agents",
		Status:     "error",
	}).Error; err != nil {
		t.Fatal(err)
	}

	srv := streamServer(t, ""+
		"data: {\"choices\":[{\"delta\":{\"content\":\"recovered\"},\"finish_reason\":null}]}\n\n"+
		"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"+
		"data: [DONE]\n\n")
	defer srv.Close()
	if err := db.Create(&models.AIProfile{
		ID: "p1", Name: "p", Enabled: true, BaseURL: srv.URL, APIKey: "k", Model: "m", TimeoutSec: 60,
	}).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user", user)
		c.Next()
	})
	router.POST("/api/chat/sessions/:id/tools/:tool_call_id/retry/", RetryChatTool)
	httpSrv := httptest.NewServer(router)
	defer httpSrv.Close()

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/chat/sessions/%d/tools/c1/retry/", httpSrv.URL, session.ID), strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{"event: tool_result", "recovered", "event: done"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in retry stream:\n%s", want, text)
		}
	}

	var row models.ChatMessage
	if err := db.Where("session_id = ? AND tool_call_id = ?", session.ID, "c1").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Status != "done" {
		t.Fatalf("retried tool status = %q", row.Status)
	}
}
