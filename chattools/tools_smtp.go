package chattools

import (
	"fmt"

	"spearhook/config"
	"spearhook/models"
	"spearhook/platform"
)

func init() {
	Register(newTool(
		"list_smtp",
		"List SMTP services (id, name, host, enabled) — read only.",
		RiskRead,
		schema(nil, nil),
		"",
		toolListSMTP,
	))
}

func toolListSMTP(ec ExecContext, _ map[string]any) Result {
	db := config.GetDB()
	var rows []models.SmtpService
	if err := platform.ScopeByOwner(db.Model(&models.SmtpService{}), ec.User).Order("id DESC").Find(&rows).Error; err != nil {
		return Result{OK: false, Error: "failed to list smtp services"}
	}
	out := make([]map[string]any, 0, len(rows))
	for _, s := range rows {
		out = append(out, map[string]any{
			"id":      s.ID,
			"name":    s.Name,
			"host":    s.Host,
			"port":    s.Port,
			"enabled": s.Enabled,
		})
	}
	return Result{OK: true, Summary: fmt.Sprintf("%d smtp services", len(out)), Result: out}
}
