package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"spearhook/chattools"
	"spearhook/config"
	"spearhook/models"
	"spearhook/utils"

	"github.com/gin-gonic/gin"
)

const chatMaxUserMsgRunes = 8000

var (
	chatRateMu   sync.Mutex
	chatLastCall = map[uint]time.Time{}
	chatDayCount = map[uint]struct {
		day   string
		count int
	}{}
)

type chatSessionDTO struct {
	ID        uint      `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Pending   bool      `json:"pending_confirm"`
}

type chatMessageDTO struct {
	ID               uint             `json:"id"`
	Role             string           `json:"role"`
	Content          string           `json:"content"`
	ReasoningContent string           `json:"reasoning_content,omitempty"`
	ToolCallsJSON    string           `json:"tool_calls_json,omitempty"`
	ToolCallID       string           `json:"tool_call_id,omitempty"`
	ToolName         string           `json:"tool_name,omitempty"`
	Status           string           `json:"status,omitempty"`
	CreatedAt        time.Time        `json:"created_at"`
	Links            []chattools.Link `json:"links,omitempty"`
}

type chatSendRequest struct {
	Content string `json:"content"`
}

type chatConfirmRequest struct {
	ToolCallID string `json:"tool_call_id"`
	Approve    *bool  `json:"approve"`
}

// ListChatSessions returns the current user's conversations.
func ListChatSessions(c *gin.Context) {
	user, ok := currentUserOrAbort(c)
	if !ok {
		return
	}
	db := config.GetDB()
	var rows []models.ChatSession
	if err := db.Where("created_by = ?", user.ID).Order("updated_at DESC, id DESC").Limit(100).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list chat sessions"})
		return
	}
	out := make([]chatSessionDTO, 0, len(rows))
	for _, s := range rows {
		out = append(out, toChatSessionDTO(s))
	}
	c.JSON(http.StatusOK, out)
}

// CreateChatSession starts an empty conversation.
func CreateChatSession(c *gin.Context) {
	user, ok := currentUserOrAbort(c)
	if !ok {
		return
	}
	session := models.ChatSession{
		Title:     "新对话",
		CreatedBy: user.ID,
	}
	if err := config.GetDB().Create(&session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create chat session"})
		return
	}
	c.JSON(http.StatusCreated, toChatSessionDTO(session))
}

// GetChatMessages returns messages for one owned session.
func GetChatMessages(c *gin.Context) {
	user, ok := currentUserOrAbort(c)
	if !ok {
		return
	}
	session, ok := loadOwnedChatSession(c, user)
	if !ok {
		return
	}
	var rows []models.ChatMessage
	if err := config.GetDB().Where("session_id = ?", session.ID).Order("id ASC").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load messages"})
		return
	}
	out := make([]chatMessageDTO, 0, len(rows))
	for _, m := range rows {
		out = append(out, toChatMessageDTO(m))
	}
	c.JSON(http.StatusOK, gin.H{
		"session":  toChatSessionDTO(session),
		"messages": out,
	})
}

// DeleteChatSession removes a conversation and its messages.
func DeleteChatSession(c *gin.Context) {
	user, ok := currentUserOrAbort(c)
	if !ok {
		return
	}
	session, ok := loadOwnedChatSession(c, user)
	if !ok {
		return
	}
	db := config.GetDB()
	_ = db.Where("session_id = ?", session.ID).Delete(&models.ChatMessage{}).Error
	if err := db.Delete(&session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete session"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// SendChatMessage appends a user message and streams the assistant turn (SSE).
func SendChatMessage(c *gin.Context) {
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
	var req chatSendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
		return
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
		return
	}
	if utf8.RuneCountInString(content) > chatMaxUserMsgRunes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message too long"})
		return
	}
	if err := checkChatRateLimit(user.ID); err != nil {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		return
	}

	profile, apiKey, active := ActiveAIConfig()
	if !active {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No AI model is enabled. Open System → AI Settings to configure one."})
		return
	}
	if strings.TrimSpace(apiKey) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Enabled AI profile has no API key."})
		return
	}

	db := config.GetDB()
	userMsg := models.ChatMessage{
		SessionID: session.ID,
		Role:      "user",
		Content:   content,
		Status:    "done",
	}
	if err := db.Create(&userMsg).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save message"})
		return
	}
	if session.Title == "" || session.Title == "新对话" || session.Title == "New chat" {
		session.Title = truncateRunes(content, 40)
		_ = db.Model(&session).Update("title", session.Title).Error
	}
	_ = db.Model(&session).Update("updated_at", time.Now()).Error

	emit, okFlush := beginChatSSE(c)
	if !okFlush {
		return
	}
	runChatOrchestrator(c.Request.Context(), emit, user, &session, profile, apiKey, false, "")
}

// ConfirmChatTool approves or rejects a pending write tool, then continues the turn (SSE).
func ConfirmChatTool(c *gin.Context) {
	user, ok := currentUserOrAbort(c)
	if !ok {
		return
	}
	session, ok := loadOwnedChatSession(c, user)
	if !ok {
		return
	}
	var req chatConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	approve := true
	if req.Approve != nil {
		approve = *req.Approve
	}
	pendingID := strings.TrimSpace(session.PendingToolCallID)
	if pendingID == "" {
		c.JSON(http.StatusConflict, gin.H{"error": "No pending confirmation"})
		return
	}
	if id := strings.TrimSpace(req.ToolCallID); id != "" && id != pendingID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tool_call_id mismatch"})
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

	if !approve {
		db := config.GetDB()
		pendingName := session.PendingToolName
		rejected := chattools.Result{OK: false, Error: "User rejected this action", Summary: "rejected"}
		raw, _ := json.Marshal(rejected)
		toolMsg := models.ChatMessage{
			SessionID:  session.ID,
			Role:       "tool",
			Content:    string(raw),
			ToolCallID: pendingID,
			ToolName:   pendingName,
			Status:     "done",
		}
		_ = db.Create(&toolMsg).Error
		clearChatPending(&session)
		emit("tool_result", gin.H{
			"tool_call_id": pendingID,
			"name":         pendingName,
			"ok":           false,
			"summary":      "rejected",
			"error":        "User rejected this action",
		})
		runChatOrchestrator(c.Request.Context(), emit, user, &session, profile, apiKey, true, "")
		return
	}

	runChatOrchestrator(c.Request.Context(), emit, user, &session, profile, apiKey, true, pendingID)
}

// ListChatPlaybooks returns the preset recipe chips for an empty session.
func ListChatPlaybooks(c *gin.Context) {
	c.JSON(http.StatusOK, chattools.Playbooks)
}

func loadOwnedChatSession(c *gin.Context, user models.User) (models.ChatSession, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session id"})
		return models.ChatSession{}, false
	}
	var session models.ChatSession
	if err := config.GetDB().Where("id = ? AND created_by = ?", uint(id), user.ID).First(&session).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return models.ChatSession{}, false
	}
	return session, true
}

func toChatSessionDTO(s models.ChatSession) chatSessionDTO {
	return chatSessionDTO{
		ID:        s.ID,
		Title:     s.Title,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
		Pending:   strings.TrimSpace(s.PendingToolCallID) != "",
	}
}

func toChatMessageDTO(m models.ChatMessage) chatMessageDTO {
	dto := chatMessageDTO{
		ID:               m.ID,
		Role:             m.Role,
		Content:          m.Content,
		ReasoningContent: m.ReasoningContent,
		ToolCallsJSON:    m.ToolCallsJSON,
		ToolCallID:       m.ToolCallID,
		ToolName:         m.ToolName,
		Status:           m.Status,
		CreatedAt:        m.CreatedAt,
	}
	if m.Role == "tool" && m.Content != "" {
		var parsed chattools.Result
		if json.Unmarshal([]byte(m.Content), &parsed) == nil {
			dto.Links = parsed.Links
		}
	}
	return dto
}

type chatSSEEmit func(event string, payload any)

func beginChatSSE(c *gin.Context) (chatSSEEmit, bool) {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming unsupported"})
		return nil, false
	}
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	flusher.Flush()

	var mu sync.Mutex
	return func(event string, payload any) {
		mu.Lock()
		defer mu.Unlock()
		if c.Request.Context().Err() != nil {
			return
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return
		}
		_, _ = fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, raw)
		flusher.Flush()
	}, true
}

// runChatOrchestrator drives LLM ↔ tools until done, confirm pause, or error.
// If resumeConfirmToolCallID is set, execute that pending write tool first then continue.
func runChatOrchestrator(
	ctx context.Context,
	emit chatSSEEmit,
	user models.User,
	session *models.ChatSession,
	profile models.AIProfile,
	apiKey string,
	afterConfirm bool,
	resumeConfirmToolCallID string,
) {
	db := config.GetDB()
	timeout := time.Duration(profile.TimeoutSec) * time.Second
	if timeout < 60*time.Second {
		timeout = 60 * time.Second
	}
	if timeout > 180*time.Second {
		timeout = 180 * time.Second
	}

	ec := chattools.ExecContext{
		Ctx:       ctx,
		User:      user,
		SessionID: session.ID,
		Emit:      emit,
	}

	if resumeConfirmToolCallID != "" && strings.TrimSpace(session.PendingToolCallID) == resumeConfirmToolCallID {
		name := session.PendingToolName
		args := session.PendingToolArgs
		emit("tool_start", gin.H{"tool_call_id": resumeConfirmToolCallID, "name": name, "arguments": args})
		toolEC := ec
		toolEC.ToolCallID = resumeConfirmToolCallID
		result := chattools.Execute(toolEC, name, args)
		persistToolMessage(session.ID, resumeConfirmToolCallID, name, result)
		clearChatPending(session)
		emitToolResult(emit, resumeConfirmToolCallID, name, result)
	}

	tools := chattools.DefinitionsForLLM()
	useStream := true
	forcedCloneDone := false
	for range config.ChatMaxToolRounds() {
		if ctx.Err() != nil {
			emit("error", gin.H{"error": "cancelled"})
			return
		}
		messages, err := buildLLMMessages(ctx, session, profile, apiKey, timeout)
		if err != nil {
			emit("error", gin.H{"error": err.Error()})
			return
		}

		emit("thinking", gin.H{"phase": "plan", "message": "planning"})

		llmCtx, cancel := context.WithTimeout(ctx, timeout)
		llmCtx = utils.WithAIProfileID(llmCtx, profile.ID)
		var result utils.ChatLLMResult
		contentEmitted := false

		if useStream {
			var deltaEmitted bool
			result, err = utils.StreamChatCompletionsWithToolsFormat(llmCtx, profile.APIFormat, profile.BaseURL, apiKey, profile.Model, timeout, messages, tools, func(chunk string) {
				deltaEmitted = true
				contentEmitted = true
				emit("token", gin.H{"content": chunk})
			})
			if err != nil {
				if deltaEmitted {
					// Partial output already reached the user; never duplicate it.
					cancel()
					emit("error", gin.H{"error": err.Error()})
					return
				}
				// Stream unsupported / failed before any output → fall back once.
				log.Printf("chat stream failed, falling back to non-streaming: %v", err)
				useStream = false
				result, err = utils.ChatCompletionsWithToolsFormat(llmCtx, profile.APIFormat, profile.BaseURL, apiKey, profile.Model, timeout, messages, tools)
			}
		} else {
			result, err = utils.ChatCompletionsWithToolsFormat(llmCtx, profile.APIFormat, profile.BaseURL, apiKey, profile.Model, timeout, messages, tools)
		}
		cancel()

		if err != nil {
			// If we already ran clone_page this turn, finish with a tool-based summary
			// instead of stranding the operator on a provider follow-up error.
			if forcedCloneDone {
				lastUser := lastChatUserContent(session.ID)
				content := chatPostCloneSummary(session.ID, lastUser)
				asst := models.ChatMessage{
					SessionID: session.ID,
					Role:      "assistant",
					Content:   content,
					Status:    "done",
				}
				_ = db.Create(&asst).Error
				if content != "" && !contentEmitted {
					emit("token", gin.H{"content": content})
				}
				emit("done", gin.H{"message_id": asst.ID, "session": toChatSessionDTO(*session)})
				return
			}
			emit("error", gin.H{"error": err.Error()})
			return
		}

		if !contentEmitted && result.Content != "" {
			emit("token", gin.H{"content": result.Content})
			contentEmitted = true
		}

		toolCallsJSON := ""
		if len(result.ToolCalls) > 0 {
			b, _ := json.Marshal(result.ToolCalls)
			toolCallsJSON = string(b)
		}

		// If the model skipped tools / safety-refused but the operator clearly asked
		// to mirror a URL, force clone_page so Chat stays on the platform pipeline.
		if len(result.ToolCalls) == 0 {
			lastUser := lastChatUserContent(session.ID)
			if !forcedCloneDone {
				if urlStr, pageName, forceOK := chatForcedCloneArgs(lastUser); forceOK {
					forcedCloneDone = true
					brief := chatForcedCloneBrief(lastUser)
					argsMap := map[string]any{"url": urlStr}
					if pageName != "" {
						argsMap["name"] = pageName
					}
					argsJSON, _ := json.Marshal(argsMap)
					tcID := fmt.Sprintf("forced_clone_%d", time.Now().UnixNano())
					synthCalls := []utils.ChatToolCall{{
						ID:   tcID,
						Type: "function",
						Function: utils.ChatToolCallFunction{
							Name:      "clone_page",
							Arguments: string(argsJSON),
						},
					}}
					callsRaw, _ := json.Marshal(synthCalls)
					asst := models.ChatMessage{
						SessionID:     session.ID,
						Role:          "assistant",
						Content:       brief,
						ToolCallsJSON: string(callsRaw),
						Status:        "done",
					}
					_ = db.Create(&asst).Error
					_ = db.Model(session).Update("updated_at", time.Now()).Error
					emit("token", gin.H{"content": brief})
					emit("thinking", gin.H{"phase": "clone", "message": "cloning"})

					emit("tool_start", gin.H{"tool_call_id": tcID, "name": "clone_page", "arguments": string(argsJSON)})
					toolEC := ec
					toolEC.ToolCallID = tcID
					execResult := chattools.Execute(toolEC, "clone_page", string(argsJSON))
					persistToolMessage(session.ID, tcID, "clone_page", execResult)
					emitToolResult(emit, tcID, "clone_page", execResult)
					// Continue so the model can summarize the real tool_result.
					continue
				}
			}

			// After a forced clone, prefer a tool-based summary over a leftover refusal.
			content := result.Content
			if forcedCloneDone && (chatLooksLikeSafetyRefusal(content) || strings.TrimSpace(content) == "") {
				content = chatPostCloneSummary(session.ID, lastUser)
				if content != "" {
					emit("token", gin.H{"content": content})
				}
			}
			asst := models.ChatMessage{
				SessionID:        session.ID,
				Role:             "assistant",
				Content:          content,
				ReasoningContent: result.ReasoningContent,
				Status:           "done",
			}
			_ = db.Create(&asst).Error
			_ = db.Model(session).Update("updated_at", time.Now()).Error
			emit("done", gin.H{"message_id": asst.ID, "session": toChatSessionDTO(*session)})
			return
		}

		asst := models.ChatMessage{
			SessionID:        session.ID,
			Role:             "assistant",
			Content:          result.Content,
			ReasoningContent: result.ReasoningContent,
			ToolCallsJSON:    toolCallsJSON,
			Status:           "done",
		}
		_ = db.Create(&asst).Error
		_ = db.Model(session).Update("updated_at", time.Now()).Error

		emit("thinking", gin.H{"phase": "tools", "message": "tools"})

		for _, tc := range result.ToolCalls {
			name := strings.TrimSpace(tc.Function.Name)
			args := tc.Function.Arguments
			if chattools.RequiresConfirm(name, chattools.DefaultConfirmPolicy) {
				session.PendingToolCallID = tc.ID
				session.PendingToolName = name
				session.PendingToolArgs = args
				_ = db.Model(session).Updates(map[string]any{
					"pending_tool_call_id": tc.ID,
					"pending_tool_name":    name,
					"pending_tool_args":    args,
					"updated_at":           time.Now(),
				}).Error
				_ = db.Model(&asst).Update("status", "awaiting_confirm").Error
				risk := chattools.RiskRead
				if tool, ok := chattools.Get(name); ok {
					risk = tool.Meta().Risk
				}
				msg := ""
				if tool, ok := chattools.Get(name); ok {
					msg = tool.Meta().ConfirmMessage
				}
				emit("confirm_required", gin.H{
					"tool_call_id": tc.ID,
					"name":         name,
					"arguments":    json.RawMessage(safeJSONObject(args)),
					"message":      msg,
					"risk":         risk,
					"session":      toChatSessionDTO(*session),
				})
				emit("done", gin.H{"awaiting_confirm": true, "session": toChatSessionDTO(*session)})
				return
			}

			if name == "clone_page" {
				forcedCloneDone = true
			}
			emit("tool_start", gin.H{"tool_call_id": tc.ID, "name": name, "arguments": args})
			toolEC := ec
			toolEC.ToolCallID = tc.ID
			execResult := chattools.Execute(toolEC, name, args)
			persistToolMessage(session.ID, tc.ID, name, execResult)
			emitToolResult(emit, tc.ID, name, execResult)
		}
	}

	emit("error", gin.H{"error": "tool round limit reached"})
	emit("done", gin.H{"error": "tool round limit reached"})
	_ = afterConfirm
}

func persistToolMessage(sessionID uint, toolCallID, name string, result chattools.Result) {
	raw, _ := json.Marshal(result)
	status := "done"
	if !result.OK {
		status = "error"
	}
	_ = config.GetDB().Create(&models.ChatMessage{
		SessionID:  sessionID,
		Role:       "tool",
		Content:    string(raw),
		ToolCallID: toolCallID,
		ToolName:   name,
		Status:     status,
	}).Error
}

func emitToolResult(emit chatSSEEmit, toolCallID, name string, result chattools.Result) {
	payload := gin.H{
		"tool_call_id": toolCallID,
		"name":         name,
		"ok":           result.OK,
		"summary":      result.Summary,
		"result":       result.Result,
		"links":        result.Links,
	}
	if result.Error != "" {
		payload["error"] = result.Error
	}
	if len(result.Ephemeral) > 0 {
		payload["ephemeral"] = result.Ephemeral
	}
	emit("tool_result", payload)
}

func lastChatUserContent(sessionID uint) string {
	var m models.ChatMessage
	err := config.GetDB().
		Where("session_id = ? AND role = ?", sessionID, "user").
		Order("id DESC").
		First(&m).Error
	if err != nil {
		return ""
	}
	return m.Content
}

func chatForcedCloneBrief(userText string) string {
	if chatLooksLikeCJK(userText) {
		return "好的，正在通过平台 clone_page 镜像目标页面到页面库。"
	}
	return "Got it — mirroring the target page into the page library via clone_page."
}

func chatLooksLikeCJK(text string) bool {
	for _, r := range text {
		if r >= 0x4e00 && r <= 0x9fff {
			return true
		}
	}
	return false
}

func chatPostCloneSummary(sessionID uint, userText string) string {
	var m models.ChatMessage
	err := config.GetDB().
		Where("session_id = ? AND role = ? AND tool_name = ?", sessionID, "tool", "clone_page").
		Order("id DESC").
		First(&m).Error
	cjk := chatLooksLikeCJK(userText)
	if err != nil {
		if cjk {
			return "已尝试通过平台克隆页面，请查看工具结果。"
		}
		return "Attempted to mirror the page via the platform — see the tool result above."
	}
	var parsed chattools.Result
	if json.Unmarshal([]byte(m.Content), &parsed) != nil {
		if cjk {
			return "克隆流程已结束，请查看上方工具结果。"
		}
		return "Clone finished — see the tool result above."
	}
	if parsed.OK {
		if parsed.Summary != "" {
			return parsed.Summary
		}
		if cjk {
			return "页面已通过平台镜像并写入页面库。"
		}
		return "Page mirrored into the page library via the platform."
	}
	if parsed.Error != "" {
		if cjk {
			return "克隆失败：" + parsed.Error
		}
		return "Clone failed: " + parsed.Error
	}
	if cjk {
		return "克隆未成功，请查看工具结果。"
	}
	return "Clone did not succeed — see the tool result."
}

func buildLLMMessages(ctx context.Context, session *models.ChatSession, profile models.AIProfile, apiKey string, timeout time.Duration) ([]utils.ChatLLMMessage, error) {
	var rows []models.ChatMessage
	if err := config.GetDB().Where("session_id = ?", session.ID).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to load history")
	}
	maxHistory := config.ChatMaxHistoryMessages()
	start := 0
	if maxHistory > 0 && len(rows) > maxHistory {
		start = len(rows) - maxHistory
		// Never begin on a tool message: its assistant tool_calls would be dropped.
		for start < len(rows) && rows[start].Role == "tool" {
			start++
		}
	}
	if start == 0 {
		return assembleLLMMessages(rows, ""), nil
	}

	summary := strings.TrimSpace(session.Summary)
	upTo := rows[start-1].ID
	if summary == "" || session.SummaryUpToMessageID < upTo {
		next := ""
		generated, err := summarizeChatWithAI(ctx, profile, apiKey, timeout, rows[:start], summary)
		if err == nil && strings.TrimSpace(generated) != "" {
			next = strings.TrimSpace(generated)
		} else {
			// Fall back to a truncated recap so the chat keeps working when the
			// summarization call fails.
			merged := summary
			if extra := summarizeChatRows(rows[:start]); extra != "" {
				if merged != "" {
					merged += "\n"
				}
				merged += extra
			}
			next = strings.TrimSpace(merged)
		}
		if next != "" {
			summary = next
			session.Summary = next
			session.SummaryUpToMessageID = upTo
			_ = config.GetDB().Model(session).Updates(map[string]any{
				"summary":                  next,
				"summary_up_to_message_id": upTo,
			}).Error
		}
	}
	return assembleLLMMessages(rows[start:], summary), nil
}

// assembleLLMMessages prepends the tool system prompt and an optional recap,
// then converts the kept turns into an LLM conversation.
func assembleLLMMessages(rows []models.ChatMessage, summary string) []utils.ChatLLMMessage {
	out := make([]utils.ChatLLMMessage, 0, len(rows)+2)
	out = append(out, utils.ChatLLMMessage{Role: "system", Content: chattools.SystemPrompt()})
	if strings.TrimSpace(summary) != "" {
		out = append(out, utils.ChatLLMMessage{
			Role:    "system",
			Content: "Summary of earlier conversation:\n" + summary,
		})
	}
	for _, m := range rows {
		switch m.Role {
		case "user":
			out = append(out, utils.ChatLLMMessage{Role: "user", Content: m.Content})
		case "assistant":
			msg := utils.ChatLLMMessage{Role: "assistant", Content: m.Content}
			if strings.TrimSpace(m.ReasoningContent) != "" {
				msg.ReasoningContent = m.ReasoningContent
			}
			if strings.TrimSpace(m.ToolCallsJSON) != "" {
				var calls []utils.ChatToolCall
				if json.Unmarshal([]byte(m.ToolCallsJSON), &calls) == nil {
					msg.ToolCalls = calls
				}
			}
			out = append(out, msg)
		case "tool":
			out = append(out, utils.ChatLLMMessage{
				Role:       "tool",
				Content:    m.Content,
				ToolCallID: m.ToolCallID,
				Name:       m.ToolName,
			})
		}
	}
	return out
}

// buildLLMMessagesFromRows is the pure fallback used by tests: it compresses
// older turns with a truncated recap, without an LLM call.
func buildLLMMessagesFromRows(rows []models.ChatMessage, maxHistory int) []utils.ChatLLMMessage {
	start := 0
	if maxHistory > 0 && len(rows) > maxHistory {
		start = len(rows) - maxHistory
		for start < len(rows) && rows[start].Role == "tool" {
			start++
		}
	}
	summary := ""
	if start > 0 {
		summary = summarizeChatRows(rows[:start])
	}
	return assembleLLMMessages(rows[start:], summary)
}

func summarizeChatRows(rows []models.ChatMessage) string {
	const perLine = 160
	const maxTotal = 2000
	var b strings.Builder
	for _, m := range rows {
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		text := strings.TrimSpace(m.Content)
		if text == "" {
			continue
		}
		if b.Len() >= maxTotal {
			break
		}
		b.WriteString(m.Role)
		b.WriteString(": ")
		b.WriteString(truncateRunes(text, perLine))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func clearChatPending(session *models.ChatSession) {
	session.PendingToolCallID = ""
	session.PendingToolName = ""
	session.PendingToolArgs = ""
	_ = config.GetDB().Model(session).Updates(map[string]any{
		"pending_tool_call_id": "",
		"pending_tool_name":    "",
		"pending_tool_args":    "",
		"updated_at":           time.Now(),
	}).Error
}

func safeJSONObject(args string) []byte {
	args = strings.TrimSpace(args)
	if args == "" {
		return []byte("{}")
	}
	if json.Valid([]byte(args)) {
		return []byte(args)
	}
	b, _ := json.Marshal(args)
	return b
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	return string(runes[:n]) + "…"
}

func checkChatRateLimit(userID uint) error {
	chatRateMu.Lock()
	defer chatRateMu.Unlock()
	now := time.Now()
	minInterval := time.Duration(config.ChatRateMinIntervalMS()) * time.Millisecond
	if last, ok := chatLastCall[userID]; ok && now.Sub(last) < minInterval {
		return fmt.Errorf("sending too fast, please wait a moment")
	}
	day := now.Format("2006-01-02")
	entry := chatDayCount[userID]
	if entry.day != day {
		entry.day = day
		entry.count = 0
	}
	dailyLimit := config.ChatDailyLimit()
	if entry.count >= dailyLimit {
		return fmt.Errorf("daily chat limit reached (%d)", dailyLimit)
	}
	entry.count++
	chatDayCount[userID] = entry
	chatLastCall[userID] = now
	return nil
}
