package chattools

import (
	"fmt"
	"strings"
	"time"

	"spearhook/config"
	"spearhook/models"
	"spearhook/platform"
	"spearhook/utils"
)

func init() {
	Register(newTool(
		"create_qr_relay",
		"Create a live QR relay channel. Returns the slug, public image URL, and a one-time upload token.",
		RiskWrite,
		schema(map[string]any{
			"name": map[string]any{"type": "string", "description": "Human-readable relay name"},
			"slug": map[string]any{"type": "string", "description": "Optional custom slug"},
		}, []string{"name"}),
		"",
		toolCreateQrRelay,
	))
	Register(newTool(
		"get_qr_relay",
		"Get a QR relay channel's health and public URL by id or slug.",
		RiskRead,
		schema(map[string]any{
			"relay_id": map[string]any{"type": "integer", "description": "Relay id"},
			"slug":     map[string]any{"type": "string", "description": "Relay slug"},
		}, nil),
		"",
		toolGetQrRelay,
	))
	Register(newTool(
		"list_qr_relays",
		"List QR relay channels with health, public URL, and upload counts.",
		RiskRead,
		schema(map[string]any{
			"limit": map[string]any{"type": "integer", "description": "Max rows (default 50, max 200)"},
		}, nil),
		"",
		toolListQrRelays,
	))
}

func toolCreateQrRelay(ec ExecContext, args map[string]any) Result {
	name := jsonString(args["name"])
	if name == "" {
		return Result{OK: false, Error: "name is required"}
	}
	slug, err := utils.NormalizeQRRelaySlug(jsonString(args["slug"]))
	if err != nil {
		return Result{OK: false, Error: err.Error()}
	}
	plain, hash, err := utils.NewQRRelayToken()
	if err != nil {
		return Result{OK: false, Error: "Failed to generate upload token"}
	}
	row := models.QrRelay{
		Name:      name,
		Slug:      slug,
		TokenHash: hash,
		Enabled:   true,
		CreatedBy: ec.User.ID,
	}
	if err := config.GetDB().Create(&row).Error; err != nil {
		if strings.Contains(strings.ToUpper(err.Error()), "UNIQUE") {
			return Result{OK: false, Error: "slug already exists"}
		}
		return Result{OK: false, Error: "Failed to create QR relay"}
	}
	publicURL := qrRelayPublicURL(row.Slug)
	return Result{
		OK:      true,
		Summary: fmt.Sprintf("created QR relay #%d (%s)", row.ID, row.Slug),
		Result: map[string]any{
			"id":         row.ID,
			"name":       row.Name,
			"slug":       row.Slug,
			"public_url": publicURL,
		},
		Ephemeral: map[string]any{
			"upload_token": plain,
		},
		Links: []Link{{Label: "QR 中继", Href: "/workbench/qr-relays"}},
	}
}

func toolGetQrRelay(ec ExecContext, args map[string]any) Result {
	db := config.GetDB()
	q := platform.ScopeByOwner(db.Model(&models.QrRelay{}), ec.User)
	if id := jsonUintPtr(args["relay_id"]); id != nil {
		q = q.Where("id = ?", *id)
	} else if slug := jsonString(args["slug"]); slug != "" {
		q = q.Where("slug = ?", slug)
	} else {
		return Result{OK: false, Error: "relay_id or slug is required"}
	}
	var row models.QrRelay
	if err := q.First(&row).Error; err != nil {
		return Result{OK: false, Error: "relay not found"}
	}
	return Result{
		OK:      true,
		Summary: fmt.Sprintf("relay #%d %s (%s)", row.ID, row.Name, qrRelayHealth(row)),
		Result: map[string]any{
			"id":             row.ID,
			"name":           row.Name,
			"slug":           row.Slug,
			"enabled":        row.Enabled,
			"health":         qrRelayHealth(row),
			"public_url":     qrRelayPublicURL(row.Slug),
			"has_image":      strings.TrimSpace(row.ImagePath) != "",
			"upload_count":   row.UploadCount,
			"last_upload_at": row.LastUploadAt,
			"last_seen_at":   row.LastSeenAt,
		},
		Links: []Link{{Label: "QR 中继", Href: "/workbench/qr-relays"}},
	}
}

func qrRelayPublicURL(slug string) string {
	path := utils.QRRelayPublicPath(slug)
	if base := platform.EffectivePublicBaseURL(); base != "" {
		return strings.TrimRight(base, "/") + path
	}
	return path
}

func qrRelayHealth(row models.QrRelay) string {
	if !row.Enabled {
		return "paused"
	}
	seen := row.LastSeenAt
	if seen == nil {
		seen = row.LastUploadAt
	}
	if seen == nil {
		return "empty"
	}
	if time.Since(*seen) <= 10*time.Second {
		return "online"
	}
	if time.Since(*seen) > 60*time.Second {
		return "stale"
	}
	return "online"
}

func toolListQrRelays(ec ExecContext, args map[string]any) Result {
	limit := 50
	if n, ok := jsonInt(args["limit"]); ok && n > 0 {
		limit = int(n)
	}
	if limit > 200 {
		limit = 200
	}
	db := config.GetDB()
	var rows []models.QrRelay
	if err := platform.ScopeByOwner(db.Model(&models.QrRelay{}), ec.User).
		Order("id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return Result{OK: false, Error: "failed to list QR relays"}
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{
			"id":             row.ID,
			"name":           row.Name,
			"slug":           row.Slug,
			"enabled":        row.Enabled,
			"health":         qrRelayHealth(row),
			"public_url":     qrRelayPublicURL(row.Slug),
			"upload_count":   row.UploadCount,
			"last_upload_at": row.LastUploadAt,
			"last_seen_at":   row.LastSeenAt,
		})
	}
	return Result{
		OK:      true,
		Summary: fmt.Sprintf("%d QR relays", len(out)),
		Result:  out,
		Links:   []Link{{Label: "QR 中继", Href: "/workbench/qr-relays"}},
	}
}
