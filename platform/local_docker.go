package platform

import (
	"context"
	"time"

	"spearhook/config"
	"spearhook/models"
	"spearhook/utils"
)

// LocalDockerStartResult describes a local Docker start.
type LocalDockerStartResult struct {
	ContainerID    string
	URL            string
	AlreadyRunning bool
	Log            string
}

// StartLocalDockerProject runs the project's built image as a container on the
// control-plane host. No Agent is involved.
func StartLocalDockerProject(project *models.Project) (LocalDockerStartResult, error) {
	var out LocalDockerStartResult
	if project == nil {
		return out, &LocalProjectError{Status: 400, Message: "project is required"}
	}
	if NormalizedProjectRunMode(project.RunMode) != models.ProjectRunModeDocker {
		return out, &LocalProjectError{Status: 400, Message: "project is configured for local Flask; use the local runtime"}
	}
	if project.Port == 0 {
		project.Port = 8080
	}
	if err := ValidatePublicPort(project.Port); err != nil {
		return out, &LocalProjectError{Status: 400, Message: err.Error()}
	}
	if project.ContainerStatus == models.ContainerStatusRunning {
		if _, ok := ParseLocalContainerPID(project.ContainerID); ok {
			return out, &LocalProjectError{Status: 409, Message: "local Flask is running; stop it before starting the Docker container"}
		}
		if project.ContainerID != "" {
			out.ContainerID = project.ContainerID
			out.URL = ProjectServiceURL(EffectivePublicBaseURL(), *project)
			out.AlreadyRunning = true
			out.Log = project.ContainerLog
			return out, nil
		}
	}
	if project.BuildStatus != models.BuildStatusSuccess || project.DockerImage == "" {
		return out, &LocalProjectError{Status: 400, Message: "project is not built; build the Docker image first"}
	}
	if project.UseHTTPS && !HasProjectSSLCert(*project) {
		return out, &LocalProjectError{Status: 400, Message: "HTTPS is enabled but the project has no certificate/key"}
	}

	dockerClient, err := utils.NewDockerClient()
	if err != nil {
		return out, &LocalProjectError{Status: 500, Message: "failed to connect to Docker: " + err.Error()}
	}

	db := config.GetDB()
	project.ContainerStatus = models.ContainerStatusStarting
	project.ContainerLog = "starting Docker container"
	if err := db.Save(project).Error; err != nil {
		return out, &LocalProjectError{Status: 500, Message: "failed to update project status"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	containerID, err := dockerClient.CreateContainer(ctx, project.DockerImage, int(project.Port), project.ID, project.UseHTTPS, project.SSLCertPath, project.SSLKeyPath)
	if err != nil {
		project.ContainerStatus = models.ContainerStatusStopped
		project.ContainerLog = "failed to create container: " + err.Error()
		_ = db.Save(project).Error
		return out, &LocalProjectError{Status: 500, Message: project.ContainerLog}
	}
	if err := dockerClient.StartContainer(ctx, containerID); err != nil {
		project.ContainerStatus = models.ContainerStatusStopped
		project.ContainerLog = "failed to start container: " + err.Error()
		_ = db.Save(project).Error
		return out, &LocalProjectError{Status: 500, Message: project.ContainerLog}
	}

	project.ContainerStatus = models.ContainerStatusRunning
	project.ContainerID = containerID
	project.ContainerLog = "Docker container running"
	if err := db.Save(project).Error; err != nil {
		return out, &LocalProjectError{Status: 500, Message: "failed to save container state"}
	}

	out.ContainerID = containerID
	out.URL = ProjectServiceURL(EffectivePublicBaseURL(), *project)
	out.Log = project.ContainerLog
	return out, nil
}

// StopLocalDockerProject stops and removes the local Docker container.
func StopLocalDockerProject(project *models.Project) error {
	if project == nil {
		return &LocalProjectError{Status: 400, Message: "project is required"}
	}
	if project.ContainerStatus != models.ContainerStatusRunning {
		return nil
	}
	if _, ok := ParseLocalContainerPID(project.ContainerID); ok {
		return &LocalProjectError{Status: 409, Message: "local Flask is running; use the local stop action"}
	}
	if project.ContainerID == "" {
		return &LocalProjectError{Status: 400, Message: "no Docker container id on this project"}
	}

	dockerClient, err := utils.NewDockerClient()
	if err != nil {
		return &LocalProjectError{Status: 500, Message: "failed to connect to Docker: " + err.Error()}
	}

	db := config.GetDB()
	project.ContainerStatus = models.ContainerStatusStopping
	project.ContainerLog = "stopping Docker container"
	_ = db.Save(project).Error

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := dockerClient.StopContainer(ctx, project.ContainerID); err != nil {
		project.ContainerStatus = models.ContainerStatusRunning
		project.ContainerLog = "failed to stop container: " + err.Error()
		_ = db.Save(project).Error
		return &LocalProjectError{Status: 500, Message: project.ContainerLog}
	}
	// Removal failure is non-fatal once the container is stopped.
	_ = dockerClient.RemoveContainer(ctx, project.ContainerID)

	project.ContainerStatus = models.ContainerStatusStopped
	project.ContainerID = ""
	project.ContainerLog = "Docker container stopped and removed"
	if err := db.Save(project).Error; err != nil {
		return &LocalProjectError{Status: 500, Message: "failed to save stopped state"}
	}
	return nil
}
