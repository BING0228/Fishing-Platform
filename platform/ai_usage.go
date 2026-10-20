package platform

import (
	"strings"
	"time"

	"spearhook/config"
	"spearhook/models"
	"spearhook/utils"

	"gorm.io/gorm"
)

func init() {
	utils.SetAIUsageRecorder(RecordAIUsage)
}

// RecordAIUsage adds one provider call's tokens to the matching AI profile.
// Missing profiles and uninitialized DBs are ignored; usage must never break
// the AI request path.
func RecordAIUsage(profileID string, usage utils.AITokenUsage) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return
	}
	if usage.TotalTokens <= 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	if usage.TotalTokens <= 0 {
		return
	}
	prompt := usage.PromptTokens
	if prompt < 0 {
		prompt = 0
	}
	completion := usage.CompletionTokens
	if completion < 0 {
		completion = 0
	}
	total := usage.TotalTokens
	if total < prompt+completion {
		total = prompt + completion
	}
	db := config.GetDB()
	if db == nil {
		return
	}
	now := time.Now()
	_ = db.Model(&models.AIProfile{}).Where("id = ?", profileID).Updates(map[string]any{
		"prompt_tokens":     gorm.Expr("prompt_tokens + ?", prompt),
		"completion_tokens": gorm.Expr("completion_tokens + ?", completion),
		"total_tokens":      gorm.Expr("total_tokens + ?", total),
		"request_count":     gorm.Expr("request_count + 1"),
		"last_used_at":      now,
	}).Error
}
