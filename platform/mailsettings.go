package platform

import (
	"errors"
	"fmt"
	"strings"

	"spearhook/config"
	"spearhook/models"
	"spearhook/utils"

	"gorm.io/gorm"
)

// EnsureMailTrackingSetting returns the singleton settings row, seeding it from
// YAML configuration on first use.
func EnsureMailTrackingSetting() (*models.MailTrackingSetting, error) {
	db := config.GetDB()
	var row models.MailTrackingSetting
	err := db.First(&row, 1).Error
	if err == nil {
		return &row, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	enabled := config.MailTrackingEnabled()
	secretPlain := strings.TrimSpace(config.MailTrackingSecret())
	enc := ""
	if secretPlain != "" {
		if e, eerr := utils.Encrypt(secretPlain); eerr == nil {
			enc = e
		}
	}
	hosts := config.MailTrackingAllowRedirectHosts()
	row = models.MailTrackingSetting{
		ID:                 1,
		PublicBaseURL:      config.PublicBaseURL(),
		Enabled:            enabled,
		Secret:             enc,
		AllowRedirectHosts: strings.Join(hosts, "\n"),
		ClickPath:          "",
		OpenPath:           "",
	}
	if err := db.Create(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// LoadMailTrackingSetting returns the settings row or nil on error.
func LoadMailTrackingSetting() *models.MailTrackingSetting {
	row, err := EnsureMailTrackingSetting()
	if err != nil {
		return nil
	}
	return row
}

// DecryptMailTrackingSecret decrypts an at-rest secret, falling back to plaintext.
func DecryptMailTrackingSecret(enc string) string {
	enc = strings.TrimSpace(enc)
	if enc == "" {
		return ""
	}
	if dec, err := utils.Decrypt(enc); err == nil && dec != "" {
		return dec
	}
	return enc
}

// SplitHostLines parses newline/comma/semicolon separated host lists.
func SplitHostLines(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ',' || r == ';'
	})
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, p := range parts {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// EffectivePublicBaseURL prefers DB setting, then YAML, then empty.
func EffectivePublicBaseURL() string {
	if row := LoadMailTrackingSetting(); row != nil {
		if base := strings.TrimRight(strings.TrimSpace(row.PublicBaseURL), "/"); base != "" {
			return base
		}
	}
	return config.PublicBaseURL()
}

// EffectiveMailTrackingEnabled prefers DB.
func EffectiveMailTrackingEnabled() bool {
	if row := LoadMailTrackingSetting(); row != nil {
		return row.Enabled
	}
	return config.MailTrackingEnabled()
}

// EffectiveMailTrackingSecret prefers DB encrypted secret, then YAML/jwt fallback.
func EffectiveMailTrackingSecret() string {
	if row := LoadMailTrackingSetting(); row != nil {
		if plain := DecryptMailTrackingSecret(row.Secret); plain != "" {
			return plain
		}
	}
	return config.MailTrackingSecret()
}

// EffectiveMailTrackingAllowRedirectHosts merges DB hosts with YAML hosts.
func EffectiveMailTrackingAllowRedirectHosts() []string {
	seen := map[string]bool{}
	out := make([]string, 0)
	add := func(list []string) {
		for _, h := range list {
			h = strings.ToLower(strings.TrimSpace(h))
			if h == "" || seen[h] {
				continue
			}
			seen[h] = true
			out = append(out, h)
		}
	}
	if row := LoadMailTrackingSetting(); row != nil {
		add(SplitHostLines(row.AllowRedirectHosts))
	}
	add(config.MailTrackingAllowRedirectHosts())
	return out
}

// ResolvePublicBaseURL returns an absolute origin for background (non-request)
// contexts: DB/YAML public base first, then the given scheme/host, then localhost.
func ResolvePublicBaseURL(scheme, host string) string {
	if base := EffectivePublicBaseURL(); base != "" {
		return base
	}
	scheme = strings.TrimSpace(scheme)
	if scheme == "" {
		scheme = "http"
	}
	if host = strings.TrimSpace(host); host != "" {
		return scheme + "://" + host
	}
	return fmt.Sprintf("http://127.0.0.1:%d", config.ServerPort())
}
