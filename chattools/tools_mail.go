package chattools

import (
	"fmt"
	"net/mail"
	"strings"
	"time"

	"spearhook/config"
	"spearhook/models"
	"spearhook/platform"
	"spearhook/utils"
)

func init() {
	Register(newTool(
		"create_mail_draft",
		"Create a mail campaign draft (subject + body + recipients). This does NOT send anything; sending requires send_mail_campaign.",
		RiskWrite,
		schema(map[string]any{
			"smtp_id":      map[string]any{"type": "integer", "description": "SMTP service id (from list_smtp)"},
			"project_id":   map[string]any{"type": "integer", "description": "Optional project id used for the landing URL"},
			"template_id":  map[string]any{"type": "integer", "description": "Optional mail template id"},
			"subject":      map[string]any{"type": "string"},
			"body":         map[string]any{"type": "string", "description": "Body template; supports {{email}}, {{landing_url}}, {{click_url}}, {{open_pixel}}"},
			"is_html":      map[string]any{"type": "boolean", "description": "Default false"},
			"track_opens":  map[string]any{"type": "boolean"},
			"track_clicks": map[string]any{"type": "boolean"},
			"landing_url":  map[string]any{"type": "string", "description": "Optional; defaults to the project login_url"},
			"recipients":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Email addresses"},
		}, []string{"smtp_id", "subject", "body", "recipients"}),
		"",
		toolCreateMailDraft,
	))
	Register(newTool(
		"send_mail_campaign",
		"Send an existing draft campaign to its recipients. ALWAYS requires operator confirmation.",
		RiskDestructive,
		schema(map[string]any{
			"campaign_id": map[string]any{"type": "integer", "description": "Campaign id returned by create_mail_draft"},
		}, []string{"campaign_id"}),
		"This will send email to every recipient of the campaign. Approve?",
		toolSendMailCampaign,
	))
	Register(newTool(
		"preview_mail",
		"Render a mail draft preview (subject/body/recipients) without sending. Accepts campaign_id, template_id, or inline subject+body.",
		RiskRead,
		schema(map[string]any{
			"campaign_id":  map[string]any{"type": "integer", "description": "Optional campaign id"},
			"template_id":  map[string]any{"type": "integer", "description": "Optional mail template id"},
			"subject":      map[string]any{"type": "string"},
			"body":         map[string]any{"type": "string"},
			"is_html":      map[string]any{"type": "boolean"},
			"track_opens":  map[string]any{"type": "boolean"},
			"track_clicks": map[string]any{"type": "boolean"},
			"landing_url":  map[string]any{"type": "string"},
			"recipient":    map[string]any{"type": "string", "description": "Optional sample recipient email"},
		}, nil),
		"",
		toolPreviewMail,
	))
	Register(newTool(
		"list_mail_campaigns",
		"List recent mail campaigns with status and delivery counts.",
		RiskRead,
		schema(map[string]any{
			"limit": map[string]any{"type": "integer", "description": "Max rows (default 20, max 100)"},
		}, nil),
		"",
		toolListMailCampaigns,
	))
	Register(newTool(
		"get_mail_campaign",
		"Get one mail campaign's status, counts, and recipient total.",
		RiskRead,
		schema(map[string]any{
			"campaign_id": map[string]any{"type": "integer", "description": "Campaign ID"},
		}, []string{"campaign_id"}),
		"",
		toolGetMailCampaign,
	))
	Register(newTool(
		"list_mail_templates",
		"List reusable mail templates (id, name, subject, is_html).",
		RiskRead,
		schema(map[string]any{
			"limit": map[string]any{"type": "integer", "description": "Max rows (default 50, max 200)"},
		}, nil),
		"",
		toolListMailTemplates,
	))
}

func toolCreateMailDraft(ec ExecContext, args map[string]any) Result {
	if !platform.EffectiveMailTrackingEnabled() {
		return Result{OK: false, Error: "Mail tracking is disabled"}
	}
	smtpID := jsonUintPtr(args["smtp_id"])
	if smtpID == nil {
		return Result{OK: false, Error: "smtp_id is required"}
	}
	subject := jsonString(args["subject"])
	body, _ := args["body"].(string)
	if subject == "" {
		return Result{OK: false, Error: "subject is required"}
	}
	if strings.TrimSpace(body) == "" {
		return Result{OK: false, Error: "body is required"}
	}
	recipients, err := normalizeRecipients(jsonStringSlice(args["recipients"]))
	if err != nil {
		return Result{OK: false, Error: err.Error()}
	}

	db := config.GetDB()
	var smtpService models.SmtpService
	if err := platform.ScopeByOwner(db, ec.User).First(&smtpService, *smtpID).Error; err != nil {
		return Result{OK: false, Error: "SMTP service not found"}
	}
	if !smtpService.Enabled {
		return Result{OK: false, Error: "SMTP service is disabled"}
	}

	projectID := jsonUintPtr(args["project_id"])
	var project models.Project
	if projectID != nil {
		if err := platform.ScopeByOwner(db, ec.User).First(&project, *projectID).Error; err != nil {
			return Result{OK: false, Error: "Project not found"}
		}
	}

	var templateID *uint
	if id := jsonUintPtr(args["template_id"]); id != nil {
		var tpl models.MailTemplate
		if err := platform.ScopeByOwner(db, ec.User).First(&tpl, *id).Error; err != nil {
			return Result{OK: false, Error: "Mail template not found"}
		}
		tid := tpl.ID
		templateID = &tid
	}

	landingURL, err := resolveDraftLandingURL(projectID, project, jsonString(args["landing_url"]))
	if err != nil {
		return Result{OK: false, Error: err.Error()}
	}

	isHTML := jsonBool(args["is_html"], false)
	trackClicks := jsonBool(args["track_clicks"], true)
	trackOpens := jsonBool(args["track_opens"], isHTML)
	if !isHTML {
		trackOpens = false
	}

	campaign := models.MailCampaign{
		SmtpServiceID:  smtpService.ID,
		ProjectID:      projectID,
		TemplateID:     templateID,
		Subject:        subject,
		BodyTemplate:   body,
		IsHTML:         isHTML,
		TrackOpens:     trackOpens,
		TrackClicks:    trackClicks,
		LandingURL:     landingURL,
		PublicBaseURL:  platform.ResolvePublicBaseURL("", ""),
		Status:         models.MailCampaignStatusScheduled,
		RecipientCount: len(recipients),
		CreatedBy:      ec.User.ID,
	}
	// No scheduled_at: a draft is never delivered by the scheduler.
	if err := db.Create(&campaign).Error; err != nil {
		return Result{OK: false, Error: "Failed to create campaign: " + err.Error()}
	}

	secret := platform.EffectiveMailTrackingSecret()
	campaign.ClickPath = config.DeriveMailTrackingAPIPath(secret, fmt.Sprintf("click|%d", campaign.ID))
	campaign.OpenPath = config.DeriveMailTrackingAPIPath(secret, fmt.Sprintf("open|%d", campaign.ID))
	if campaign.ClickPath == campaign.OpenPath {
		campaign.OpenPath = config.DeriveMailTrackingAPIPath(secret, fmt.Sprintf("open|%d|x", campaign.ID))
	}
	_ = db.Model(&campaign).Updates(map[string]any{
		"click_path": campaign.ClickPath,
		"open_path":  campaign.OpenPath,
	}).Error

	for _, email := range recipients {
		_ = db.Create(&models.MailRecipient{
			CampaignID: campaign.ID,
			Email:      strings.ToLower(email),
		}).Error
	}

	return Result{
		OK:      true,
		Summary: fmt.Sprintf("created mail draft #%d for %d recipient(s); not sent", campaign.ID, len(recipients)),
		Result: map[string]any{
			"id":              campaign.ID,
			"subject":         campaign.Subject,
			"status":          campaign.Status,
			"recipient_count": len(recipients),
			"landing_url":     landingURL,
			"sent":            false,
		},
		Links: []Link{{Label: "发信工作台", Href: "/workbench/mail"}},
	}
}

func toolSendMailCampaign(ec ExecContext, args map[string]any) Result {
	campaignID := jsonUintPtr(args["campaign_id"])
	if campaignID == nil {
		return Result{OK: false, Error: "campaign_id is required"}
	}
	db := config.GetDB()
	var campaign models.MailCampaign
	if err := platform.ScopeByOwner(db, ec.User).First(&campaign, *campaignID).Error; err != nil {
		return Result{OK: false, Error: "Campaign not found"}
	}
	switch campaign.Status {
	case models.MailCampaignStatusScheduled:
		// ok
	case models.MailCampaignStatusSending:
		return Result{OK: false, Error: "Campaign is already sending"}
	case models.MailCampaignStatusSent, models.MailCampaignStatusPartial, models.MailCampaignStatusFailed:
		return Result{OK: false, Error: "Campaign has already been delivered; create a new draft"}
	default:
		return Result{OK: false, Error: "Campaign is not in a sendable state"}
	}
	var recipientCount int64
	db.Model(&models.MailRecipient{}).Where("campaign_id = ?", campaign.ID).Count(&recipientCount)
	if recipientCount == 0 {
		return Result{OK: false, Error: "Campaign has no recipients"}
	}

	now := time.Now().UTC()
	if err := db.Model(&campaign).Updates(map[string]any{
		"scheduled_at": now,
		"status":       models.MailCampaignStatusScheduled,
		"updated_at":   now,
	}).Error; err != nil {
		return Result{OK: false, Error: "Failed to queue campaign: " + err.Error()}
	}

	return Result{
		OK:      true,
		Summary: fmt.Sprintf("queued campaign #%d for delivery to %d recipient(s)", campaign.ID, recipientCount),
		Result: map[string]any{
			"id":              campaign.ID,
			"status":          models.MailCampaignStatusScheduled,
			"recipient_count": recipientCount,
			"queued_at":       now,
		},
		Links: []Link{{Label: "发信工作台", Href: "/workbench/mail"}},
	}
}

func normalizeRecipients(values []string) ([]string, error) {
	raw := utils.NormalizeEmailList(values)
	if len(raw) == 0 {
		return nil, fmt.Errorf("recipients is required")
	}
	for _, addr := range raw {
		if _, err := mail.ParseAddress(addr); err != nil {
			return nil, fmt.Errorf("invalid email address: %s", addr)
		}
	}
	return raw, nil
}

func resolveDraftLandingURL(projectID *uint, project models.Project, landingURL string) (string, error) {
	landingURL = strings.TrimSpace(landingURL)
	if landingURL != "" {
		if utils.ExtractURLHost(landingURL) == "" {
			return "", fmt.Errorf("landing_url is invalid")
		}
		return landingURL, nil
	}
	if projectID == nil || *projectID == 0 {
		return "", fmt.Errorf("landing_url or project_id is required")
	}
	if strings.TrimSpace(project.LoginURL) == "" {
		return "", fmt.Errorf("project has no login_url; provide landing_url")
	}
	if utils.ExtractURLHost(project.LoginURL) == "" {
		return "", fmt.Errorf("project login_url is invalid")
	}
	return strings.TrimSpace(project.LoginURL), nil
}

func toolPreviewMail(ec ExecContext, args map[string]any) Result {
	db := config.GetDB()
	subject := jsonString(args["subject"])
	body, _ := args["body"].(string)
	isHTML := jsonBool(args["is_html"], false)
	trackClicks := jsonBool(args["track_clicks"], true)
	trackOpens := jsonBool(args["track_opens"], isHTML)
	landingURL := jsonString(args["landing_url"])
	recipient := jsonString(args["recipient"])
	recipientCount := 0

	if campaignID := jsonUintPtr(args["campaign_id"]); campaignID != nil {
		var campaign models.MailCampaign
		if err := platform.ScopeByOwner(db, ec.User).First(&campaign, *campaignID).Error; err != nil {
			return Result{OK: false, Error: "campaign not found"}
		}
		subject = campaign.Subject
		body = campaign.BodyTemplate
		isHTML = campaign.IsHTML
		trackOpens = campaign.TrackOpens
		trackClicks = campaign.TrackClicks
		landingURL = campaign.LandingURL
		var count int64
		db.Model(&models.MailRecipient{}).Where("campaign_id = ?", campaign.ID).Count(&count)
		recipientCount = int(count)
		if recipient == "" {
			var row models.MailRecipient
			if db.Where("campaign_id = ?", campaign.ID).Order("id ASC").First(&row).Error == nil {
				recipient = row.Email
			}
		}
	} else if templateID := jsonUintPtr(args["template_id"]); templateID != nil {
		var tpl models.MailTemplate
		if err := platform.ScopeByOwner(db, ec.User).First(&tpl, *templateID).Error; err != nil {
			return Result{OK: false, Error: "mail template not found"}
		}
		subject = tpl.Subject
		body = tpl.Body
		isHTML = tpl.IsHTML
	}

	if strings.TrimSpace(subject) == "" || strings.TrimSpace(body) == "" {
		return Result{OK: false, Error: "subject and body (or campaign_id/template_id) are required"}
	}
	if !isHTML {
		trackOpens = false
	}
	if recipient == "" {
		recipient = "recipient@example.com"
	}
	if landingURL == "" {
		landingURL = "https://example.com/login"
	}
	rendered := utils.RenderMailTemplate(body, map[string]string{
		"email":       recipient,
		"sig":         "preview-sig",
		"v":           "preview-v",
		"landing_url": landingURL,
		"click_url":   landingURL,
		"open_pixel":  "",
	}, isHTML, trackOpens)

	return Result{
		OK:      true,
		Summary: fmt.Sprintf("mail preview: %s", subject),
		Result: map[string]any{
			"subject":          subject,
			"body":             truncateRunes(rendered, 4000),
			"is_html":          isHTML,
			"track_opens":      trackOpens,
			"track_clicks":     trackClicks,
			"recipient_sample": recipient,
			"recipient_count":  recipientCount,
			"landing_url":      landingURL,
		},
		Links: []Link{{Label: "发信工作台", Href: "/workbench/mail"}},
	}
}

func toolListMailCampaigns(ec ExecContext, args map[string]any) Result {
	limit := 20
	if n, ok := jsonInt(args["limit"]); ok && n > 0 {
		limit = int(n)
	}
	if limit > 100 {
		limit = 100
	}
	db := config.GetDB()
	var rows []models.MailCampaign
	if err := platform.ScopeByOwner(db.Model(&models.MailCampaign{}), ec.User).
		Order("id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return Result{OK: false, Error: "failed to list mail campaigns"}
	}
	out := make([]map[string]any, 0, len(rows))
	for _, c := range rows {
		out = append(out, map[string]any{
			"id":              c.ID,
			"subject":         c.Subject,
			"status":          c.Status,
			"recipient_count": c.RecipientCount,
			"sent_count":      c.SentCount,
			"failed_count":    c.FailedCount,
			"scheduled_at":    c.ScheduledAt,
			"created_at":      c.CreatedAt,
		})
	}
	return Result{
		OK:      true,
		Summary: fmt.Sprintf("%d mail campaigns", len(out)),
		Result:  out,
		Links:   []Link{{Label: "发信工作台", Href: "/workbench/mail"}},
	}
}

func toolGetMailCampaign(ec ExecContext, args map[string]any) Result {
	campaignID := jsonUintPtr(args["campaign_id"])
	if campaignID == nil {
		return Result{OK: false, Error: "campaign_id is required"}
	}
	db := config.GetDB()
	var campaign models.MailCampaign
	if err := platform.ScopeByOwner(db, ec.User).First(&campaign, *campaignID).Error; err != nil {
		return Result{OK: false, Error: "campaign not found"}
	}
	var recipients int64
	db.Model(&models.MailRecipient{}).Where("campaign_id = ?", campaign.ID).Count(&recipients)
	return Result{
		OK:      true,
		Summary: fmt.Sprintf("campaign #%d %s (%s)", campaign.ID, campaign.Subject, campaign.Status),
		Result: map[string]any{
			"id":               campaign.ID,
			"subject":          campaign.Subject,
			"status":           campaign.Status,
			"recipient_count":  campaign.RecipientCount,
			"recipients_in_db": recipients,
			"sent_count":       campaign.SentCount,
			"failed_count":     campaign.FailedCount,
			"open_count":       campaign.OpenCount,
			"click_count":      campaign.ClickCount,
			"submit_count":     campaign.SubmitCount,
			"scheduled_at":     campaign.ScheduledAt,
			"landing_url":      campaign.LandingURL,
			"created_at":       campaign.CreatedAt,
		},
		Links: []Link{{Label: "邮件详情", Href: fmt.Sprintf("/workbench/mail/%d", campaign.ID)}},
	}
}

func toolListMailTemplates(ec ExecContext, args map[string]any) Result {
	limit := 50
	if n, ok := jsonInt(args["limit"]); ok && n > 0 {
		limit = int(n)
	}
	if limit > 200 {
		limit = 200
	}
	db := config.GetDB()
	var rows []models.MailTemplate
	if err := platform.ScopeByOwner(db.Model(&models.MailTemplate{}), ec.User).
		Order("id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return Result{OK: false, Error: "failed to list mail templates"}
	}
	out := make([]map[string]any, 0, len(rows))
	for _, tpl := range rows {
		out = append(out, map[string]any{
			"id":      tpl.ID,
			"name":    tpl.Name,
			"subject": tpl.Subject,
			"is_html": tpl.IsHTML,
		})
	}
	return Result{
		OK:      true,
		Summary: fmt.Sprintf("%d mail templates", len(out)),
		Result:  out,
		Links:   []Link{{Label: "发信工作台", Href: "/workbench/mail"}},
	}
}
