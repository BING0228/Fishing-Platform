package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"spearhook/chattools"
	"spearhook/config"
	"spearhook/models"

	"github.com/gin-gonic/gin"
)

type chatToolCallRef struct {
	Name string
	Args string
}

func findChatToolCall(sessionID uint, toolCallID string) (chatToolCallRef, bool) {
	var rows []models.ChatMessage
	if err := config.GetDB().
		Where("session_id = ? AND role = ?", sessionID, "assistant").
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return chatToolCallRef{}, false
	}
	for _, row := range rows {
		if strings.TrimSpace(row.ToolCallsJSON) == "" {
			continue
		}
		var calls []struct {
			ID       string `json:"id"`
			Function struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			} `json:"function"`
		}
		if json.Unmarshal([]byte(row.ToolCallsJSON), &calls) != nil {
			continue
		}
		for _, call := range calls {
			if call.ID == toolCallID {
				return chatToolCallRef{Name: call.Function.Name, Args: call.Function.Arguments}, true
			}
		}
	}
	return chatToolCallRef{}, false
}

// upsertToolMessage replaces the stored result for a tool_call (retry must not
// create a second tool result for the same call id).
func upsertToolMessage(sessionID uint, toolCallID, name string, result chattools.Result) {
	db := config.GetDB()
	raw, _ := json.Marshal(result)
	status := "done"
	if !result.OK {
		status = "error"
	}
	var row models.ChatMessage
	err := db.Where("session_id = ? AND role = ? AND tool_call_id = ?", sessionID, "tool", toolCallID).
		Order("id DESC").First(&row).Error
	if err == nil {
		_ = db.Model(&row).Updates(map[string]any{
			"content":   string(raw),
			"tool_name": name,
			"status":    status,
		}).Error
		return
	}
	_ = db.Create(&models.ChatMessage{
		SessionID:  sessionID,
		Role:       "tool",
		Content:    string(raw),
		ToolCallID: toolCallID,
		ToolName:   name,
		Status:     status,
	}).Error
}

// RetryChatTool re-executes a previous tool_call and resumes the turn (SSE).
func RetryChatTool(c *gin.Context) {
	user, ok := currentUserOrAbort(c)
	if !ok {
		return
	}
	session, ok := loadOwnedChatSession(c, user)
	if !ok {
		return
	}
	if strings.TrimSpace(session.PendingToolCallID) != "" {
		c.JSON(http.StatusConflict, gin.H{"error": "A write action is awaiting confirmation. Confirm or reject it first."})
		return
	}
	toolCallID := strings.TrimSpace(c.Param("tool_call_id"))
	if toolCallID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tool_call_id is required"})
		return
	}
	call, found := findChatToolCall(session.ID, toolCallID)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tool call not found"})
		return
	}
	profile, apiKey, active := ActiveAIConfig()
	if !active || strings.TrimSpace(apiKey) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "AI profile unavailable"})
		return
	}

	emit, okFlush := beginChatSSE(c)
	if !okFlush {
		return
	}

	emit("tool_start", gin.H{"tool_call_id": toolCallID, "name": call.Name, "arguments": call.Args})
	ec := chattools.ExecContext{
		Ctx:        c.Request.Context(),
		User:       user,
		SessionID:  session.ID,
		ToolCallID: toolCallID,
		Emit:       emit,
	}
	result := chattools.Execute(ec, call.Name, call.Args)
	upsertToolMessage(session.ID, toolCallID, call.Name, result)
	emitToolResult(emit, toolCallID, call.Name, result)

	runChatOrchestrator(c.Request.Context(), emit, user, &session, profile, apiKey, true, "")
}
