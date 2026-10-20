package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"spearhook/config"
	"spearhook/models"
	"spearhook/utils"
)

func setupMailCampaignTest(t *testing.T) models.User {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "mail.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := models.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	config.DB = db
	admin := models.User{Username: "admin", Password: "x", Role: models.UserRoleAdmin}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatal(err)
	}
	return admin
}

func TestResolveCampaignSchedule(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	sendNow, when := resolveCampaignSchedule(nil, now)
	if !sendNow || when != nil {
		t.Fatalf("empty schedule should send now")
	}
	soon := now.Add(5 * time.Second)
	sendNow, when = resolveCampaignSchedule(&soon, now)
	if !sendNow || when != nil {
		t.Fatalf("near-future should send now")
	}
	later := now.Add(time.Hour)
	sendNow, when = resolveCampaignSchedule(&later, now)
	if sendNow || when == nil || !when.Equal(later) {
		t.Fatalf("future schedule not kept: sendNow=%v when=%v", sendNow, when)
	}
}

func TestCreateMailCampaignScheduledDoesNotSend(t *testing.T) {
	admin := setupMailCampaignTest(t)
	smtp := models.SmtpService{
		Name: "smtp", Host: "127.0.0.1", Port: 2525, FromEmail: "from@example.com", Enabled: true, CreatedBy: admin.ID,
	}
	if err := config.DB.Create(&smtp).Error; err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	sent := 0
	prev := sendCampaignMailFn
	sendCampaignMailFn = func(cfg utils.SmtpSendConfig, msg utils.SmtpMessage) error {
		mu.Lock()
		sent++
		mu.Unlock()
		return nil
	}
	t.Cleanup(func() { sendCampaignMailFn = prev })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/mail-campaigns", func(c *gin.Context) {
		c.Set("user", admin)
		CreateMailCampaign(c)
	})

	when := time.Now().UTC().Add(2 * time.Hour)
	body, _ := json.Marshal(map[string]any{
		"smtp_service_id": smtp.ID,
		"landing_url":     "https://phish.example/login",
		"subject":         "notice",
		"body":            "hello {{email}}",
		"is_html":         false,
		"recipients":      []string{"a@example.com"},
		"scheduled_at":    when.Format(time.RFC3339),
	})
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mail-campaigns", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	var created models.MailCampaign
	if err := json.Unmarshal(res.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Status != models.MailCampaignStatusScheduled {
		t.Fatalf("status=%s want scheduled", created.Status)
	}
	if created.ScheduledAt == nil {
		t.Fatal("scheduled_at missing")
	}
	mu.Lock()
	got := sent
	mu.Unlock()
	if got != 0 {
		t.Fatalf("scheduled campaign sent %d messages", got)
	}

	if err := config.DB.Model(&created).Update("scheduled_at", time.Now().UTC().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	ProcessDueMailCampaigns()
	mu.Lock()
	got = sent
	mu.Unlock()
	if got != 1 {
		t.Fatalf("due campaign sent %d want 1", got)
	}
	var after models.MailCampaign
	if err := config.DB.First(&after, created.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after.Status != models.MailCampaignStatusSent || after.SentCount != 1 {
		t.Fatalf("after deliver status=%s sent=%d", after.Status, after.SentCount)
	}
}

func TestMailTemplateCRUD(t *testing.T) {
	admin := setupMailCampaignTest(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/mail-templates", func(c *gin.Context) {
		c.Set("user", admin)
		CreateMailTemplate(c)
	})
	r.GET("/mail-templates", func(c *gin.Context) {
		c.Set("user", admin)
		GetMailTemplates(c)
	})
	r.PUT("/mail-templates/:id", func(c *gin.Context) {
		c.Set("user", admin)
		UpdateMailTemplate(c)
	})
	r.DELETE("/mail-templates/:id", func(c *gin.Context) {
		c.Set("user", admin)
		DeleteMailTemplate(c)
	})

	createBody, _ := json.Marshal(map[string]any{
		"name": "高温津贴", "subject": "津贴发放", "body": "<p>{{click_url}}</p>", "is_html": true,
	})
	createRes := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mail-templates", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(createRes, req)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", createRes.Code, createRes.Body.String())
	}
	var tpl models.MailTemplate
	if err := json.Unmarshal(createRes.Body.Bytes(), &tpl); err != nil {
		t.Fatal(err)
	}
	if tpl.ID == 0 || tpl.Name != "高温津贴" || !tpl.IsHTML {
		t.Fatalf("created %+v", tpl)
	}

	listRes := httptest.NewRecorder()
	r.ServeHTTP(listRes, httptest.NewRequest(http.MethodGet, "/mail-templates", nil))
	if listRes.Code != http.StatusOK {
		t.Fatalf("list status=%d", listRes.Code)
	}

	updBody, _ := json.Marshal(map[string]any{
		"name": "高温津贴", "subject": "更新主题", "body": "plain {{email}}", "is_html": false,
	})
	updRes := httptest.NewRecorder()
	updReq := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/mail-templates/%d", tpl.ID), bytes.NewReader(updBody))
	updReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(updRes, updReq)
	if updRes.Code != http.StatusOK {
		t.Fatalf("update status=%d body=%s", updRes.Code, updRes.Body.String())
	}

	delRes := httptest.NewRecorder()
	r.ServeHTTP(delRes, httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/mail-templates/%d", tpl.ID), nil))
	if delRes.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", delRes.Code, delRes.Body.String())
	}
}
