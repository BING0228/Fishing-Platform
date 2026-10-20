package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"spearhook/models"
)

func TestSummarizeChatWithAI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"Cloned page #7; job #9 running."}}]}`)
	}))
	defer srv.Close()

	rows := []models.ChatMessage{
		{Role: "user", Content: "clone example.com"},
		{Role: "assistant", Content: "cloned"},
	}
	summary, err := summarizeChatWithAI(context.Background(), models.AIProfile{BaseURL: srv.URL, Model: "m"}, "k", 10*time.Second, rows, "earlier work")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(summary, "job #9") {
		t.Fatalf("summary = %q", summary)
	}
}
