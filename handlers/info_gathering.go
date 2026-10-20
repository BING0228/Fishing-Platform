package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"spearhook/config"
	"spearhook/models"
	"spearhook/platform"

	"github.com/gin-gonic/gin"
)

type createInfoGatherJobRequest struct {
	Target         string `json:"target"`
	Notes          string `json:"notes"`
	IncludeXSearch bool   `json:"include_x_search"`
}

// GetInfoGatherJobs lists jobs for the current user (admins see all).
func GetInfoGatherJobs(c *gin.Context) {
	user, ok := currentUserOrAbort(c)
	if !ok {
		return
	}
	db := config.GetDB()
	var jobs []models.InfoGatherJob
	if err := scopeByOwner(db.Model(&models.InfoGatherJob{}), user).
		Order("id DESC").
		Find(&jobs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list info gather jobs"})
		return
	}
	c.JSON(http.StatusOK, jobs)
}

// GetInfoGatherJob returns one job.
func GetInfoGatherJob(c *gin.Context) {
	job, ok := loadInfoGatherJobOwned(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, job)
}

// GetInfoGatherFindings returns findings for a job.
func GetInfoGatherFindings(c *gin.Context) {
	job, ok := loadInfoGatherJobOwned(c)
	if !ok {
		return
	}
	db := config.GetDB()
	var findings []models.InfoGatherFinding
	if err := db.Where("job_id = ?", job.ID).Order("id ASC").Find(&findings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list findings"})
		return
	}
	c.JSON(http.StatusOK, findings)
}

// CreateInfoGatherJob creates a job and starts async AI search.
func CreateInfoGatherJob(c *gin.Context) {
	user, ok := currentUserOrAbort(c)
	if !ok {
		return
	}
	var req createInfoGatherJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}
	target := strings.TrimSpace(req.Target)
	if target == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target is required"})
		return
	}
	if len(target) > 500 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target is too long"})
		return
	}

	profile, apiKey, active := ActiveAIConfig()
	if !active {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No AI model is enabled. Configure one in System → AI Settings"})
		return
	}
	if strings.TrimSpace(apiKey) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Enabled AI profile has no API key"})
		return
	}

	job := models.InfoGatherJob{
		Target:         target,
		Notes:          strings.TrimSpace(req.Notes),
		IncludeXSearch: req.IncludeXSearch,
		Status:         models.InfoGatherJobPending,
		Model:          profile.Model,
		CreatedBy:      user.ID,
	}
	db := config.GetDB()
	if err := db.Create(&job).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create job"})
		return
	}

	platform.StartInfoGatherJob(job.ID)
	c.JSON(http.StatusCreated, job)
}

// RetryInfoGatherJob clears findings and re-runs the job.
func RetryInfoGatherJob(c *gin.Context) {
	job, ok := loadInfoGatherJobOwned(c)
	if !ok {
		return
	}
	if job.Status == models.InfoGatherJobRunning || job.Status == models.InfoGatherJobPending {
		c.JSON(http.StatusConflict, gin.H{"error": "Job is already running"})
		return
	}
	profile, apiKey, active := ActiveAIConfig()
	if !active || strings.TrimSpace(apiKey) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No AI model is enabled. Configure one in System → AI Settings"})
		return
	}

	db := config.GetDB()
	if err := db.Where("job_id = ?", job.ID).Delete(&models.InfoGatherFinding{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear previous findings"})
		return
	}
	now := time.Now()
	updates := map[string]any{
		"status":           models.InfoGatherJobPending,
		"error_message":    "",
		"raw_response":     "",
		"summary_notes":    "",
		"email_count":      0,
		"phone_count":      0,
		"finding_count":    0,
		"progress_stage":   "starting",
		"progress_percent": 0,
		"progress_message": "",
		"model":            profile.Model,
		"started_at":       nil,
		"finished_at":      nil,
		"updated_at":       now,
	}
	if err := db.Model(&job).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reset job"})
		return
	}
	_ = db.First(&job, job.ID)
	platform.StartInfoGatherJob(job.ID)
	c.JSON(http.StatusOK, job)
}

// DeleteInfoGatherJob deletes a job and its findings (owner or admin).
func DeleteInfoGatherJob(c *gin.Context) {
	job, ok := loadInfoGatherJobOwned(c)
	if !ok {
		return
	}
	if job.Status == models.InfoGatherJobRunning {
		c.JSON(http.StatusConflict, gin.H{"error": "Cannot delete a running job"})
		return
	}
	db := config.GetDB()
	tx := db.Begin()
	if err := tx.Where("job_id = ?", job.ID).Delete(&models.InfoGatherFinding{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete findings"})
		return
	}
	if err := tx.Delete(&job).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete job"})
		return
	}
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit delete"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func loadInfoGatherJobOwned(c *gin.Context) (models.InfoGatherJob, bool) {
	user, ok := currentUserOrAbort(c)
	if !ok {
		return models.InfoGatherJob{}, false
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job id"})
		return models.InfoGatherJob{}, false
	}
	db := config.GetDB()
	var job models.InfoGatherJob
	if err := db.First(&job, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return models.InfoGatherJob{}, false
	}
	if !canAccessOwned(user, job.CreatedBy) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return models.InfoGatherJob{}, false
	}
	return job, true
}
