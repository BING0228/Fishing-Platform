package platform

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"spearhook/config"
	"spearhook/models"
	"spearhook/utils"

	"gorm.io/gorm"
)

const dockerfileTemplate = `FROM %s
WORKDIR /app
COPY . /app
RUN pip install -i https://mirrors.aliyun.com/pypi/simple -r requirements.txt
EXPOSE 5000
CMD ["python", "app.py"]
`

// DockerImageName is the per-build image tag.
func DockerImageName(projectID uint, projectName string, builtAt time.Time) string {
	return fmt.Sprintf("%d-%s-%s", projectID, strings.TrimSpace(projectName), builtAt.Format("20060102150405"))
}

// StartDockerBuild marks the project as building and launches the image build
// in the background. Poll get_project / build_status for completion.
func StartDockerBuild(project *models.Project) error {
	if project == nil {
		return &LocalProjectError{Status: 400, Message: "project is required"}
	}
	if NormalizedProjectRunMode(project.RunMode) != models.ProjectRunModeDocker {
		return &LocalProjectError{Status: 400, Message: "local Flask projects do not need a Docker image; use project_action start"}
	}
	if project.BuildStatus == models.BuildStatusBuilding {
		return &LocalProjectError{Status: 409, Message: "a build is already in progress"}
	}
	if strings.TrimSpace(project.HtmlFilePath) == "" {
		return &LocalProjectError{Status: 400, Message: "no HTML file uploaded; create the project with a page first"}
	}

	db := config.GetDB()
	taskID := fmt.Sprintf("task_%d_%d", project.ID, time.Now().Unix())
	project.BuildStatus = models.BuildStatusBuilding
	project.TaskID = taskID
	project.DockerImage = ""
	project.ContainerLog = "building Docker image"
	if err := db.Save(project).Error; err != nil {
		return &LocalProjectError{Status: 500, Message: "failed to update project build status"}
	}
	go runDockerBuild(project.ID)
	return nil
}

func runDockerBuild(projectID uint) {
	db := config.GetDB()
	var project models.Project
	if err := db.Preload("Robots").First(&project, projectID).Error; err != nil {
		return
	}
	image, code, err := BuildDockerImage(&project)
	if err != nil {
		saveDockerBuildFailure(db, projectID, "Docker build failed: "+err.Error())
		return
	}
	saveDockerBuildSuccess(db, projectID, image, code)
}

// saveDockerBuildSuccess writes the build result using DB column names. Using
// field names here silently breaks Updates (container_code/container_log do not
// exist; the columns are flask_code/flask_log).
func saveDockerBuildSuccess(db *gorm.DB, projectID uint, image, code string) error {
	return db.Model(&models.Project{}).Where("id = ?", projectID).Updates(map[string]any{
		"build_status": models.BuildStatusSuccess,
		"docker_image": image,
		"flask_code":   code,
		"flask_log":    "Docker image built: " + image,
	}).Error
}

func saveDockerBuildFailure(db *gorm.DB, projectID uint, message string) error {
	return db.Model(&models.Project{}).Where("id = ?", projectID).Updates(map[string]any{
		"build_status": models.BuildStatusFailed,
		"flask_log":    message,
	}).Error
}

// BuildDockerImage synchronously builds the project's Docker image.
func BuildDockerImage(project *models.Project) (string, string, error) {
	if project == nil {
		return "", "", fmt.Errorf("project is required")
	}
	tempDir, err := os.MkdirTemp("", "container-build-*")
	if err != nil {
		return "", "", fmt.Errorf("failed to create build directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	containerCode, err := PrepareProjectRuntimeFiles(project, tempDir)
	if err != nil {
		return "", "", err
	}

	dockerfile := fmt.Sprintf(dockerfileTemplate, utils.RequiredProjectBaseImage)
	if err := os.WriteFile(filepath.Join(tempDir, "Dockerfile"), []byte(dockerfile), 0644); err != nil {
		return "", "", fmt.Errorf("failed to write Dockerfile: %w", err)
	}

	dockerClient, err := utils.NewDockerClient()
	if err != nil {
		return "", "", fmt.Errorf("failed to connect to Docker: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	image := DockerImageName(project.ID, project.Name, time.Now())
	if err := dockerClient.BuildImage(ctx, tempDir, image); err != nil {
		return "", "", err
	}
	return image, containerCode, nil
}
