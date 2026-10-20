package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"spearhook/config"
	"spearhook/models"
	"spearhook/utils"
)

var sendCampaignMailFn = utils.SendSMTPMail

var dueMailMu sync.Mutex

type mailCampaignCreatePayload struct {
	SmtpServiceID uint        `json:"smtp_service_id"`
	ProjectID     *uint       `json:"project_id"`
	TemplateID    *uint       `json:"template_id"`
	LandingURL    string      `json:"landing_url"`
	Subject       string      `json:"subject"`
	Body          string      `json:"body"`
	IsHTML        bool        `json:"is_html"`
	TrackOpens    *bool       `json:"track_opens"`
	TrackClicks   *bool       `json:"track_clicks"`
	Recipients    interface{} `json:"recipients"`
	ScheduledAt   *time.Time  `json:"scheduled_at"`
}

func resolveMailPublicBase(c *gin.Context) string {
	return requestPublicBaseURL(c)
}

func parseRecipientEmails(value interface{}) ([]string, error) {
	var raw []string
	switch v := value.(type) {
	case string:
		raw = utils.NormalizeEmailList([]string{v})
	case []interface{}:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, fmt.Sprint(item))
		}
		raw = utils.NormalizeEmailList(parts)
	case []string:
		raw = utils.NormalizeEmailList(v)
	default:
		return nil, errors.New("recipients must be a list of emails")
	}
	if len(raw) == 0 {
		return nil, errors.New("recipients is required")
	}
	for _, addr := range raw {
		if _, err := mail.ParseAddress(addr); err != nil {
			return nil, errors.New("invalid email address: " + addr)
		}
	}
	return raw, nil
}

func resolveCampaignLandingURL(db *gorm.DB, projectID *uint, landingURL string) (string, error) {
	landingURL = strings.TrimSpace(landingURL)
	if landingURL != "" {
		if utils.ExtractURLHost(landingURL) == "" {
			return "", errors.New("landing_url is invalid")
		}
		return landingURL, nil
	}
	if projectID == nil || *projectID == 0 {
		return "", errors.New("landing_url or project_id is required")
	}
	var project models.Project
	if err := db.First(&project, *projectID).Error; err != nil {
		return "", errors.New("project not found")
	}
	if strings.TrimSpace(project.LoginURL) == "" {
		return "", errors.New("project has no login_url; provide landing_url")
	}
	if utils.ExtractURLHost(project.LoginURL) == "" {
		return "", errors.New("project login_url is invalid")
	}
	return strings.TrimSpace(project.LoginURL), nil
}

func mailAllowedHosts(landingURL string, extra []string) []string {
	hosts := append([]string{}, extra...)
	if host := utils.ExtractURLHost(landingURL); host != "" {
		hosts = append(hosts, host)
	}
	return hosts
}

// CreateMailCampaign creates a campaign and sends personalized messages.
func CreateMailCampaign(c *gin.Context) {
	if !EffectiveMailTrackingEnabled() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Mail tracking is disabled"})
		return
	}

	var payload mailCampaignCreatePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}
	if payload.SmtpServiceID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "smtp_service_id is required"})
		return
	}
	subject := strings.TrimSpace(payload.Subject)
	body := payload.Body
	if subject == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "subject is required"})
		return
	}
	if strings.TrimSpace(body) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body is required"})
		return
	}
	recipients, err := parseRecipientEmails(payload.Recipients)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, ok := currentUserOrAbort(c)
	if !ok {
		return
	}
	db := config.GetDB()
	var smtpService models.SmtpService
	if err := scopeByOwner(db, user).First(&smtpService, payload.SmtpServiceID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "SMTP service not found"})
		return
	}
	if !smtpService.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "SMTP service is disabled"})
		return
	}

	if payload.ProjectID != nil && *payload.ProjectID != 0 {
		var project models.Project
		if err := scopeByOwner(db, user).First(&project, *payload.ProjectID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Project not found"})
			return
		}
	}

	var templateID *uint
	if payload.TemplateID != nil && *payload.TemplateID != 0 {
		var tpl models.MailTemplate
		if err := scopeByOwner(db, user).First(&tpl, *payload.TemplateID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Mail template not found"})
			return
		}
		id := tpl.ID
		templateID = &id
	}

	landingURL, err := resolveCampaignLandingURL(db, payload.ProjectID, payload.LandingURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trackClicks := true
	if payload.TrackClicks != nil {
		trackClicks = *payload.TrackClicks
	}
	trackOpens := payload.IsHTML
	if payload.TrackOpens != nil {
		trackOpens = *payload.TrackOpens
	}
	if !payload.IsHTML {
		trackOpens = false
	}

	campaign := models.MailCampaign{
		SmtpServiceID:  payload.SmtpServiceID,
		ProjectID:      payload.ProjectID,
		TemplateID:     templateID,
		Subject:        subject,
		BodyTemplate:   body,
		IsHTML:         payload.IsHTML,
		TrackOpens:     trackOpens,
		TrackClicks:    trackClicks,
		LandingURL:     landingURL,
		PublicBaseURL:  resolveMailPublicBase(c),
		Status:         models.MailCampaignStatusSending,
		RecipientCount: len(recipients),
		CreatedBy:      user.ID,
	}
	sendNow, scheduledAt := resolveCampaignSchedule(payload.ScheduledAt, time.Now())
	campaign.ScheduledAt = scheduledAt
	if !sendNow {
		campaign.Status = models.MailCampaignStatusScheduled
	}
	if err := db.Create(&campaign).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create campaign"})
		return
	}

	secret := EffectiveMailTrackingSecret()
	campaign.ClickPath = config.DeriveMailTrackingAPIPath(secret, fmt.Sprintf("click|%d", campaign.ID))
	campaign.OpenPath = config.DeriveMailTrackingAPIPath(secret, fmt.Sprintf("open|%d", campaign.ID))
	if campaign.ClickPath == campaign.OpenPath {
		campaign.OpenPath = config.DeriveMailTrackingAPIPath(secret, fmt.Sprintf("open|%d|x", campaign.ID))
	}
	_ = db.Model(&campaign).Updates(map[string]interface{}{
		"click_path": campaign.ClickPath,
		"open_path":  campaign.OpenPath,
	}).Error

	for _, email := range recipients {
		_ = db.Create(&models.MailRecipient{
			CampaignID: campaign.ID,
			Email:      strings.ToLower(email),
		}).Error
	}

	if sendNow {
		deliverMailCampaign(campaign.ID)
	}

	var out models.MailCampaign
	_ = db.First(&out, campaign.ID)
	c.JSON(http.StatusCreated, out)
}

func GetMailCampaigns(c *gin.Context) {
	user, ok := currentUserOrAbort(c)
	if !ok {
		return
	}
	db := config.GetDB()
	var items []models.MailCampaign
	if err := scopeByOwner(db, user).Order("created_at DESC").Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list campaigns"})
		return
	}
	c.JSON(http.StatusOK, items)
}

func GetMailCampaign(c *gin.Context) {
	user, ok := currentUserOrAbort(c)
	if !ok {
		return
	}
	db := config.GetDB()
	var item models.MailCampaign
	if err := scopeByOwner(db, user).First(&item, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Campaign not found"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// DeleteMailCampaign deletes a campaign and its recipients/events (owner or admin).
func DeleteMailCampaign(c *gin.Context) {
	user, ok := currentUserOrAbort(c)
	if !ok {
		return
	}
	db := config.GetDB()
	var campaign models.MailCampaign
	if err := scopeByOwner(db, user).First(&campaign, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Campaign not found"})
		return
	}
	if campaign.Status == models.MailCampaignStatusSending {
		c.JSON(http.StatusConflict, gin.H{"error": "Cannot delete a campaign that is still sending"})
		return
	}

	tx := db.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete campaign"})
		return
	}
	if err := tx.Where("campaign_id = ?", campaign.ID).Delete(&models.MailEvent{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete campaign events"})
		return
	}
	if err := tx.Where("campaign_id = ?", campaign.ID).Delete(&models.MailRecipient{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete campaign recipients"})
		return
	}
	if err := tx.Delete(&campaign).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete campaign"})
		return
	}
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit delete"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func GetMailCampaignRecipients(c *gin.Context) {
	user, ok := currentUserOrAbort(c)
	if !ok {
		return
	}
	db := config.GetDB()
	var campaign models.MailCampaign
	if err := scopeByOwner(db, user).First(&campaign, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Campaign not found"})
		return
	}
	var items []models.MailRecipient
	if err := db.Where("campaign_id = ?", campaign.ID).Order("id ASC").Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list recipients"})
		return
	}
	c.JSON(http.StatusOK, items)
}

func GetMailCampaignEvents(c *gin.Context) {
	user, ok := currentUserOrAbort(c)
	if !ok {
		return
	}
	db := config.GetDB()
	var campaign models.MailCampaign
	if err := scopeByOwner(db, user).First(&campaign, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Campaign not found"})
		return
	}
	limit := 200
	if raw := c.Query("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}
	type mailEventRow struct {
		ID          uint      `json:"id"`
		CampaignID  uint      `json:"campaign_id"`
		RecipientID uint      `json:"recipient_id"`
		Kind        string    `json:"kind"`
		IP          string    `json:"ip"`
		UserAgent   string    `json:"user_agent"`
		TargetURL   string    `json:"target_url"`
		CreatedAt   time.Time `json:"created_at"`
		Email       string    `json:"email"`
	}
	var items []mailEventRow
	if err := db.Table("mail_events AS e").
		Select("e.id, e.campaign_id, e.recipient_id, e.kind, e.ip, e.user_agent, e.target_url, e.created_at, COALESCE(r.email, '') AS email").
		Joins("LEFT JOIN mail_recipients AS r ON r.id = e.recipient_id").
		Where("e.campaign_id = ?", campaign.ID).
		Order("e.id DESC").
		Limit(limit).
		Scan(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list events"})
		return
	}
	c.JSON(http.StatusOK, items)
}

func resolveCampaignSchedule(raw *time.Time, now time.Time) (sendNow bool, scheduledAt *time.Time) {
	if raw == nil {
		return true, nil
	}
	when := raw.UTC()
	if !when.After(now.Add(15 * time.Second)) {
		return true, nil
	}
	return false, &when
}

func deliverMailCampaign(campaignID uint) {
	db := config.GetDB()
	var campaign models.MailCampaign
	if err := db.First(&campaign, campaignID).Error; err != nil {
		return
	}
	var smtpService models.SmtpService
	if err := db.First(&smtpService, campaign.SmtpServiceID).Error; err != nil || !smtpService.Enabled {
		_ = db.Model(&campaign).Update("status", models.MailCampaignStatusFailed).Error
		return
	}

	publicBase := strings.TrimRight(strings.TrimSpace(campaign.PublicBaseURL), "/")
	if publicBase == "" {
		publicBase = EffectivePublicBaseURL()
	}
	secret := EffectiveMailTrackingSecret()
	smtpCfg := smtpConfigFromModel(smtpService)

	var recipients []models.MailRecipient
	if err := db.Where("campaign_id = ?", campaign.ID).Order("id ASC").Find(&recipients).Error; err != nil {
		return
	}

	sent := campaign.SentCount
	failed := campaign.FailedCount
	for i := range recipients {
		recipient := &recipients[i]
		if recipient.SentAt != nil {
			continue
		}
		sig := utils.MailTrackingSign(secret, campaign.ID, recipient.Email)
		v := utils.BuildMailTrackingV(secret, campaign.ID, recipient.Email)
		landingWithV := utils.AppendMailTrackingV(campaign.LandingURL, v)
		vars := map[string]string{
			"email":       recipient.Email,
			"sig":         sig,
			"v":           v,
			"landing_url": landingWithV,
			"click_url":   landingWithV,
			"open_pixel":  "",
		}
		if campaign.TrackClicks {
			vars["click_url"] = utils.BuildMailClickURL(
				publicBase,
				campaign.ClickPath,
				secret,
				campaign.ID,
				recipient.Email,
				campaign.LandingURL,
			)
		}
		if campaign.TrackOpens {
			vars["open_pixel"] = utils.BuildMailOpenPixelURL(
				publicBase,
				campaign.OpenPath,
				secret,
				campaign.ID,
				recipient.Email,
			)
		}
		rendered := utils.RenderMailTemplate(campaign.BodyTemplate, vars, campaign.IsHTML, campaign.TrackOpens)
		sendErr := sendCampaignMailFn(smtpCfg, utils.SmtpMessage{
			To:      []string{recipient.Email},
			Subject: campaign.Subject,
			Body:    rendered,
			HTML:    campaign.IsHTML,
		})
		now := time.Now()
		if sendErr != nil {
			failed++
			_ = db.Model(recipient).Updates(map[string]interface{}{
				"last_error": sendErr.Error(),
			}).Error
			_ = db.Create(&models.MailEvent{
				CampaignID:  campaign.ID,
				RecipientID: recipient.ID,
				Kind:        models.MailEventError,
				TargetURL:   sendErr.Error(),
			}).Error
			continue
		}
		sent++
		_ = db.Model(recipient).Updates(map[string]interface{}{
			"sent_at":    &now,
			"last_error": "",
		}).Error
		_ = db.Create(&models.MailEvent{
			CampaignID:  campaign.ID,
			RecipientID: recipient.ID,
			Kind:        models.MailEventSent,
		}).Error
	}

	status := models.MailCampaignStatusSent
	if sent == 0 {
		status = models.MailCampaignStatusFailed
	} else if failed > 0 {
		status = models.MailCampaignStatusPartial
	}
	_ = db.Model(&campaign).Updates(map[string]interface{}{
		"sent_count":   sent,
		"failed_count": failed,
		"status":       status,
	}).Error
}

// StartMailCampaignScheduler sends due scheduled campaigns in the background.
func StartMailCampaignScheduler(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	go func() {
		ProcessDueMailCampaigns()
		tick := time.NewTicker(15 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				ProcessDueMailCampaigns()
			}
		}
	}()
}

// ProcessDueMailCampaigns claims scheduled campaigns whose time has arrived.
func ProcessDueMailCampaigns() {
	if !dueMailMu.TryLock() {
		return
	}
	defer dueMailMu.Unlock()

	db := config.GetDB()
	if db == nil {
		return
	}
	now := time.Now().UTC()
	var due []models.MailCampaign
	if err := db.Where("status = ? AND scheduled_at IS NOT NULL AND scheduled_at <= ?", models.MailCampaignStatusScheduled, now).
		Order("scheduled_at ASC").
		Limit(20).
		Find(&due).Error; err != nil {
		log.Printf("mail scheduler list: %v", err)
		return
	}
	for i := range due {
		res := db.Model(&models.MailCampaign{}).
			Where("id = ? AND status = ?", due[i].ID, models.MailCampaignStatusScheduled).
			Update("status", models.MailCampaignStatusSending)
		if res.Error != nil || res.RowsAffected != 1 {
			continue
		}
		deliverMailCampaign(due[i].ID)
	}
}
