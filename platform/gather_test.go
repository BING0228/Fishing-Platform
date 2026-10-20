package platform

import (
	"path/filepath"
	"testing"
	"time"

	"spearhook/chatbus"
	"spearhook/config"
	"spearhook/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestFinishInfoGatherJobPublishesToChat(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "gather.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := models.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	config.DB = db

	session := models.ChatSession{Title: "t", CreatedBy: 1}
	if err := db.Create(&session).Error; err != nil {
		t.Fatal(err)
	}
	job := models.InfoGatherJob{
		Target:        "example.com",
		Status:        models.InfoGatherJobRunning,
		CreatedBy:     1,
		ChatSessionID: session.ID,
	}
	if err := db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}

	ch, cancel := chatbus.Subscribe(session.ID)
	defer cancel()

	FinishInfoGatherJob(job.ID, models.InfoGatherJobSucceeded, "", "", []models.InfoGatherFinding{
		{Kind: "email", Value: "a@example.com", SourceURL: "https://example.com"},
	}, "notes")

	select {
	case ev := <-ch:
		if ev.Name != "job_done" {
			t.Fatalf("event = %q", ev.Name)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for job_done")
	}

	var msg models.ChatMessage
	if err := db.Where("session_id = ? AND role = ?", session.ID, "system").First(&msg).Error; err != nil {
		t.Fatalf("system message not persisted: %v", err)
	}
	if msg.ToolName != "info_gather_job" {
		t.Fatalf("tool_name = %q", msg.ToolName)
	}
}
