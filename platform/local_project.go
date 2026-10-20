package platform

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"spearhook/config"
	"spearhook/models"
	"spearhook/utils"
)

const (
	// LocalContainerIDPrefix marks a ContainerID that refers to a local process.
	LocalContainerIDPrefix = "local:"
	// FlaskRequirementsContent is written into every local runtime directory.
	FlaskRequirementsContent = "flask==2.2.2\nrequests==2.26.0\nWerkzeug==2.2.2\nflask-cors==4.0.0\n"
)

// LocalProjectError carries the HTTP status the handler should return.
type LocalProjectError struct {
	Status  int
	Message string
}

func (e *LocalProjectError) Error() string { return e.Message }

// HasProjectSSLCert reports whether both certificate and key are configured.
func HasProjectSSLCert(project models.Project) bool {
	return strings.TrimSpace(project.SSLCertPath) != "" && strings.TrimSpace(project.SSLKeyPath) != ""
}

// LocalProjectAppDir is the working directory for a local Flask project.
func LocalProjectAppDir(projectID uint) string {
	return filepath.Join("data", "local_apps", fmt.Sprintf("project_%d", projectID))
}

// LocalProjectLogPath is the Flask log file for a local project.
func LocalProjectLogPath(projectID uint) string {
	return filepath.Join(LocalProjectAppDir(projectID), "flask.log")
}

// MakeLocalContainerID encodes a local PID in the ContainerID field.
func MakeLocalContainerID(pid int) string {
	return fmt.Sprintf("%s%d", LocalContainerIDPrefix, pid)
}

// ParseLocalContainerPID extracts a PID from a local ContainerID.
func ParseLocalContainerPID(containerID string) (int, bool) {
	if !strings.HasPrefix(containerID, LocalContainerIDPrefix) {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimPrefix(containerID, LocalContainerIDPrefix))
	return pid, err == nil && pid > 0
}

// ProjectHTMLVars builds placeholder values for a project's landing HTML.
func ProjectHTMLVars(project *models.Project) utils.ProjectHTMLVars {
	vars := utils.ProjectHTMLVars{SubmitURL: "/api/submit"}
	if project == nil {
		return vars
	}
	vars.SubmitURL = project.ContainerRoute
	vars.RedirectURL = project.LoginURL
	if project.QrRelayID == nil || *project.QrRelayID == 0 {
		return vars
	}
	var relay models.QrRelay
	if err := config.GetDB().Select("id", "slug", "enabled").First(&relay, *project.QrRelayID).Error; err != nil {
		return vars
	}
	if !relay.Enabled || strings.TrimSpace(relay.Slug) == "" {
		return vars
	}
	path := utils.QRRelayPublicPath(relay.Slug)
	if base := EffectivePublicBaseURL(); base != "" {
		vars.QRRelayURL = strings.TrimRight(base, "/") + path
	} else {
		vars.QRRelayURL = path
	}
	return vars
}

// PrepareProjectRuntimeFiles generates app.py/requirements/index.html for a
// local Flask project and returns the generated app code.
func PrepareProjectRuntimeFiles(project *models.Project, targetDir string) (string, error) {
	if strings.TrimSpace(targetDir) == "" {
		return "", errors.New("runtime directory is required")
	}
	if project == nil || project.HtmlFilePath == "" {
		return "", errors.New("no HTML file uploaded")
	}
	if _, err := os.Stat(project.HtmlFilePath); err != nil {
		return "", fmt.Errorf("HTML file not found: %w", err)
	}

	project.FrontendRoute = NormalizeRoutePath(project.FrontendRoute, "/")
	project.ContainerRoute = NormalizeRoutePath(project.ContainerRoute, "/api/submit")

	if err := os.RemoveAll(targetDir); err != nil {
		return "", fmt.Errorf("failed to reset runtime directory: %w", err)
	}
	templatesDir := filepath.Join(targetDir, "templates")
	staticDir := filepath.Join(targetDir, "static")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create templates directory: %w", err)
	}
	if err := os.MkdirAll(staticDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create static directory: %w", err)
	}

	rendered, err := utils.RenderProjectHTMLFile(project.HtmlFilePath, ProjectHTMLVars(project))
	if err != nil {
		return "", fmt.Errorf("failed to write HTML file: %w", err)
	}
	if err := os.WriteFile(filepath.Join(templatesDir, "index.html"), rendered, 0644); err != nil {
		return "", fmt.Errorf("failed to write HTML file: %w", err)
	}

	robotConfigs := make([]utils.RobotConfig, len(project.Robots))
	for i, robot := range project.Robots {
		robotConfigs[i] = utils.RobotConfig{
			ID:      robot.ID,
			Webhook: robot.Webhook,
			Type:    string(robot.RobotType),
			Secret:  robot.Secret,
		}
	}

	containerSecret := config.ContainerSecret()
	if containerSecret == "" {
		return "", errors.New("CONTAINER_SECRET is not configured")
	}

	containerCode := utils.GenerateContainerCode(
		project.Name,
		project.ID,
		project.FrontendRoute,
		project.ContainerRoute,
		project.LoginURL,
		containerSecret,
		robotConfigs,
	)
	if err := os.WriteFile(filepath.Join(targetDir, "app.py"), []byte(containerCode), 0644); err != nil {
		return "", fmt.Errorf("failed to write app.py: %w", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "requirements.txt"), []byte(FlaskRequirementsContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write requirements.txt: %w", err)
	}
	return containerCode, nil
}

// ProjectServiceURL is the local runtime URL (public host, project port, route).
func ProjectServiceURL(publicBase string, project models.Project) string {
	host := "127.0.0.1"
	if parsed, err := url.Parse(strings.TrimSpace(publicBase)); err == nil && parsed.Hostname() != "" {
		host = parsed.Hostname()
	}
	protocol := "http"
	if project.UseHTTPS {
		protocol = "https"
	}
	route := NormalizeRoutePath(project.FrontendRoute, "/")
	if route == "/" {
		route = ""
	}
	return fmt.Sprintf("%s://%s:%d%s", protocol, host, project.Port, route)
}

// LocalProjectStartResult describes a successful local start.
type LocalProjectStartResult struct {
	PID            int
	URL            string
	AlreadyRunning bool
	Log            string
}

// StartLocalProject prepares the runtime directory and starts local Flask.
// The caller must preload project.Robots.
func StartLocalProject(project *models.Project, backendBaseURL string) (LocalProjectStartResult, error) {
	var out LocalProjectStartResult
	if project == nil {
		return out, &LocalProjectError{Status: 400, Message: "project is required"}
	}
	if NormalizedProjectRunMode(project.RunMode) != models.ProjectRunModeLocal {
		return out, &LocalProjectError{Status: 400, Message: "project is configured for Docker; use Agent deployment"}
	}
	db := config.GetDB()
	if project.Port == 0 {
		project.Port = 8080
	}
	if err := ValidatePublicPort(project.Port); err != nil {
		return out, &LocalProjectError{Status: 400, Message: err.Error()}
	}
	if project.ContainerStatus == models.ContainerStatusRunning && project.ContainerID != "" {
		if pid, ok := ParseLocalContainerPID(project.ContainerID); ok {
			out.PID = pid
			out.URL = ProjectServiceURL(EffectivePublicBaseURL(), *project)
			out.AlreadyRunning = true
			out.Log = project.ContainerLog
			return out, nil
		}
		return out, &LocalProjectError{Status: 409, Message: "a Docker container is running; stop it before starting local Flask"}
	}
	if project.ContainerStatus == models.ContainerStatusStarting || project.ContainerStatus == models.ContainerStatusStopping {
		return out, &LocalProjectError{Status: 409, Message: "project is starting or stopping; retry shortly"}
	}
	if project.UseHTTPS && !HasProjectSSLCert(*project) {
		return out, &LocalProjectError{Status: 400, Message: "HTTPS is enabled but the project has no certificate/key"}
	}

	project.ContainerStatus = models.ContainerStatusStarting
	project.ContainerLog = "preparing local Flask runtime"
	if err := db.Save(project).Error; err != nil {
		return out, &LocalProjectError{Status: 500, Message: "failed to update project status"}
	}

	appDir := LocalProjectAppDir(project.ID)
	containerCode, err := PrepareProjectRuntimeFiles(project, appDir)
	if err != nil {
		project.ContainerStatus = models.ContainerStatusStopped
		project.ContainerLog = "local Flask preparation failed: " + err.Error()
		project.BuildStatus = models.BuildStatusFailed
		_ = db.Save(project).Error
		return out, &LocalProjectError{Status: 400, Message: project.ContainerLog}
	}

	project.ContainerCode = containerCode
	project.BuildStatus = models.BuildStatusSuccess
	project.ContainerLog = "starting local Flask"
	if err := db.Save(project).Error; err != nil {
		return out, &LocalProjectError{Status: 500, Message: "failed to save local runtime files"}
	}

	process, err := utils.StartLocalFlaskApp(utils.LocalFlaskConfig{
		AppDir:         appDir,
		LogPath:        LocalProjectLogPath(project.ID),
		Port:           project.Port,
		BackendBaseURL: backendBaseURL,
		UseHTTPS:       project.UseHTTPS,
		SSLCertPath:    project.SSLCertPath,
		SSLKeyPath:     project.SSLKeyPath,
	})
	if err != nil {
		project.ContainerStatus = models.ContainerStatusStopped
		project.ContainerID = ""
		project.ContainerLog = "local Flask start failed: " + err.Error()
		_ = db.Save(project).Error
		return out, &LocalProjectError{Status: 500, Message: project.ContainerLog}
	}

	project.ContainerStatus = models.ContainerStatusRunning
	project.ContainerID = MakeLocalContainerID(process.PID)
	project.ContainerLog = fmt.Sprintf("local Flask running\nPID: %d\nDirectory: %s\nPython: %s", process.PID, appDir, process.PythonPath)
	if err := db.Save(project).Error; err != nil {
		return out, &LocalProjectError{Status: 500, Message: "failed to save local running state"}
	}

	out.PID = process.PID
	out.URL = ProjectServiceURL(EffectivePublicBaseURL(), *project)
	out.Log = project.ContainerLog
	return out, nil
}

// StopLocalProject stops a local Flask process and updates project state.
func StopLocalProject(project *models.Project, pid int) error {
	if project == nil {
		return &LocalProjectError{Status: 400, Message: "project is required"}
	}
	db := config.GetDB()
	project.ContainerStatus = models.ContainerStatusStopping
	project.ContainerLog = "stopping local Flask"
	if err := db.Save(project).Error; err != nil {
		return &LocalProjectError{Status: 500, Message: "failed to update project status"}
	}
	if err := utils.StopLocalProcess(pid); err != nil {
		project.ContainerLog = "local Flask stop error: " + err.Error()
	} else {
		project.ContainerLog = fmt.Sprintf("local Flask stopped\nPID: %d", pid)
	}
	project.ContainerStatus = models.ContainerStatusStopped
	project.ContainerID = ""
	if err := db.Save(project).Error; err != nil {
		return &LocalProjectError{Status: 500, Message: "failed to save stopped state"}
	}
	return nil
}
