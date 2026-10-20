package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"spearhook/config"
	"spearhook/models"
	"spearhook/platform"
	"spearhook/utils"
)

type mailTrackingSettingsDTO struct {
	PublicBaseURL      string   `json:"public_base_url"`
	Enabled            bool     `json:"enabled"`
	Secret             string   `json:"secret"`
	SecretSet          bool     `json:"secret_set"`
	AllowRedirectHosts []string `json:"allow_redirect_hosts"`
}

type mailTrackingSettingsPayload struct {
	PublicBaseURL      string   `json:"public_base_url"`
	Enabled            *bool    `json:"enabled"`
	Secret             string   `json:"secret"`
	AllowRedirectHosts []string `json:"allow_redirect_hosts"`
}

// GetMailTrackingSettings returns platform-managed mail tracking config (admin).
func GetMailTrackingSettings(c *gin.Context) {
	row, err := ensureMailTrackingSetting()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load mail tracking settings"})
		return
	}
	c.JSON(http.StatusOK, toMailTrackingDTO(row))
}

// UpdateMailTrackingSettings updates the singleton mail tracking config (admin).
func UpdateMailTrackingSettings(c *gin.Context) {
	var payload mailTrackingSettingsPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	row, err := ensureMailTrackingSetting()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load mail tracking settings"})
		return
	}

	base := strings.TrimRight(strings.TrimSpace(payload.PublicBaseURL), "/")
	if base != "" {
		u, err := url.Parse(base)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "public_base_url must be an http(s) URL"})
			return
		}
		base = strings.TrimRight(u.String(), "/")
	}

	hosts := make([]string, 0, len(payload.AllowRedirectHosts))
	for _, h := range payload.AllowRedirectHosts {
		h = strings.ToLower(strings.TrimSpace(h))
		if h != "" {
			hosts = append(hosts, h)
		}
	}

	row.PublicBaseURL = base
	if payload.Enabled != nil {
		row.Enabled = *payload.Enabled
	}
	row.AllowRedirectHosts = strings.Join(hosts, "\n")
	// Click/open paths are generated per mail campaign, not configured globally.

	secretPlain := strings.TrimSpace(payload.Secret)
	if secretPlain != "" && !looksMaskedSecret(secretPlain) {
		enc, err := utils.Encrypt(secretPlain)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encrypt secret"})
			return
		}
		row.Secret = enc
	}

	db := config.GetDB()
	if err := db.Save(row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save mail tracking settings"})
		return
	}
	c.JSON(http.StatusOK, toMailTrackingDTO(row))
}

// EnsureMailTrackingSettingForBoot seeds DB settings from YAML when missing.
func EnsureMailTrackingSettingForBoot() (*models.MailTrackingSetting, error) {
	return ensureMailTrackingSetting()
}

func ensureMailTrackingSetting() (*models.MailTrackingSetting, error) {
	return platform.EnsureMailTrackingSetting()
}

func toMailTrackingDTO(row *models.MailTrackingSetting) mailTrackingSettingsDTO {
	plain := decryptMailTrackingSecret(row.Secret)
	hosts := splitHostLines(row.AllowRedirectHosts)
	return mailTrackingSettingsDTO{
		PublicBaseURL:      row.PublicBaseURL,
		Enabled:            row.Enabled,
		Secret:             maskSecret(plain),
		SecretSet:          plain != "",
		AllowRedirectHosts: hosts,
	}
}

func decryptMailTrackingSecret(enc string) string {
	return platform.DecryptMailTrackingSecret(enc)
}

func splitHostLines(raw string) []string {
	return platform.SplitHostLines(raw)
}

func loadMailTrackingSetting() *models.MailTrackingSetting {
	return platform.LoadMailTrackingSetting()
}

// EffectivePublicBaseURL prefers DB setting, then YAML, then empty.
func EffectivePublicBaseURL() string {
	return platform.EffectivePublicBaseURL()
}

// requestPublicBaseURL is the http(s) origin to turn relative submit paths
// into full URLs (DB/YAML public_base_url, else this request's host).
func requestPublicBaseURL(c *gin.Context) string {
	if base := EffectivePublicBaseURL(); base != "" {
		return base
	}
	scheme := "http"
	if c != nil && c.Request != nil {
		if c.Request.TLS != nil {
			scheme = "https"
		}
		if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
			scheme = strings.TrimSpace(strings.Split(proto, ",")[0])
		}
		if host := strings.TrimSpace(c.Request.Host); host != "" {
			return scheme + "://" + host
		}
	}
	return fmt.Sprintf("http://127.0.0.1:%d", config.ServerPort())
}

// EffectiveMailTrackingEnabled prefers DB.
func EffectiveMailTrackingEnabled() bool {
	return platform.EffectiveMailTrackingEnabled()
}

// EffectiveMailTrackingSecret prefers DB encrypted secret, then YAML/jwt fallback.
func EffectiveMailTrackingSecret() string {
	return platform.EffectiveMailTrackingSecret()
}

// EffectiveMailTrackingAllowRedirectHosts merges DB hosts with YAML hosts.
func EffectiveMailTrackingAllowRedirectHosts() []string {
	return platform.EffectiveMailTrackingAllowRedirectHosts()
}

// DispatchMailTracking routes GET /api/:slug to open/click handlers when the
// slug matches a per-campaign tracking path.
func isReservedAPISlug(slug string) bool {
	s := strings.ToLower(strings.Trim(strings.TrimSpace(slug), "/"))
	switch s {
	case "auth", "agents", "agent", "users", "projects", "robots", "messages",
		"credentials", "ip-blacklist", "smtp-services", "mail-campaigns",
		"phishing-pages", "dashboard", "audit-logs", "ai-settings",
		"mail-tracking-settings", "info-gather-jobs", "qr-relays", "qr-relay",
		"mail-templates", "robot_push_logs":
		return true
	default:
		return false
	}
}

func DispatchMailTracking(c *gin.Context) {
	slug := strings.TrimSpace(c.Param("slug"))
	if slug == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "API endpoint not found"})
		return
	}
	// Never let the catch-all /api/:slug shadow real platform API prefixes.
	if isReservedAPISlug(slug) {
		c.JSON(http.StatusNotFound, gin.H{"error": "API endpoint not found"})
		return
	}
	reqPath := config.NormalizeMailTrackingPath("/api/" + slug)
	db := config.GetDB()

	var openHit int64
	_ = db.Model(&models.MailCampaign{}).Where("open_path = ?", reqPath).Limit(1).Count(&openHit).Error
	if openHit > 0 {
		TrackMailOpen(c)
		return
	}
	var clickHit int64
	_ = db.Model(&models.MailCampaign{}).Where("click_path = ?", reqPath).Limit(1).Count(&clickHit).Error
	if clickHit > 0 {
		TrackMailClick(c)
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "API endpoint not found"})
}
