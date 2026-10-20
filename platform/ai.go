package platform

import (
	"strings"

	"spearhook/config"
	"spearhook/models"
	"spearhook/utils"
)

// ActiveAIConfig returns the enabled profile with decrypted API key for server use.
// Chat, page mirroring, and info gathering share this single active profile.
func ActiveAIConfig() (models.AIProfile, string, bool) {
	db := config.GetDB()
	var profile models.AIProfile
	if err := db.Where("enabled = ?", true).Order("updated_at DESC").First(&profile).Error; err != nil {
		return models.AIProfile{}, "", false
	}
	plain := ""
	if strings.TrimSpace(profile.APIKey) != "" {
		if dec, err := utils.Decrypt(profile.APIKey); err == nil {
			plain = dec
		} else {
			plain = profile.APIKey
		}
	}
	return profile, plain, true
}
