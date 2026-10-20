package chattools

import (
	"fmt"
	"time"

	"spearhook/config"
	"spearhook/models"
	"spearhook/platform"
)

func init() {
	Register(newTool(
		"list_agents",
		"List registered Agents with online/offline status and deployment counts.",
		RiskRead,
		schema(nil, nil),
		"",
		toolListAgents,
	))
}

func toolListAgents(_ ExecContext, _ map[string]any) Result {
	db := config.GetDB()
	var agents []models.Agent
	if err := db.Order("last_seen_at DESC, id DESC").Find(&agents).Error; err != nil {
		return Result{OK: false, Error: "failed to list agents"}
	}
	now := time.Now()
	out := make([]map[string]any, 0, len(agents))
	online := 0
	for _, agent := range agents {
		status := platform.CurrentAgentStatus(agent, now)
		if status == models.AgentStatusOnline {
			online++
		}
		var depCount int64
		db.Model(&models.ProjectDeployment{}).
			Where("agent_id = ? AND desired_status <> ?", agent.ID, models.DeploymentStatusRemoved).
			Count(&depCount)
		out = append(out, map[string]any{
			"id":               agent.ID,
			"agent_id":         agent.AgentID,
			"name":             agent.Name,
			"hostname":         agent.Hostname,
			"ip_address":       agent.IPAddress,
			"status":           status,
			"deployment_count": depCount,
			"last_seen_at":     agent.LastSeenAt,
		})
	}
	return Result{
		OK:      true,
		Summary: fmt.Sprintf("%d agents (%d online)", len(out), online),
		Result:  out,
	}
}
