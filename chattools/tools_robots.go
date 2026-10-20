package chattools

import (
	"fmt"

	"spearhook/config"
	"spearhook/models"
	"spearhook/platform"
)

func init() {
	Register(newTool(
		"list_robots",
		"List webhook robots (id, name, type, status) — read only.",
		RiskRead,
		schema(nil, nil),
		"",
		toolListRobots,
	))
	Register(newTool(
		"bind_robot",
		"Bind a webhook robot to a project so project events are pushed to it.",
		RiskWrite,
		schema(map[string]any{
			"project_id": map[string]any{"type": "integer", "description": "Project ID"},
			"robot_id":   map[string]any{"type": "integer", "description": "Robot ID (from list_robots)"},
		}, []string{"project_id", "robot_id"}),
		"",
		toolBindRobot,
	))
	Register(newTool(
		"list_project_hooks",
		"List webhook robots bound to a project.",
		RiskRead,
		schema(map[string]any{
			"project_id": map[string]any{"type": "integer", "description": "Project ID"},
		}, []string{"project_id"}),
		"",
		toolListProjectHooks,
	))
}

func toolListRobots(ec ExecContext, _ map[string]any) Result {
	db := config.GetDB()
	var rows []models.Robot
	if err := platform.ScopeByOwner(db.Model(&models.Robot{}), ec.User).Order("id DESC").Find(&rows).Error; err != nil {
		return Result{OK: false, Error: "failed to list robots"}
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"id":         r.ID,
			"name":       r.Name,
			"status":     r.Status,
			"robot_type": r.RobotType,
		})
	}
	return Result{OK: true, Summary: fmt.Sprintf("%d robots", len(out)), Result: out}
}

func toolBindRobot(ec ExecContext, args map[string]any) Result {
	projectID := jsonUintPtr(args["project_id"])
	robotID := jsonUintPtr(args["robot_id"])
	if projectID == nil {
		return Result{OK: false, Error: "project_id is required"}
	}
	if robotID == nil {
		return Result{OK: false, Error: "robot_id is required"}
	}
	db := config.GetDB()
	var project models.Project
	if err := platform.ScopeByOwner(db, ec.User).First(&project, *projectID).Error; err != nil {
		return Result{OK: false, Error: "project not found"}
	}
	var robot models.Robot
	if err := platform.ScopeByOwner(db, ec.User).First(&robot, *robotID).Error; err != nil {
		return Result{OK: false, Error: "robot not found"}
	}

	var bound []models.Robot
	if err := db.Model(&project).Association("Robots").Find(&bound); err != nil {
		return Result{OK: false, Error: "failed to load project robots"}
	}
	for _, existing := range bound {
		if existing.ID == robot.ID {
			return Result{
				OK:      true,
				Summary: fmt.Sprintf("robot #%d already bound to project #%d", robot.ID, project.ID),
				Result:  map[string]any{"project_id": project.ID, "robot_id": robot.ID, "already_bound": true},
				Links:   []Link{{Label: "打开项目", Href: fmt.Sprintf("/projects/%d", project.ID)}},
			}
		}
	}
	if err := db.Model(&project).Association("Robots").Append(&robot); err != nil {
		return Result{OK: false, Error: "failed to bind robot: " + err.Error()}
	}
	return Result{
		OK:      true,
		Summary: fmt.Sprintf("bound robot #%d (%s) to project #%d", robot.ID, robot.Name, project.ID),
		Result: map[string]any{
			"project_id": project.ID,
			"robot_id":   robot.ID,
			"robot_name": robot.Name,
		},
		Links: []Link{{Label: "打开项目", Href: fmt.Sprintf("/projects/%d", project.ID)}},
	}
}

func toolListProjectHooks(ec ExecContext, args map[string]any) Result {
	projectID := jsonUintPtr(args["project_id"])
	if projectID == nil {
		return Result{OK: false, Error: "project_id is required"}
	}
	db := config.GetDB()
	var project models.Project
	if err := platform.ScopeByOwner(db, ec.User).Preload("Robots").First(&project, *projectID).Error; err != nil {
		return Result{OK: false, Error: "project not found"}
	}
	out := make([]map[string]any, 0, len(project.Robots))
	for _, r := range project.Robots {
		out = append(out, map[string]any{
			"id":         r.ID,
			"name":       r.Name,
			"robot_type": r.RobotType,
			"status":     r.Status,
		})
	}
	return Result{
		OK:      true,
		Summary: fmt.Sprintf("%d hooks on project #%d", len(out), project.ID),
		Result:  out,
		Links:   []Link{{Label: "打开项目", Href: fmt.Sprintf("/projects/%d", project.ID)}},
	}
}
