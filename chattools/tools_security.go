package chattools

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"spearhook/config"
	"spearhook/models"
	"spearhook/platform"
	"spearhook/utils"

	"gorm.io/gorm"
)

func init() {
	Register(newTool(
		"list_credentials",
		"List captured credential metadata (username, IP, location) for projects the user can access. Never returns passwords.",
		RiskRead,
		schema(map[string]any{
			"project_id": map[string]any{"type": "integer", "description": "Optional project id"},
			"limit":      map[string]any{"type": "integer", "description": "Max rows (default 50, max 200)"},
		}, nil),
		"",
		toolListCredentials,
	))
	Register(newTool(
		"add_ip_blacklist",
		"Block the source IP of a captured credential at the host firewall. The IP is derived from the credential.",
		RiskWrite,
		schema(map[string]any{
			"credential_id": map[string]any{"type": "integer", "description": "Credential id whose IP should be blocked"},
			"ip":            map[string]any{"type": "string", "description": "Optional; must match the credential IP"},
			"reason":        map[string]any{"type": "string"},
		}, []string{"credential_id"}),
		"",
		toolAddIPBlacklist,
	))
	Register(newTool(
		"export_credentials",
		"Export captured credentials (including passwords) as CSV. Requires confirmation; the CSV is returned once and is NOT stored in chat history.",
		RiskDestructive,
		schema(map[string]any{
			"project_id": map[string]any{"type": "integer", "description": "Optional project id"},
			"limit":      map[string]any{"type": "integer", "description": "Max rows (default 500, max 5000)"},
		}, nil),
		"This exports credential passwords as CSV. Approve?",
		toolExportCredentials,
	))
	Register(newTool(
		"list_ip_blacklist",
		"List IP blacklist entries with reason and enabled state.",
		RiskRead,
		schema(map[string]any{
			"limit": map[string]any{"type": "integer", "description": "Max rows (default 100, max 500)"},
		}, nil),
		"",
		toolListIPBlacklist,
	))
}

func credentialQueryForUser(db *gorm.DB, user models.User) *gorm.DB {
	q := db.Model(&models.Credential{})
	if user.Role.IsAdmin() {
		return q
	}
	return q.Where(
		"project_id IN (?)",
		db.Model(&models.Project{}).Select("id").Where("created_by = ?", user.ID),
	)
}

func toolListCredentials(ec ExecContext, args map[string]any) Result {
	limit := 50
	if n, ok := jsonInt(args["limit"]); ok && n > 0 {
		limit = int(n)
	}
	if limit > 200 {
		limit = 200
	}
	db := config.GetDB()
	var rows []models.Credential
	q := credentialQueryForUser(db, ec.User)
	if projectID := jsonUintPtr(args["project_id"]); projectID != nil {
		var project models.Project
		if err := platform.ScopeByOwner(db, ec.User).First(&project, *projectID).Error; err != nil {
			return Result{OK: false, Error: "project not found"}
		}
		q = q.Where("project_id = ?", project.ID)
	}
	if err := q.Order("id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return Result{OK: false, Error: "failed to list credentials"}
	}
	out := make([]map[string]any, 0, len(rows))
	for _, c := range rows {
		out = append(out, map[string]any{
			"id":          c.ID,
			"project_id":  c.ProjectID,
			"username":    c.Username,
			"ip_address":  c.IPAddress,
			"ip_location": c.IPLocation,
			"campaign_id": c.CampaignID,
			"created_at":  c.CreatedAt,
		})
	}
	return Result{OK: true, Summary: fmt.Sprintf("%d credentials", len(out)), Result: out}
}

func toolAddIPBlacklist(ec ExecContext, args map[string]any) Result {
	credentialID := jsonUintPtr(args["credential_id"])
	if credentialID == nil {
		return Result{OK: false, Error: "ip blacklist entries can only be created from captured credential IPs"}
	}
	db := config.GetDB()
	var credential models.Credential
	if err := credentialQueryForUser(db, ec.User).Where("id = ?", *credentialID).First(&credential).Error; err != nil {
		return Result{OK: false, Error: "credential not found or not accessible"}
	}
	credentialIP, err := normalizeBlacklistIP(credential.IPAddress)
	if err != nil {
		return Result{OK: false, Error: "credential has no valid IP address"}
	}
	if requested := jsonString(args["ip"]); requested != "" {
		requestedIP, err := normalizeBlacklistIP(requested)
		if err != nil {
			return Result{OK: false, Error: err.Error()}
		}
		if requestedIP != credentialIP {
			return Result{OK: false, Error: "IP must match the credential source IP"}
		}
	}

	reason := jsonString(args["reason"])
	if reason == "" {
		reason = "Blocked from captured credential"
	}
	entry := models.IPBlacklist{
		IPAddress: credentialIP,
		Reason:    reason,
		Enabled:   true,
		CreatedBy: ec.User.ID,
	}
	if err := db.Create(&entry).Error; err != nil {
		return Result{OK: false, Error: "Failed to create IP blacklist entry"}
	}
	if err := utils.ApplyHostIPBlock(entry.IPAddress); err != nil {
		_ = db.Delete(&entry).Error
		return Result{OK: false, Error: "Failed to apply host firewall rule: " + err.Error()}
	}
	return Result{
		OK:      true,
		Summary: fmt.Sprintf("blocked %s (entry #%d)", entry.IPAddress, entry.ID),
		Result: map[string]any{
			"id":            entry.ID,
			"ip_address":    entry.IPAddress,
			"reason":        entry.Reason,
			"credential_id": credential.ID,
		},
		Links: []Link{{Label: "IP 黑名单", Href: "/workbench/blacklist"}},
	}
}

func normalizeBlacklistIP(value string) (string, error) {
	candidate := strings.TrimSpace(value)
	if candidate == "" {
		return "", errors.New("IP address is required")
	}
	if strings.Contains(candidate, "/") {
		prefix, err := netip.ParsePrefix(candidate)
		if err != nil {
			return "", errors.New("Invalid IP or CIDR")
		}
		return prefix.Masked().String(), nil
	}
	addr, err := netip.ParseAddr(candidate)
	if err != nil {
		return "", errors.New("Invalid IP or CIDR")
	}
	return addr.String(), nil
}

func toolExportCredentials(ec ExecContext, args map[string]any) Result {
	limit := 500
	if n, ok := jsonInt(args["limit"]); ok && n > 0 {
		limit = int(n)
	}
	if limit > 5000 {
		limit = 5000
	}
	db := config.GetDB()
	q := credentialQueryForUser(db, ec.User)
	if projectID := jsonUintPtr(args["project_id"]); projectID != nil {
		var project models.Project
		if err := platform.ScopeByOwner(db, ec.User).First(&project, *projectID).Error; err != nil {
			return Result{OK: false, Error: "project not found"}
		}
		q = q.Where("project_id = ?", project.ID)
	}
	var rows []models.Credential
	if err := q.Order("id ASC").Limit(limit).Find(&rows).Error; err != nil {
		return Result{OK: false, Error: "failed to list credentials"}
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"id", "project_id", "username", "password", "ip_address", "ip_location", "captcha_value", "created_at"})
	for _, c := range rows {
		_ = w.Write([]string{
			fmt.Sprint(c.ID),
			fmt.Sprint(c.ProjectID),
			c.Username,
			c.Password,
			c.IPAddress,
			c.IPLocation,
			c.CaptchaValue,
			c.CreatedAt.Format(time.RFC3339),
		})
	}
	w.Flush()

	return Result{
		OK:      true,
		Summary: fmt.Sprintf("exported %d credentials", len(rows)),
		Result: map[string]any{
			"count":    len(rows),
			"filename": "credentials.csv",
		},
		// Ephemeral so the CSV (with passwords) is streamed once and never
		// persisted into chat history.
		Ephemeral: map[string]any{
			"csv":      buf.String(),
			"filename": "credentials.csv",
		},
	}
}

func toolListIPBlacklist(ec ExecContext, args map[string]any) Result {
	limit := 100
	if n, ok := jsonInt(args["limit"]); ok && n > 0 {
		limit = int(n)
	}
	if limit > 500 {
		limit = 500
	}
	db := config.GetDB()
	var rows []models.IPBlacklist
	if err := platform.ScopeByOwner(db.Model(&models.IPBlacklist{}), ec.User).
		Order("id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return Result{OK: false, Error: "failed to list IP blacklist"}
	}
	out := make([]map[string]any, 0, len(rows))
	for _, entry := range rows {
		out = append(out, map[string]any{
			"id":         entry.ID,
			"ip_address": entry.IPAddress,
			"reason":     entry.Reason,
			"enabled":    entry.Enabled,
			"created_at": entry.CreatedAt,
		})
	}
	return Result{
		OK:      true,
		Summary: fmt.Sprintf("%d blacklist entries", len(out)),
		Result:  out,
		Links:   []Link{{Label: "IP 黑名单", Href: "/ip-blacklist"}},
	}
}
