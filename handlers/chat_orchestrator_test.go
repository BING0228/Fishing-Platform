package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"spearhook/config"
	"spearhook/models"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newChatTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "chat.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := models.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	config.DB = db
	return db
}

type capturedEvent struct {
	name    string
	payload map[string]any
}

func captureEmit() (func(string, any), *[]capturedEvent) {
	var mu sync.Mutex
	events := &[]capturedEvent{}
	emit := func(name string, payload any) {
		mu.Lock()
		defer mu.Unlock()
		var m map[string]any
		switch typed := payload.(type) {
		case gin.H:
			m = typed
		case map[string]any:
			m = typed
		}
		*events = append(*events, capturedEvent{name: name, payload: m})
	}
	return emit, events
}

func streamServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(body))
	}))
}

func hasEvent(events []capturedEvent, name string) bool {
	for _, ev := range events {
		if ev.name == name {
			return true
		}
	}
	return false
}

func TestOrchestratorRequestsConfirmationForDestructiveTool(t *testing.T) {
	db := newChatTestDB(t)
	user := models.User{Username: "op", Password: "x", Role: models.UserRoleOperator}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	session := models.ChatSession{Title: "t", CreatedBy: user.ID}
	if err := db.Create(&session).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ChatMessage{SessionID: session.ID, Role: "user", Content: "send it"}).Error; err != nil {
		t.Fatal(err)
	}

	srv := streamServer(t, ""+
		"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_9\",\"type\":\"function\",\"function\":{\"name\":\"send_mail_campaign\",\"arguments\":\"{\\\"campaign_id\\\":1}\"}}]},\"finish_reason\":null}]}\n\n"+
		"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n"+
		"data: [DONE]\n\n")
	defer srv.Close()

	emit, events := captureEmit()
	profile := models.AIProfile{BaseURL: srv.URL, Model: "m", TimeoutSec: 60}

	runChatOrchestrator(context.Background(), emit, user, &session, profile, "k", false, "")

	if !hasEvent(*events, "confirm_required") {
		t.Fatalf("expected confirm_required, got %+v", *events)
	}
	if strings.TrimSpace(session.PendingToolCallID) != "call_9" {
		t.Fatalf("pending id = %q", session.PendingToolCallID)
	}
	if session.PendingToolName != "send_mail_campaign" {
		t.Fatalf("pending name = %q", session.PendingToolName)
	}
	for _, ev := range *events {
		if ev.name == "tool_start" || ev.name == "tool_result" {
			t.Fatalf("destructive tool must not run before confirmation: %+v", ev)
		}
	}
}

func TestOrchestratorFallsBackToNonStreaming(t *testing.T) {
	db := newChatTestDB(t)
	user := models.User{Username: "fallback", Password: "x", Role: models.UserRoleOperator}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	session := models.ChatSession{Title: "t", CreatedBy: user.ID}
	if err := db.Create(&session).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ChatMessage{SessionID: session.ID, Role: "user", Content: "hi"}).Error; err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if bytes.Contains(raw, []byte(`"stream":true`)) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"message":"stream unsupported"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"fallback ok"},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()

	emit, events := captureEmit()
	profile := models.AIProfile{BaseURL: srv.URL, Model: "m", TimeoutSec: 60}
	runChatOrchestrator(context.Background(), emit, user, &session, profile, "k", false, "")

	var streamed string
	for _, ev := range *events {
		if ev.name == "token" {
			streamed += fmt.Sprint(ev.payload["content"])
		}
	}
	if !strings.Contains(streamed, "fallback ok") {
		t.Fatalf("expected fallback content, events=%+v", *events)
	}
	if !hasEvent(*events, "done") {
		t.Fatalf("expected done, events=%+v", *events)
	}
}

func TestOrchestratorExecutesApprovedDestructiveTool(t *testing.T) {
	db := newChatTestDB(t)
	user := models.User{Username: "op2", Password: "x", Role: models.UserRoleOperator}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	session := models.ChatSession{
		Title:             "t",
		CreatedBy:         user.ID,
		PendingToolCallID: "call_7",
		PendingToolName:   "send_mail_campaign",
		PendingToolArgs:   `{"campaign_id": 1}`,
	}
	if err := db.Create(&session).Error; err != nil {
		t.Fatal(err)
	}
	campaign := models.MailCampaign{
		SmtpServiceID: 1,
		Subject:       "s",
		BodyTemplate:  "b",
		Status:        models.MailCampaignStatusScheduled,
		LandingURL:    "https://example.com/login",
		PublicBaseURL: "http://127.0.0.1:8080",
		CreatedBy:     user.ID,
	}
	if err := db.Create(&campaign).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.MailRecipient{CampaignID: campaign.ID, Email: "a@example.com"}).Error; err != nil {
		t.Fatal(err)
	}
	session.PendingToolArgs = fmt.Sprintf(`{"campaign_id": %d}`, campaign.ID)
	if err := db.Model(&session).Update("pending_tool_args", session.PendingToolArgs).Error; err != nil {
		t.Fatal(err)
	}

	srv := streamServer(t, ""+
		"data: {\"choices\":[{\"delta\":{\"content\":\"queued\"},\"finish_reason\":null}]}\n\n"+
		"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"+
		"data: [DONE]\n\n")
	defer srv.Close()

	emit, events := captureEmit()
	profile := models.AIProfile{BaseURL: srv.URL, Model: "m", TimeoutSec: 60}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	runChatOrchestrator(ctx, emit, user, &session, profile, "k", true, "call_7")

	var toolResult map[string]any
	for _, ev := range *events {
		if ev.name == "tool_result" {
			toolResult = ev.payload
		}
	}
	if toolResult == nil {
		t.Fatalf("expected tool_result, got %+v", *events)
	}
	if toolResult["ok"] != true {
		t.Fatalf("tool_result not ok: %+v", toolResult)
	}
	if session.PendingToolCallID != "" {
		t.Fatalf("pending should be cleared, got %q", session.PendingToolCallID)
	}
	var stored models.MailCampaign
	if err := db.First(&stored, campaign.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.ScheduledAt == nil {
		t.Fatal("approved campaign should be queued for delivery")
	}
	if !hasEvent(*events, "done") {
		t.Fatalf("expected done event, got %+v", *events)
	}
}
