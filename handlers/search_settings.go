package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"spearhook/config"
	"spearhook/models"
	"spearhook/platform"
	"spearhook/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type searchProviderDTO struct {
	ID             string `json:"id"`
	Label          string `json:"label"`
	DefaultBaseURL string `json:"default_base_url"`
	RequiresKey    bool   `json:"requires_key"`
	Native         bool   `json:"native,omitempty"`
}

type searchProfileDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	Provider  string `json:"provider"`
	BaseURL   string `json:"base_url"`
	APIKey    string `json:"api_key"`
	APIKeySet bool   `json:"api_key_set"`
	Count     int    `json:"count"`
}

type searchSettingsDTO struct {
	Profiles  []searchProfileDTO  `json:"profiles"`
	Providers []searchProviderDTO `json:"providers"`
}

type searchSettingsPayload struct {
	Profiles []searchProfileDTO `json:"profiles"`
}

type searchTestRequest struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key"`
	Count    int    `json:"count"`
}

func searchProviderDTOs() []searchProviderDTO {
	rows := utils.SearchProviders()
	out := make([]searchProviderDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, searchProviderDTO{
			ID:             row.ID,
			Label:          row.Label,
			DefaultBaseURL: row.DefaultBaseURL,
			RequiresKey:    row.RequiresKey,
			Native:         row.Native,
		})
	}
	return out
}

func newSearchProfileID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("search-%d", time.Now().UnixNano())
	}
	return "search-" + hex.EncodeToString(buf)
}

func searchProfilePlainKey(row models.SearchProfile) string {
	if strings.TrimSpace(row.APIKey) == "" {
		return ""
	}
	if dec, err := utils.Decrypt(row.APIKey); err == nil && dec != "" {
		return dec
	}
	return row.APIKey
}

func toSearchProfileDTO(row models.SearchProfile) searchProfileDTO {
	plain := searchProfilePlainKey(row)
	return searchProfileDTO{
		ID:        row.ID,
		Name:      row.Name,
		Enabled:   row.Enabled,
		Provider:  utils.NormalizeSearchProvider(row.Provider),
		BaseURL:   row.BaseURL,
		APIKey:    maskSecret(plain),
		APIKeySet: plain != "",
		Count:     utils.SearchCountOrDefault(row.Count),
	}
}

func toSearchProfileDTOs(rows []models.SearchProfile) []searchProfileDTO {
	out := make([]searchProfileDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toSearchProfileDTO(row))
	}
	return out
}

// GetSearchSettings returns web-search profiles from the database (admin only,
// keys masked). Any number of profiles may be enabled.
func GetSearchSettings(c *gin.Context) {
	rows, err := platform.EnsureSearchProfiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load search settings"})
		return
	}
	c.JSON(http.StatusOK, searchSettingsDTO{
		Profiles:  toSearchProfileDTOs(rows),
		Providers: searchProviderDTOs(),
	})
}

// UpdateSearchSettings replaces the web-search profiles. Unlike AI profiles,
// any number of rows may be enabled; enabled standalone APIs are all called.
func UpdateSearchSettings(c *gin.Context) {
	var payload searchSettingsPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}
	if len(payload.Profiles) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one search profile is required"})
		return
	}

	db := config.GetDB()
	var existing []models.SearchProfile
	if err := db.Find(&existing).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load search settings"})
		return
	}
	existingByID := map[string]models.SearchProfile{}
	for _, row := range existing {
		existingByID[row.ID] = row
	}

	merged := make([]models.SearchProfile, 0, len(payload.Profiles))
	seenIDs := map[string]struct{}{}
	for _, p := range payload.Profiles {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "each search profile requires a name"})
			return
		}
		provider := utils.NormalizeSearchProvider(p.Provider)
		if !utils.IsKnownSearchProvider(provider) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported search provider"})
			return
		}
		baseURL := strings.TrimRight(strings.TrimSpace(p.BaseURL), "/")
		if provider == utils.SearchProviderAuto {
			baseURL = ""
		} else if baseURL == "" {
			baseURL = utils.DefaultSearchBaseURL(provider)
		}
		count := utils.SearchCountOrDefault(p.Count)

		id := strings.TrimSpace(p.ID)
		if id == "" {
			id = newSearchProfileID()
		}
		if _, ok := seenIDs[id]; ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "duplicate search profile id"})
			return
		}
		seenIDs[id] = struct{}{}

		apiKeyPlain := strings.TrimSpace(p.APIKey)
		encryptedKey := ""
		if apiKeyPlain == "" || looksMaskedSecret(apiKeyPlain) {
			if old, ok := existingByID[id]; ok {
				encryptedKey = old.APIKey
			} else if looksMaskedSecret(apiKeyPlain) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "api_key is required for a new profile"})
				return
			}
		} else {
			enc, err := utils.Encrypt(apiKeyPlain)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encrypt API key"})
				return
			}
			encryptedKey = enc
		}

		merged = append(merged, models.SearchProfile{
			ID:       id,
			Name:     name,
			Enabled:  p.Enabled,
			Provider: provider,
			BaseURL:  baseURL,
			APIKey:   encryptedKey,
			Count:    count,
		})
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("1 = 1").Delete(&models.SearchProfile{}).Error; err != nil {
			return err
		}
		for i := range merged {
			if err := tx.Create(&merged[i]).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save search settings"})
		return
	}

	c.JSON(http.StatusOK, searchSettingsDTO{
		Profiles:  toSearchProfileDTOs(merged),
		Providers: searchProviderDTOs(),
	})
}

// TestSearchSettings probes a saved profile or inline form values (admin).
func TestSearchSettings(c *gin.Context) {
	var req searchTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	var saved *models.SearchProfile
	if id := strings.TrimSpace(req.ID); id != "" {
		db := config.GetDB()
		var row models.SearchProfile
		if err := db.Where("id = ?", id).First(&row).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "search profile not found"})
			return
		}
		saved = &row
	}

	providerRaw := strings.TrimSpace(req.Provider)
	if providerRaw == "" && saved != nil {
		providerRaw = saved.Provider
	}
	provider := utils.NormalizeSearchProvider(providerRaw)
	if !utils.IsKnownSearchProvider(provider) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported search provider"})
		return
	}

	baseURL := strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
	if baseURL == "" && saved != nil {
		baseURL = strings.TrimRight(strings.TrimSpace(saved.BaseURL), "/")
	}
	if provider == utils.SearchProviderAuto {
		baseURL = ""
	} else if baseURL == "" {
		baseURL = utils.DefaultSearchBaseURL(provider)
	}

	apiKey := strings.TrimSpace(req.APIKey)
	if (apiKey == "" || looksMaskedSecret(apiKey)) && saved != nil {
		apiKey = searchProfilePlainKey(*saved)
	}

	count := req.Count
	if count <= 0 && saved != nil {
		count = saved.Count
	}
	count = utils.SearchCountOrDefault(count)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
	defer cancel()

	if provider == utils.SearchProviderAuto {
		profile, profileKey, active := ActiveAIConfig()
		if !active || strings.TrimSpace(profileKey) == "" {
			c.JSON(http.StatusOK, gin.H{"ok": false, "provider": provider, "error": "No AI profile is enabled"})
			return
		}
		result, err := utils.ProbeWebSearchFormat(ctx, profile.APIFormat, profile.BaseURL, profileKey, profile.Model, 60*time.Second)
		payload := gin.H{
			"ok":         err == nil,
			"provider":   provider,
			"mode":       result.Mode,
			"endpoint":   result.Endpoint,
			"latency_ms": result.LatencyMs,
			"status":     result.StatusCode,
		}
		if err != nil {
			payload["error"] = err.Error()
		}
		c.JSON(http.StatusOK, payload)
		return
	}

	if apiKey == "" {
		c.JSON(http.StatusOK, gin.H{"ok": false, "provider": provider, "error": "API key is required"})
		return
	}
	cfg := utils.SearchProviderConfig{
		Provider: provider,
		BaseURL:  baseURL,
		APIKey:   apiKey,
		Count:    count,
	}
	result, err := utils.ProbeSearchProvider(ctx, cfg, 30*time.Second)
	payload := gin.H{
		"ok":           err == nil,
		"provider":     provider,
		"endpoint":     result.Endpoint,
		"latency_ms":   result.LatencyMs,
		"status":       result.StatusCode,
		"result_count": result.ResultCount,
	}
	if err != nil {
		payload["error"] = err.Error()
	}
	c.JSON(http.StatusOK, payload)
}
