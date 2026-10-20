package chattools

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"spearhook/config"
	"spearhook/models"
	"spearhook/platform"
	"spearhook/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	_ = utils.InitEncryption("chattools-test-encryption-key")
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "chattools.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := models.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	config.DB = db
	return db
}

func mustCreate[T any](t *testing.T, db *gorm.DB, row *T) *T {
	t.Helper()
	if err := db.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	return row
}

func operatorExec(user models.User) ExecContext {
	return ExecContext{User: user}
}

func resultIDs(t *testing.T, res Result) map[uint]bool {
	t.Helper()
	if !res.OK {
		t.Fatalf("tool failed: %+v", res)
	}
	rows, ok := res.Result.([]map[string]any)
	if !ok {
		t.Fatalf("unexpected result type %T", res.Result)
	}
	ids := map[uint]bool{}
	for _, row := range rows {
		if id, ok := row["id"].(uint); ok {
			ids[id] = true
		}
	}
	return ids
}

func TestOperatorCannotSeeOtherProjects(t *testing.T) {
	db := newTestDB(t)
	owner := mustCreate(t, db, &models.User{Username: "owner", Password: "x", Role: models.UserRoleOperator})
	other := mustCreate(t, db, &models.User{Username: "other", Password: "x", Role: models.UserRoleOperator})
	admin := mustCreate(t, db, &models.User{Username: "admin", Password: "x", Role: models.UserRoleAdmin})

	ownProject := mustCreate(t, db, &models.Project{Name: "owner-project", CreatedBy: owner.ID})
	otherProject := mustCreate(t, db, &models.Project{Name: "other-project", CreatedBy: other.ID})

	res := Execute(operatorExec(*owner), "list_projects", "{}")
	ids := resultIDs(t, res)
	if !ids[ownProject.ID] || ids[otherProject.ID] {
		t.Fatalf("owner saw ids=%v", ids)
	}

	res = Execute(operatorExec(*other), "get_project", `{"project_id": 999}`)
	if res.OK {
		t.Fatal("missing project should fail")
	}
	res = Execute(operatorExec(*other), "get_project", `{"project_id": `+itoa(ownProject.ID)+`}`)
	if res.OK {
		t.Fatal("operator must not read another operator's project")
	}

	res = Execute(operatorExec(*admin), "list_projects", "{}")
	adminIDs := resultIDs(t, res)
	if !adminIDs[ownProject.ID] || !adminIDs[otherProject.ID] {
		t.Fatalf("admin should see all, got %v", adminIDs)
	}
}

func TestOperatorCannotSeeOtherPages(t *testing.T) {
	db := newTestDB(t)
	owner := mustCreate(t, db, &models.User{Username: "powner", Password: "x", Role: models.UserRoleOperator})
	other := mustCreate(t, db, &models.User{Username: "pother", Password: "x", Role: models.UserRoleOperator})

	ownPage := mustCreate(t, db, &models.PhishingPage{Name: "own-page", URL: "https://a", Html: "<html></html>", CreatedBy: owner.ID})
	otherPage := mustCreate(t, db, &models.PhishingPage{Name: "other-page", URL: "https://b", Html: "<html></html>", CreatedBy: other.ID})

	ids := resultIDs(t, Execute(operatorExec(*owner), "list_pages", "{}"))
	if !ids[ownPage.ID] || ids[otherPage.ID] {
		t.Fatalf("owner saw pages=%v", ids)
	}
	if res := Execute(operatorExec(*other), "get_page", `{"page_id": `+itoa(ownPage.ID)+`}`); res.OK {
		t.Fatal("operator must not read another operator's page")
	}
}

func TestOperatorCannotReadOtherCredentials(t *testing.T) {
	db := newTestDB(t)
	owner := mustCreate(t, db, &models.User{Username: "cowner", Password: "x", Role: models.UserRoleOperator})
	other := mustCreate(t, db, &models.User{Username: "cother", Password: "x", Role: models.UserRoleOperator})
	project := mustCreate(t, db, &models.Project{Name: "cred-project", CreatedBy: owner.ID})
	mustCreate(t, db, &models.Credential{ProjectID: project.ID, Username: "u", Password: "p", IPAddress: "203.0.113.9"})

	ids := resultIDs(t, Execute(operatorExec(*owner), "list_credentials", "{}"))
	if len(ids) != 1 {
		t.Fatalf("owner should see one credential, got %v", ids)
	}
	if res := Execute(operatorExec(*other), "list_credentials", "{}"); res.OK {
		if len(resultIDs(t, res)) != 0 {
			t.Fatal("other operator must not see foreign credentials")
		}
	}
	if res := Execute(operatorExec(*other), "list_credentials", `{"project_id": `+itoa(project.ID)+`}`); res.OK {
		t.Fatal("other operator must not query a foreign project")
	}
}

func TestWriteToolRecordsAudit(t *testing.T) {
	db := newTestDB(t)
	user := mustCreate(t, db, &models.User{Username: "auditor", Password: "x", Role: models.UserRoleOperator})
	project := mustCreate(t, db, &models.Project{Name: "audit-project", CreatedBy: user.ID})
	robot := mustCreate(t, db, &models.Robot{Name: "hook", Webhook: "https://hook.example", CreatedBy: user.ID})

	ec := ExecContext{User: *user, SessionID: 7, ToolCallID: "call_1"}
	res := Execute(ec, "bind_robot", `{"project_id": `+itoa(project.ID)+`, "robot_id": `+itoa(robot.ID)+`}`)
	if !res.OK {
		t.Fatalf("bind_robot failed: %+v", res)
	}

	var logs []models.AuditLog
	if err := db.Where("action = ?", "chat_tool_bind_robot").Find(&logs).Error; err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected one audit row, got %d", len(logs))
	}
	if logs[0].ActorID == nil || *logs[0].ActorID != user.ID {
		t.Fatalf("audit actor = %v", logs[0].ActorID)
	}
}

func TestInfoGatherJobScoped(t *testing.T) {
	db := newTestDB(t)
	owner := mustCreate(t, db, &models.User{Username: "gowner", Password: "x", Role: models.UserRoleOperator})
	other := mustCreate(t, db, &models.User{Username: "gother", Password: "x", Role: models.UserRoleOperator})
	job := mustCreate(t, db, &models.InfoGatherJob{Target: "example.com", CreatedBy: owner.ID})

	if res := Execute(operatorExec(*owner), "get_info_gather", `{"job_id": `+itoa(job.ID)+`}`); !res.OK {
		t.Fatalf("owner should read job: %+v", res)
	}
	if res := Execute(operatorExec(*other), "get_info_gather", `{"job_id": `+itoa(job.ID)+`}`); res.OK {
		t.Fatal("other operator must not read the job")
	}
}

func TestListProjectHooks(t *testing.T) {
	db := newTestDB(t)
	user := mustCreate(t, db, &models.User{Username: "hooks", Password: "x", Role: models.UserRoleOperator})
	project := mustCreate(t, db, &models.Project{Name: "hooks-project", CreatedBy: user.ID})
	robot := mustCreate(t, db, &models.Robot{Name: "hook", Webhook: "https://hook.example", CreatedBy: user.ID})
	if err := db.Model(project).Association("Robots").Append(robot); err != nil {
		t.Fatal(err)
	}

	res := Execute(operatorExec(*user), "list_project_hooks", `{"project_id": `+itoa(project.ID)+`}`)
	ids := resultIDs(t, res)
	if !ids[robot.ID] {
		t.Fatalf("hooks = %v", ids)
	}
}

func TestPreviewMail(t *testing.T) {
	db := newTestDB(t)
	user := mustCreate(t, db, &models.User{Username: "mailer", Password: "x", Role: models.UserRoleOperator})
	campaign := mustCreate(t, db, &models.MailCampaign{
		SmtpServiceID: 1,
		Subject:       "Hello",
		BodyTemplate:  "Hi {{email}}",
		CreatedBy:     user.ID,
	})
	mustCreate(t, db, &models.MailRecipient{CampaignID: campaign.ID, Email: "a@example.com"})

	res := Execute(operatorExec(*user), "preview_mail", `{"campaign_id": `+itoa(campaign.ID)+`}`)
	if !res.OK {
		t.Fatalf("preview failed: %+v", res)
	}
	m, _ := res.Result.(map[string]any)
	if m["subject"] != "Hello" || m["recipient_count"] != 1 {
		t.Fatalf("preview = %+v", m)
	}
	body, _ := m["body"].(string)
	if !strings.Contains(body, "a@example.com") {
		t.Fatalf("rendered body = %q", body)
	}
}

func TestExportCredentialsEphemeral(t *testing.T) {
	db := newTestDB(t)
	user := mustCreate(t, db, &models.User{Username: "exporter", Password: "x", Role: models.UserRoleOperator})
	project := mustCreate(t, db, &models.Project{Name: "export-project", CreatedBy: user.ID})
	mustCreate(t, db, &models.Credential{
		ProjectID: project.ID,
		Username:  "alice",
		Password:  "secret-pass",
		IPAddress: "203.0.113.5",
	})

	res := Execute(operatorExec(*user), "export_credentials", "{}")
	if !res.OK {
		t.Fatalf("export failed: %+v", res)
	}
	csvText, _ := res.Ephemeral["csv"].(string)
	if !strings.Contains(csvText, "secret-pass") || !strings.Contains(csvText, "alice") {
		t.Fatalf("csv = %q", csvText)
	}
	if m, ok := res.Result.(map[string]any); ok {
		if _, leaked := m["csv"]; leaked {
			t.Fatal("CSV must not be in the persisted result")
		}
	}
}

func TestProjectActionAutoBindsOnlineAgent(t *testing.T) {
	db := newTestDB(t)
	user := mustCreate(t, db, &models.User{Username: "deployer", Password: "x", Role: models.UserRoleOperator})
	project := mustCreate(t, db, &models.Project{Name: "auto-bind", CreatedBy: user.ID})
	now := time.Now()
	mustCreate(t, db, &models.Agent{AgentID: "a1", Name: "A1", TokenHash: "hash", LastSeenAt: &now})

	res := Execute(operatorExec(*user), "project_action", `{"project_id": `+itoa(project.ID)+`, "action":"deploy"}`)
	if !res.OK {
		t.Fatalf("auto-bind failed: %+v", res)
	}
	result, _ := res.Result.(map[string]any)
	if result["auto_bound"] != true {
		t.Fatalf("result = %+v", result)
	}

	var deployment models.ProjectDeployment
	if err := db.Where("project_id = ?", project.ID).First(&deployment).Error; err != nil {
		t.Fatalf("deployment not created: %v", err)
	}
	if deployment.DesiredStatus != models.DeploymentStatusRunning {
		t.Fatalf("desired_status = %q", deployment.DesiredStatus)
	}
	var tasks int64
	db.Model(&models.AgentTask{}).Where("project_id = ?", project.ID).Count(&tasks)
	if tasks == 0 {
		t.Fatal("expected a queued agent task")
	}
}

func TestProjectActionNoOnlineAgent(t *testing.T) {
	db := newTestDB(t)
	user := mustCreate(t, db, &models.User{Username: "noagent", Password: "x", Role: models.UserRoleOperator})
	project := mustCreate(t, db, &models.Project{Name: "no-agent", CreatedBy: user.ID})

	res := Execute(operatorExec(*user), "project_action", `{"project_id": `+itoa(project.ID)+`, "action":"deploy", "target":"agent"}`)
	if res.OK {
		t.Fatalf("expected failure: %+v", res)
	}
	if !strings.Contains(res.Error, "no online Agents") {
		t.Fatalf("error = %q", res.Error)
	}
}

func TestProjectActionDefaultsToLocalWithoutAgents(t *testing.T) {
	db := newTestDB(t)
	user := mustCreate(t, db, &models.User{Username: "localdefault", Password: "x", Role: models.UserRoleOperator})
	project := mustCreate(t, db, &models.Project{Name: "local-default", CreatedBy: user.ID, BuildStatus: models.BuildStatusPending})

	res := Execute(operatorExec(*user), "project_action", `{"project_id": `+itoa(project.ID)+`, "action":"deploy"}`)
	if res.OK {
		t.Fatalf("unbuilt docker project must fail: %+v", res)
	}
	if !strings.Contains(res.Error, "not built") {
		t.Fatalf("expected local docker build error, got %q", res.Error)
	}
	if strings.Contains(res.Error, "online Agents") {
		t.Fatalf("must not fall back to Agent: %q", res.Error)
	}
}

func TestProjectActionLocalDockerStopIdempotent(t *testing.T) {
	db := newTestDB(t)
	user := mustCreate(t, db, &models.User{Username: "dockerstop", Password: "x", Role: models.UserRoleOperator})
	project := mustCreate(t, db, &models.Project{
		Name:            "docker-stop",
		RunMode:         models.ProjectRunModeDocker,
		ContainerStatus: models.ContainerStatusStopped,
		CreatedBy:       user.ID,
	})
	res := Execute(operatorExec(*user), "project_action", `{"project_id": `+itoa(project.ID)+`, "action":"stop", "target":"local"}`)
	if !res.OK {
		t.Fatalf("stop failed: %+v", res)
	}
	result, _ := res.Result.(map[string]any)
	if result["already_stopped"] != true {
		t.Fatalf("result = %+v", result)
	}
}

func TestProjectActionLocalStopIdempotent(t *testing.T) {
	db := newTestDB(t)
	user := mustCreate(t, db, &models.User{Username: "localstop", Password: "x", Role: models.UserRoleOperator})
	project := mustCreate(t, db, &models.Project{
		Name:            "local-stop",
		RunMode:         models.ProjectRunModeLocal,
		ContainerStatus: models.ContainerStatusStopped,
		CreatedBy:       user.ID,
	})

	res := Execute(operatorExec(*user), "project_action", `{"project_id": `+itoa(project.ID)+`, "action":"stop"}`)
	if !res.OK {
		t.Fatalf("stop failed: %+v", res)
	}
	result, _ := res.Result.(map[string]any)
	if result["already_stopped"] != true {
		t.Fatalf("result = %+v", result)
	}
}

func TestProjectActionLocalStartMissingHTML(t *testing.T) {
	db := newTestDB(t)
	user := mustCreate(t, db, &models.User{Username: "localstart", Password: "x", Role: models.UserRoleOperator})
	project := mustCreate(t, db, &models.Project{
		Name:         "local-start",
		RunMode:      models.ProjectRunModeLocal,
		HtmlFilePath: filepath.Join(t.TempDir(), "missing.html"),
		CreatedBy:    user.ID,
	})

	res := Execute(operatorExec(*user), "project_action", `{"project_id": `+itoa(project.ID)+`, "action":"start"}`)
	if res.OK {
		t.Fatalf("expected failure: %+v", res)
	}
	if !strings.Contains(res.Error, "preparation failed") {
		t.Fatalf("error = %q", res.Error)
	}
}

func TestUpdateProjectSwitchesToDocker(t *testing.T) {
	db := newTestDB(t)
	user := mustCreate(t, db, &models.User{Username: "switcher", Password: "x", Role: models.UserRoleOperator})
	project := mustCreate(t, db, &models.Project{
		Name:            "old-name",
		RunMode:         models.ProjectRunModeLocal,
		ContainerStatus: models.ContainerStatusStopped,
		CreatedBy:       user.ID,
	})

	res := Execute(operatorExec(*user), "update_project", `{"project_id": `+itoa(project.ID)+`, "name":"new-name", "run_mode":"docker", "description":"updated"}`)
	if !res.OK {
		t.Fatalf("update failed: %+v", res)
	}
	var updated models.Project
	if err := db.First(&updated, project.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updated.Name != "new-name" || platform.NormalizedProjectRunMode(updated.RunMode) != models.ProjectRunModeDocker {
		t.Fatalf("updated = %+v", updated)
	}
	if updated.BuildStatus != models.BuildStatusPending {
		t.Fatalf("build_status = %q", updated.BuildStatus)
	}
	if updated.DeploymentRevision < 2 {
		t.Fatalf("deployment_revision = %d", updated.DeploymentRevision)
	}
}

func TestUpdateProjectBindsAgents(t *testing.T) {
	db := newTestDB(t)
	user := mustCreate(t, db, &models.User{Username: "binder", Password: "x", Role: models.UserRoleOperator})
	project := mustCreate(t, db, &models.Project{Name: "bind-agents", CreatedBy: user.ID})
	agent := mustCreate(t, db, &models.Agent{AgentID: "a1", Name: "A1", TokenHash: "hash"})

	res := Execute(operatorExec(*user), "update_project", `{"project_id": `+itoa(project.ID)+`, "agent_ids":[`+itoa(agent.ID)+`]}`)
	if !res.OK {
		t.Fatalf("update failed: %+v", res)
	}
	var deployments []models.ProjectDeployment
	if err := db.Where("project_id = ?", project.ID).Find(&deployments).Error; err != nil {
		t.Fatal(err)
	}
	if len(deployments) != 1 || deployments[0].AgentID != agent.ID {
		t.Fatalf("deployments = %+v", deployments)
	}
	var tasks int64
	db.Model(&models.AgentTask{}).Where("project_id = ?", project.ID).Count(&tasks)
	if tasks == 0 {
		t.Fatal("expected a queued agent task")
	}
}

func TestUpdateProjectRejectsLocalAgents(t *testing.T) {
	db := newTestDB(t)
	user := mustCreate(t, db, &models.User{Username: "localbind", Password: "x", Role: models.UserRoleOperator})
	project := mustCreate(t, db, &models.Project{
		Name:      "local-bind",
		RunMode:   models.ProjectRunModeLocal,
		CreatedBy: user.ID,
	})
	res := Execute(operatorExec(*user), "update_project", `{"project_id": `+itoa(project.ID)+`, "agent_ids":[1]}`)
	if res.OK || !strings.Contains(res.Error, "cannot bind Agents") {
		t.Fatalf("res = %+v", res)
	}
}

func TestUpdateProjectNoFields(t *testing.T) {
	db := newTestDB(t)
	user := mustCreate(t, db, &models.User{Username: "noop", Password: "x", Role: models.UserRoleOperator})
	project := mustCreate(t, db, &models.Project{Name: "noop-project", CreatedBy: user.ID})
	res := Execute(operatorExec(*user), "update_project", `{"project_id": `+itoa(project.ID)+`}`)
	if res.OK || !strings.Contains(res.Error, "no updatable fields") {
		t.Fatalf("res = %+v", res)
	}
}

func TestBuildProjectValidation(t *testing.T) {
	db := newTestDB(t)
	user := mustCreate(t, db, &models.User{Username: "builder", Password: "x", Role: models.UserRoleOperator})
	local := mustCreate(t, db, &models.Project{
		Name:      "build-local",
		RunMode:   models.ProjectRunModeLocal,
		CreatedBy: user.ID,
	})
	res := Execute(operatorExec(*user), "build_project", `{"project_id": `+itoa(local.ID)+`}`)
	if res.OK || !strings.Contains(res.Error, "do not need a Docker image") {
		t.Fatalf("local build res = %+v", res)
	}

	docker := mustCreate(t, db, &models.Project{
		Name:      "build-docker",
		RunMode:   models.ProjectRunModeDocker,
		CreatedBy: user.ID,
	})
	res = Execute(operatorExec(*user), "build_project", `{"project_id": `+itoa(docker.ID)+`}`)
	if res.OK || !strings.Contains(res.Error, "no HTML file") {
		t.Fatalf("docker build res = %+v", res)
	}
}

func TestListInfoGatherJobsScoped(t *testing.T) {
	db := newTestDB(t)
	owner := mustCreate(t, db, &models.User{Username: "gjob", Password: "x", Role: models.UserRoleOperator})
	other := mustCreate(t, db, &models.User{Username: "gjob2", Password: "x", Role: models.UserRoleOperator})
	own := mustCreate(t, db, &models.InfoGatherJob{Target: "own.example", CreatedBy: owner.ID})
	foreign := mustCreate(t, db, &models.InfoGatherJob{Target: "other.example", CreatedBy: other.ID})

	ids := resultIDs(t, Execute(operatorExec(*owner), "list_info_gather_jobs", "{}"))
	if !ids[own.ID] || ids[foreign.ID] {
		t.Fatalf("owner saw jobs=%v", ids)
	}
	ids = resultIDs(t, Execute(operatorExec(*other), "list_info_gather_jobs", "{}"))
	if !ids[foreign.ID] || ids[own.ID] {
		t.Fatalf("other saw jobs=%v", ids)
	}
}

func TestListIPBlacklistScoped(t *testing.T) {
	db := newTestDB(t)
	owner := mustCreate(t, db, &models.User{Username: "bl", Password: "x", Role: models.UserRoleOperator})
	other := mustCreate(t, db, &models.User{Username: "bl2", Password: "x", Role: models.UserRoleOperator})
	own := mustCreate(t, db, &models.IPBlacklist{IPAddress: "203.0.113.10", CreatedBy: owner.ID})
	foreign := mustCreate(t, db, &models.IPBlacklist{IPAddress: "203.0.113.11", CreatedBy: other.ID})

	ids := resultIDs(t, Execute(operatorExec(*owner), "list_ip_blacklist", "{}"))
	if !ids[own.ID] || ids[foreign.ID] {
		t.Fatalf("owner saw entries=%v", ids)
	}
}

func itoa(id uint) string {
	if id == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for id > 0 {
		i--
		buf[i] = byte('0' + id%10)
		id /= 10
	}
	return string(buf[i:])
}
