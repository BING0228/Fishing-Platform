package platform

import (
	"path/filepath"
	"testing"

	"spearhook/config"
	"spearhook/models"
	"spearhook/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestRecordAIUsage(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "usage.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := models.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	config.DB = db

	profile := models.AIProfile{
		ID:      "usage-profile",
		Name:    "Usage",
		BaseURL: "https://example.com/v1",
		Model:   "m",
	}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatal(err)
	}

	RecordAIUsage(profile.ID, utils.AITokenUsage{PromptTokens: 10, CompletionTokens: 5})
	RecordAIUsage(profile.ID, utils.AITokenUsage{TotalTokens: 7})

	var got models.AIProfile
	if err := db.First(&got, "id = ?", profile.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.PromptTokens != 10 || got.CompletionTokens != 5 || got.TotalTokens != 22 {
		t.Fatalf("usage = prompt:%d completion:%d total:%d", got.PromptTokens, got.CompletionTokens, got.TotalTokens)
	}
	if got.RequestCount != 2 {
		t.Fatalf("requests = %d", got.RequestCount)
	}
	if got.LastUsedAt == nil {
		t.Fatal("last_used_at was not set")
	}
}
