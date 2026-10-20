package platform

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"spearhook/models"
	"spearhook/utils"
)

// MaxHTMLUploadSizeBytes caps uploaded/generated landing HTML.
const MaxHTMLUploadSizeBytes = 30 << 20

// ErrHTMLFileTooLarge is returned when landing HTML exceeds MaxHTMLUploadSizeBytes.
var ErrHTMLFileTooLarge = errors.New("HTML file exceeds the 30MB upload limit")

// WritePreparedHTMLFile normalizes landing HTML and writes it to ./data/html_files.
func WritePreparedHTMLFile(html string) (string, error) {
	prepared := utils.PrepareLandingHTML(html)
	if strings.TrimSpace(prepared) == "" {
		return "", errors.New("HTML content is empty")
	}
	if len(prepared) > MaxHTMLUploadSizeBytes {
		return "", ErrHTMLFileTooLarge
	}

	uploadDir := "./data/html_files"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return "", err
	}

	filename := strconv.FormatInt(time.Now().UnixNano(), 10) + ".html"
	filePath := filepath.Join(uploadDir, filename)
	if err := os.WriteFile(filePath, []byte(prepared), 0644); err != nil {
		return "", err
	}
	return filePath, nil
}

// ValidateProjectName enforces the project-name charset used by the console.
func ValidateProjectName(value string) error {
	name := strings.TrimSpace(value)
	if name == "" {
		return errors.New("project name is required")
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
			return errors.New("project name must contain only English letters, numbers, and hyphens, without spaces or other special characters")
		}
	}
	return nil
}

// NormalizeRoutePath ensures a leading slash, or returns defaultValue when empty.
func NormalizeRoutePath(value string, defaultValue string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultValue
	}
	if !strings.HasPrefix(value, "/") {
		return "/" + value
	}
	return value
}

// NormalizedProjectRunMode maps unknown values to docker.
func NormalizedProjectRunMode(value models.ProjectRunMode) models.ProjectRunMode {
	switch value {
	case models.ProjectRunModeLocal:
		return models.ProjectRunModeLocal
	case models.ProjectRunModeDocker:
		return models.ProjectRunModeDocker
	default:
		return models.ProjectRunModeDocker
	}
}

// ParseProjectRunMode validates a run-mode string.
func ParseProjectRunMode(value string) (models.ProjectRunMode, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", string(models.ProjectRunModeDocker):
		return models.ProjectRunModeDocker, nil
	case string(models.ProjectRunModeLocal):
		return models.ProjectRunModeLocal, nil
	default:
		return "", fmt.Errorf("invalid run mode: %s", value)
	}
}

// ApplyPhishingPageDefaults copies the source page URL into the project login URL.
func ApplyPhishingPageDefaults(project *models.Project, page *models.PhishingPage) {
	if project == nil || page == nil {
		return
	}
	if strings.TrimSpace(project.LoginURL) == "" && strings.TrimSpace(page.URL) != "" {
		project.LoginURL = strings.TrimSpace(page.URL)
	}
}
