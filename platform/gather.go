package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"spearhook/chatbus"
	"spearhook/config"
	"spearhook/models"
	"spearhook/utils"
)

const (
	InfoGatherMaxConcurrent  = 3
	InfoGatherDefaultTimeout = 180 * time.Second
)

var (
	infoGatherSem     = make(chan struct{}, InfoGatherMaxConcurrent)
	infoGatherRunMu   sync.Mutex
	infoGatherRunning = map[uint]struct{}{}
)

// StartInfoGatherJob launches the async runner for an existing job row.
func StartInfoGatherJob(jobID uint) {
	go RunInfoGatherJob(jobID)
}

// RunInfoGatherJob executes one info gather job against the active AI profile.
func RunInfoGatherJob(jobID uint) {
	infoGatherRunMu.Lock()
	if _, busy := infoGatherRunning[jobID]; busy {
		infoGatherRunMu.Unlock()
		return
	}
	infoGatherRunning[jobID] = struct{}{}
	infoGatherRunMu.Unlock()
	defer func() {
		infoGatherRunMu.Lock()
		delete(infoGatherRunning, jobID)
		infoGatherRunMu.Unlock()
	}()

	infoGatherSem <- struct{}{}
	defer func() { <-infoGatherSem }()

	db := config.GetDB()
	var job models.InfoGatherJob
	if err := db.First(&job, jobID).Error; err != nil {
		return
	}

	now := time.Now()
	_ = db.Model(&job).Updates(map[string]any{
		"status":           models.InfoGatherJobRunning,
		"started_at":       now,
		"updated_at":       now,
		"progress_stage":   "starting",
		"progress_percent": 5,
		"progress_message": "Starting info gathering",
	}).Error

	profile, apiKey, active := ActiveAIConfig()
	if !active || strings.TrimSpace(apiKey) == "" {
		FinishInfoGatherJob(jobID, models.InfoGatherJobFailed, "No AI model is enabled", "", nil, "")
		return
	}

	timeout := time.Duration(profile.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = InfoGatherDefaultTimeout
	}
	if timeout < 120*time.Second {
		timeout = 120 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout+30*time.Second)
	defer cancel()
	ctx = utils.WithAIProfileID(ctx, profile.ID)

	setProgress := func(stage string, percent int, message string) {
		_ = db.Model(&models.InfoGatherJob{}).Where("id = ?", jobID).Updates(map[string]any{
			"progress_stage":   stage,
			"progress_percent": percent,
			"progress_message": message,
			"updated_at":       time.Now(),
		}).Error
		if job.ChatSessionID > 0 {
			chatbus.Publish(job.ChatSessionID, "job_progress", map[string]any{
				"job_id":  jobID,
				"status":  "running",
				"stage":   stage,
				"percent": percent,
				"message": message,
			})
		}
	}

	result, err := utils.GatherContactsWithAIFormat(ctx, profile.APIFormat, profile.BaseURL, apiKey, profile.Model, timeout, utils.InfoGatherInput{
		Target:         job.Target,
		Notes:          job.Notes,
		IncludeXSearch: job.IncludeXSearch,
		OnProgress:     setProgress,
		Searches:       SearchConfigs(),
	})

	if err != nil && len(result.Findings) == 0 {
		FinishInfoGatherJob(jobID, models.InfoGatherJobFailed, err.Error(), result.RawResponse, nil, result.Notes)
		return
	}

	status := models.InfoGatherJobSucceeded
	errMsg := ""
	if err != nil {
		status = models.InfoGatherJobPartial
		errMsg = err.Error()
	}

	setProgress("saving", 95, "Saving email/phone findings")
	findings := make([]models.InfoGatherFinding, 0, len(result.Findings))
	for _, f := range result.Findings {
		if f.Kind != "email" && f.Kind != "phone" {
			continue
		}
		findings = append(findings, models.InfoGatherFinding{
			JobID:     jobID,
			Kind:      f.Kind,
			Value:     f.Value,
			Label:     f.Label,
			SourceURL: f.SourceURL,
			Snippet:   f.Snippet,
		})
	}
	FinishInfoGatherJob(jobID, status, errMsg, result.RawResponse, findings, result.Notes)
}

// FinishInfoGatherJob persists findings and terminal job state.
func FinishInfoGatherJob(jobID uint, status models.InfoGatherJobStatus, errMsg, raw string, findings []models.InfoGatherFinding, notes string) {
	db := config.GetDB()
	var job models.InfoGatherJob
	_ = db.First(&job, jobID).Error
	tx := db.Begin()
	if tx.Error != nil {
		return
	}

	emailCount, phoneCount := 0, 0
	if len(findings) > 0 {
		_ = tx.Where("job_id = ?", jobID).Delete(&models.InfoGatherFinding{})
		for i := range findings {
			findings[i].JobID = jobID
			if err := tx.Create(&findings[i]).Error; err != nil {
				// unique conflict → skip duplicate
				continue
			}
			switch findings[i].Kind {
			case "email":
				emailCount++
			case "phone":
				phoneCount++
			}
		}
	} else {
		var existing []models.InfoGatherFinding
		_ = tx.Where("job_id = ?", jobID).Find(&existing).Error
		for _, f := range existing {
			switch f.Kind {
			case "email":
				emailCount++
			case "phone":
				phoneCount++
			}
		}
		findings = existing
	}

	finished := time.Now()
	updates := map[string]any{
		"status":           status,
		"error_message":    errMsg,
		"raw_response":     raw,
		"summary_notes":    notes,
		"email_count":      emailCount,
		"phone_count":      phoneCount,
		"finding_count":    len(findings),
		"progress_percent": 100,
		"finished_at":      finished,
		"updated_at":       finished,
	}
	switch status {
	case models.InfoGatherJobFailed:
		updates["progress_message"] = errMsg
		if errMsg == "" {
			updates["progress_message"] = "Failed"
		}
		// Keep progress_stage at the step that failed (already written by OnProgress).
	case models.InfoGatherJobPartial:
		updates["progress_stage"] = "done"
		updates["progress_message"] = "Completed with warnings"
	default:
		updates["progress_stage"] = "done"
		updates["progress_message"] = "Completed"
	}
	if err := tx.Model(&models.InfoGatherJob{}).Where("id = ?", jobID).Updates(updates).Error; err != nil {
		tx.Rollback()
		return
	}
	_ = tx.Commit()

	if job.ChatSessionID > 0 {
		payload := map[string]any{
			"job_id":        jobID,
			"status":        string(status),
			"stage":         updates["progress_stage"],
			"percent":       updates["progress_percent"],
			"message":       updates["progress_message"],
			"email_count":   emailCount,
			"phone_count":   phoneCount,
			"finding_count": len(findings),
			"link":          fmt.Sprintf("/workbench/info-gathering/%d", jobID),
		}
		raw, marshalErr := json.Marshal(payload)
		if marshalErr == nil {
			// Persist the terminal state so reloading the session still shows it.
			_ = db.Create(&models.ChatMessage{
				SessionID: job.ChatSessionID,
				Role:      "system",
				ToolName:  "info_gather_job",
				Content:   string(raw),
				Status:    "done",
			}).Error
		}
		chatbus.Publish(job.ChatSessionID, "job_done", payload)
	}
}
