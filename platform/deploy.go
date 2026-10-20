package platform

import (
	"encoding/json"
	"errors"
	"fmt"

	"spearhook/models"

	"gorm.io/gorm"
)

const (
	// AgentProjectPortStart and AgentProjectPortEnd bound controller-assigned ports.
	AgentProjectPortStart uint = 20000
	AgentProjectPortEnd   uint = 30000
)

// EnqueueAgentTask creates one idempotent task for an agent deployment.
func EnqueueAgentTask(tx *gorm.DB, deployment models.ProjectDeployment, taskType string) error {
	taskID, err := GenerateURLToken("task_", 16)
	if err != nil {
		return err
	}
	var project models.Project
	if err := tx.First(&project, deployment.ProjectID).Error; err != nil {
		return err
	}
	if taskType == "project.deploy" && deployment.RuntimePort == 0 {
		port, err := AllocateAgentProjectPort(tx, deployment.AgentID)
		if err != nil {
			return err
		}
		deployment.RuntimePort = port
		if err := tx.Model(&models.ProjectDeployment{}).Where("id = ?", deployment.ID).
			Update("runtime_port", port).Error; err != nil {
			return err
		}
	}
	payload, err := json.Marshal(map[string]any{
		"project_id":        deployment.ProjectID,
		"deployment_id":     deployment.ID,
		"revision":          deployment.DesiredRevision,
		"desired_status":    deployment.DesiredStatus,
		"project_name":      project.Name,
		"frontend_route":    project.FrontendRoute,
		"container_route":   project.ContainerRoute,
		"runtime_port":      deployment.RuntimePort,
		"artifact_endpoint": fmt.Sprintf("/api/agent/deployments/%d/artifact/", deployment.ID),
	})
	if err != nil {
		return err
	}
	return tx.Create(&models.AgentTask{
		TaskID:       taskID,
		AgentID:      deployment.AgentID,
		ProjectID:    deployment.ProjectID,
		DeploymentID: deployment.ID,
		Type:         taskType,
		Payload:      string(payload),
		Status:       models.AgentTaskStatusPending,
	}).Error
}

// AllocateAgentProjectPort picks the lowest free runtime port for an agent.
func AllocateAgentProjectPort(tx *gorm.DB, agentID uint) (uint, error) {
	var used []uint
	if err := tx.Model(&models.ProjectDeployment{}).
		Where("agent_id = ? AND desired_status <> ? AND runtime_port BETWEEN ? AND ?", agentID, models.DeploymentStatusRemoved, AgentProjectPortStart, AgentProjectPortEnd).
		Pluck("runtime_port", &used).Error; err != nil {
		return 0, err
	}
	taken := make(map[uint]struct{}, len(used))
	for _, port := range used {
		taken[port] = struct{}{}
	}
	for port := AgentProjectPortStart; port <= AgentProjectPortEnd; port++ {
		if _, exists := taken[port]; !exists {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no project port available for agent %d in range %d-%d", agentID, AgentProjectPortStart, AgentProjectPortEnd)
}

// SyncProjectDeployments reconciles the selected agents and queues idempotent tasks.
func SyncProjectDeployments(tx *gorm.DB, project *models.Project, agentIDs []uint) error {
	selected := make(map[uint]struct{}, len(agentIDs))
	for _, id := range agentIDs {
		if id == 0 {
			continue
		}
		selected[id] = struct{}{}
	}
	if len(selected) > 0 {
		var count int64
		ids := make([]uint, 0, len(selected))
		for id := range selected {
			ids = append(ids, id)
		}
		if err := tx.Model(&models.Agent{}).Where("id IN ?", ids).Count(&count).Error; err != nil {
			return err
		}
		if count != int64(len(ids)) {
			return errors.New("one or more selected agents do not exist")
		}
	}

	var existing []models.ProjectDeployment
	if err := tx.Where("project_id = ?", project.ID).Find(&existing).Error; err != nil {
		return err
	}
	byAgent := make(map[uint]models.ProjectDeployment, len(existing))
	for _, deployment := range existing {
		byAgent[deployment.AgentID] = deployment
	}

	for agentID := range selected {
		deployment, exists := byAgent[agentID]
		if !exists {
			deployment = models.ProjectDeployment{
				ProjectID:       project.ID,
				AgentID:         agentID,
				DesiredStatus:   models.DeploymentStatusRunning,
				ActualStatus:    models.DeploymentStatusPending,
				DesiredRevision: project.DeploymentRevision,
			}
			if err := tx.Create(&deployment).Error; err != nil {
				return err
			}
		} else {
			if deployment.DesiredStatus == models.DeploymentStatusRemoved {
				deployment.RuntimePort = 0
				deployment.RuntimeURL = ""
			}
			deployment.DesiredStatus = models.DeploymentStatusRunning
			deployment.DesiredRevision = project.DeploymentRevision
			deployment.ActualStatus = models.DeploymentStatusPending
			deployment.LastError = ""
			if err := tx.Save(&deployment).Error; err != nil {
				return err
			}
		}
		if err := EnqueueAgentTask(tx, deployment, "project.deploy"); err != nil {
			return err
		}
	}

	for _, deployment := range existing {
		if _, keep := selected[deployment.AgentID]; keep || deployment.DesiredStatus == models.DeploymentStatusRemoved {
			continue
		}
		deployment.DesiredStatus = models.DeploymentStatusRemoved
		if err := tx.Save(&deployment).Error; err != nil {
			return err
		}
		if err := EnqueueAgentTask(tx, deployment, "project.delete"); err != nil {
			return err
		}
	}
	return nil
}
