package platform

import (
	"errors"
	"sort"
	"strings"

	"spearhook/config"
	"spearhook/models"
	"spearhook/utils"

	"gorm.io/gorm"
)

// EnsureSearchSetting returns the legacy singleton search config, seeding it
// from the YAML search section on first use. It is retained for migration into
// SearchProfile and for fresh installs that still start with the YAML section.
func EnsureSearchSetting() (*models.SearchSetting, error) {
	db := config.GetDB()
	var row models.SearchSetting
	err := db.First(&row, 1).Error
	if err == nil {
		return &row, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	provider := utils.NormalizeSearchProvider(config.SearchProvider())
	if !utils.IsKnownSearchProvider(provider) {
		provider = utils.SearchProviderAuto
	}
	baseURL := strings.TrimRight(strings.TrimSpace(config.SearchBaseURL()), "/")
	if baseURL == "" {
		baseURL = utils.DefaultSearchBaseURL(provider)
	}
	encryptedKey := ""
	if key := config.SearchAPIKey(); key != "" {
		if enc, eerr := utils.Encrypt(key); eerr == nil {
			encryptedKey = enc
		}
	}
	row = models.SearchSetting{
		ID:       1,
		Provider: provider,
		BaseURL:  baseURL,
		APIKey:   encryptedKey,
		Count:    utils.SearchCountOrDefault(config.SearchCount()),
	}
	if err := db.Create(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// EnsureSearchProfiles returns all search profiles. On a database that only
// has the legacy singleton row, it migrates that row into the first profile so
// existing provider/key settings are preserved.
func EnsureSearchProfiles() ([]models.SearchProfile, error) {
	db := config.GetDB()
	var rows []models.SearchProfile
	if err := db.Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) > 0 {
		return rows, nil
	}

	legacy, err := EnsureSearchSetting()
	if err != nil {
		return nil, err
	}
	provider := utils.NormalizeSearchProvider(legacy.Provider)
	if !utils.IsKnownSearchProvider(provider) {
		provider = utils.SearchProviderAuto
	}
	baseURL := strings.TrimRight(strings.TrimSpace(legacy.BaseURL), "/")
	if provider == utils.SearchProviderAuto {
		baseURL = ""
	} else if baseURL == "" {
		baseURL = utils.DefaultSearchBaseURL(provider)
	}
	row := models.SearchProfile{
		ID:       "default",
		Name:     "Default",
		Enabled:  true,
		Provider: provider,
		BaseURL:  baseURL,
		APIKey:   legacy.APIKey,
		Count:    utils.SearchCountOrDefault(legacy.Count),
	}
	if err := db.Create(&row).Error; err != nil {
		return nil, err
	}
	return []models.SearchProfile{row}, nil
}

// SearchConfigs resolves every enabled search profile with decrypted keys.
// Standalone providers are ordered before "auto"; callers query standalone
// providers in parallel and use "auto" as the native fallback. If nothing is
// enabled, it returns a single "auto" config so info gathering keeps its
// historical behavior.
func SearchConfigs() []utils.SearchProviderConfig {
	rows, err := EnsureSearchProfiles()
	if err != nil || len(rows) == 0 {
		return []utils.SearchProviderConfig{{
			Provider: utils.SearchProviderAuto,
			Count:    utils.SearchCountOrDefault(0),
		}}
	}

	out := make([]utils.SearchProviderConfig, 0, len(rows))
	for _, row := range rows {
		if !row.Enabled {
			continue
		}
		provider := utils.NormalizeSearchProvider(row.Provider)
		if !utils.IsKnownSearchProvider(provider) {
			provider = utils.SearchProviderAuto
		}
		baseURL := strings.TrimRight(strings.TrimSpace(row.BaseURL), "/")
		if provider == utils.SearchProviderAuto {
			baseURL = ""
		} else if baseURL == "" {
			baseURL = utils.DefaultSearchBaseURL(provider)
		}
		key := ""
		if strings.TrimSpace(row.APIKey) != "" {
			if dec, derr := utils.Decrypt(row.APIKey); derr == nil && dec != "" {
				key = dec
			} else {
				key = row.APIKey
			}
		}
		out = append(out, utils.SearchProviderConfig{
			Provider: provider,
			BaseURL:  baseURL,
			APIKey:   key,
			Count:    utils.SearchCountOrDefault(row.Count),
		})
	}
	if len(out) == 0 {
		return []utils.SearchProviderConfig{{
			Provider: utils.SearchProviderAuto,
			Count:    utils.SearchCountOrDefault(0),
		}}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Provider == utils.SearchProviderAuto {
			return false
		}
		if out[j].Provider == utils.SearchProviderAuto {
			return true
		}
		return false
	})
	return out
}
