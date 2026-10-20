package handlers

import (
	"context"
	"strings"
	"time"

	"spearhook/models"
	"spearhook/utils"
)

const chatSummarySystemPrompt = `You maintain a compact running summary of a security-operations console chat.
Preserve concrete facts the operator will need later: goals, tool names and outcomes, IDs (projects, pages, jobs, campaigns), pending confirmations, and decisions.
Drop pleasantries and repetition. Write at most ~300 words. Output only the summary.`

// summarizeChatWithAI folds older turns (and a previous summary) into one
// concise recap. The caller falls back to truncation if this fails.
func summarizeChatWithAI(ctx context.Context, profile models.AIProfile, apiKey string, timeout time.Duration, rows []models.ChatMessage, previous string) (string, error) {
	var b strings.Builder
	if strings.TrimSpace(previous) != "" {
		b.WriteString("Previous summary:\n")
		b.WriteString(truncateRunes(previous, 4000))
		b.WriteString("\n\nNew turns to fold in:\n")
	}
	for _, m := range rows {
		switch m.Role {
		case "user":
			b.WriteString("user: " + truncateRunes(m.Content, 500) + "\n")
		case "assistant":
			if strings.TrimSpace(m.Content) != "" {
				b.WriteString("assistant: " + truncateRunes(m.Content, 500) + "\n")
			}
			if strings.TrimSpace(m.ToolCallsJSON) != "" {
				b.WriteString("assistant tools: " + truncateRunes(m.ToolCallsJSON, 800) + "\n")
			}
		case "tool":
			b.WriteString("tool " + m.ToolName + ": " + truncateRunes(m.Content, 400) + "\n")
		}
	}
	transcript := b.String()
	if len(transcript) > 24000 {
		// Keep the most recent turns; the previous summary is already at the top
		// only for small histories, so re-add it when truncating.
		head := ""
		if strings.TrimSpace(previous) != "" {
			head = "Previous summary:\n" + truncateRunes(previous, 4000) + "\n\n"
		}
		transcript = head + transcript[len(transcript)-20000:]
	}

	summaryTimeout := timeout
	if summaryTimeout <= 0 || summaryTimeout > 30*time.Second {
		summaryTimeout = 30 * time.Second
	}
	callCtx, cancel := context.WithTimeout(ctx, summaryTimeout)
	defer cancel()
	callCtx = utils.WithAIProfileID(callCtx, profile.ID)

	result, err := utils.ChatCompletionsWithToolsFormat(
		callCtx,
		profile.APIFormat,
		profile.BaseURL,
		apiKey,
		profile.Model,
		summaryTimeout,
		[]utils.ChatLLMMessage{
			{Role: "system", Content: chatSummarySystemPrompt},
			{Role: "user", Content: transcript},
		},
		nil,
	)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Content), nil
}
