package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"fishing-platform-backend/config"
	"fishing-platform-backend/models"
	"fishing-platform-backend/utils"
	"github.com/gin-gonic/gin"
)

type mirrorPageRequest struct {
	URL         string `json:"url" binding:"required"`
	Name        string `json:"name"`
	Description string `json:"description"`
	SubmitURL   string `json:"submit_url"`
	RedirectURL string `json:"redirect_url"`
}

type mirrorProgressEvent struct {
	Stage       string      `json:"stage"`
	Percent     int         `json:"percent,omitempty"`
	Message     string      `json:"message,omitempty"`
	Error       string      `json:"error,omitempty"`
	StageFailed string      `json:"stage_failed,omitempty"`
	Detail      interface{} `json:"detail,omitempty"`
	Page        interface{} `json:"page,omitempty"`
}

// MirrorPhishingPage fetches a URL, rewrites login submit via AI, and saves a phishing page.
// Progress is streamed as SSE events so the UI can show real stages.
func MirrorPhishingPage(c *gin.Context) {
	user, ok := currentUserOrAbort(c)
	if !ok {
		return
	}

	var req mirrorPageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url is required"})
		return
	}

	// AI connectivity is checked in System → AI Settings; mirror only requires an enabled profile.
	profile, apiKey, active := ActiveAIConfig()
	if !active {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No AI model is enabled. Open System → AI Settings to configure and test one."})
		return
	}
	if strings.TrimSpace(apiKey) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Enabled AI profile has no API key. Open System → AI Settings to fix and test it."})
		return
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming unsupported"})
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	flusher.Flush()

	var writeMu sync.Mutex
	writeSSE := func(eventName string, payload []byte) bool {
		writeMu.Lock()
		defer writeMu.Unlock()
		if c.Request.Context().Err() != nil {
			return false
		}
		if _, err := fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", eventName, payload); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	emit := func(ev mirrorProgressEvent) bool {
		if c.Request.Context().Err() != nil {
			return false
		}
		raw, err := json.Marshal(ev)
		if err != nil {
			return false
		}
		return writeSSE("progress", raw)
	}
	fail := func(stage, msg string) {
		_ = emit(mirrorProgressEvent{
			Stage:       "failed",
			StageFailed: stage,
			Error:       msg,
			Message:     msg,
		})
	}

	submitURL := utils.LandingSubmitToken(req.SubmitURL)
	redirectURL := utils.LandingRedirectToken(req.RedirectURL)

	if !emit(mirrorProgressEvent{Stage: "fetching", Percent: 20, Message: "Fetching target page"}) {
		return
	}
	rawHTML, finalURL, err := utils.FetchURLHTML(c.Request.Context(), req.URL, 30*time.Second)
	if err != nil {
		fail("fetching", "Failed to fetch page: "+err.Error())
		return
	}

	originalHost := ""
	if finalURL != nil {
		originalHost = finalURL.Host
	}
	aiHTML := utils.StripHeavyBase64DataURIs(rawHTML)
	timeout := utils.AIRewriteTimeout(time.Duration(profile.TimeoutSec)*time.Second, len(aiHTML))
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeout+45*time.Second)
	defer cancel()

	// Keepalive pings while AI may take a long time.
	pingStop := make(chan struct{})
	defer close(pingStop)
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-pingStop:
				return
			case <-c.Request.Context().Done():
				return
			case <-ticker.C:
				_ = writeSSE("ping", []byte("{}"))
			}
		}
	}()

	if !emit(mirrorProgressEvent{
		Stage:   "preparing",
		Percent: 35,
		Message: "Preparing HTML for AI",
		Detail: map[string]interface{}{
			"host":  originalHost,
			"bytes": len(rawHTML),
		},
	}) {
		return
	}

	if !emit(mirrorProgressEvent{
		Stage:   "rewriting",
		Percent: 45,
		Message: "AI rewriting login form",
		Detail: map[string]interface{}{
			"model": profile.Model,
			"host":  originalHost,
			"bytes": len(aiHTML),
		},
	}) {
		return
	}
	rewritten, err := utils.RewriteHTMLWithAI(ctx, profile.BaseURL, apiKey, profile.Model, timeout, utils.MirrorRewriteInput{
		RawHTML:      aiHTML,
		SubmitURL:    submitURL,
		RedirectURL:  redirectURL,
		OriginalHost: originalHost,
	})
	if err != nil {
		fail("rewriting", "AI rewrite failed: "+err.Error())
		return
	}
	if !emit(mirrorProgressEvent{Stage: "rewriting", Percent: 85, Message: "AI rewrite finished"}) {
		return
	}

	if !emit(mirrorProgressEvent{Stage: "validating", Percent: 90, Message: "Validating rewritten HTML"}) {
		return
	}
	if err := utils.ValidateRewrittenHTML(rewritten, submitURL); err != nil {
		fail("validating", "AI rewrite failed validation: "+err.Error())
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		if finalURL != nil && finalURL.Host != "" {
			name = finalURL.Host
		} else {
			name = "Mirrored page"
		}
	}
	pageURL := strings.TrimSpace(req.URL)
	if finalURL != nil {
		pageURL = finalURL.String()
	}
	desc := strings.TrimSpace(req.Description)
	if desc == "" {
		desc = "Mirrored from " + pageURL
	}

	if !emit(mirrorProgressEvent{Stage: "saving", Percent: 95, Message: "Saving mirrored page"}) {
		return
	}
	page := models.PhishingPage{
		Name:        name,
		URL:         pageURL,
		Description: desc,
		Html:        rewritten,
		CreatedBy:   user.ID,
	}
	db := config.GetDB()
	if err := db.Create(&page).Error; err != nil {
		fail("saving", "Failed to save mirrored page")
		return
	}

	_ = emit(mirrorProgressEvent{
		Stage:   "done",
		Percent: 100,
		Message: "Page mirrored successfully",
		Page:    page,
	})
}
