package platform

import (
	"path/filepath"
	"testing"

	"spearhook/config"
	"spearhook/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestDockerBuildResultPersists(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "build.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := models.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	config.DB = db

	project := models.Project{Name: "build-result", RunMode: models.ProjectRunModeDocker, BuildStatus: models.BuildStatusBuilding}
	if err := db.Create(&project).Error; err != nil {
		t.Fatal(err)
	}

	if err := saveDockerBuildSuccess(db, project.ID, "8-demo-20260101", "app-code"); err != nil {
		t.Fatal(err)
	}
	var ok models.Project
	if err := db.First(&ok, project.ID).Error; err != nil {
		t.Fatal(err)
	}
	if ok.BuildStatus != models.BuildStatusSuccess || ok.DockerImage != "8-demo-20260101" || ok.ContainerCode != "app-code" {
		t.Fatalf("success state = %+v", ok)
	}
	if ok.ContainerLog != "Docker image built: 8-demo-20260101" {
		t.Fatalf("log = %q", ok.ContainerLog)
	}

	if err := saveDockerBuildFailure(db, project.ID, "boom"); err != nil {
		t.Fatal(err)
	}
	var failed models.Project
	if err := db.First(&failed, project.ID).Error; err != nil {
		t.Fatal(err)
	}
	if failed.BuildStatus != models.BuildStatusFailed || failed.ContainerLog != "boom" {
		t.Fatalf("failed state = %+v", failed)
	}
}
