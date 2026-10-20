package platform

import (
	"path/filepath"
	"strings"
	"testing"

	"spearhook/config"
	"spearhook/models"
	"spearhook/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestProjectHTMLVarsIncludesEnabledQRRelay(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "qr-vars.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := models.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	config.DB = db

	relay := models.QrRelay{Name: "qr", Slug: "qr-vars-slug", Enabled: true, CreatedBy: 1}
	if err := db.Create(&relay).Error; err != nil {
		t.Fatal(err)
	}
	project := models.Project{
		Name:           "qr-project",
		QrRelayID:      &relay.ID,
		ContainerRoute: "/api/submit",
		LoginURL:       "https://example.com/login",
		FrontendRoute:  "/login",
	}

	vars := ProjectHTMLVars(&project)
	if !strings.Contains(vars.QRRelayURL, "/q/qr-vars-slug.png") {
		t.Fatalf("QRRelayURL = %q", vars.QRRelayURL)
	}
	out := utils.ApplyProjectHTMLPlaceholders(
		`<div>{{QR_RELAY_IMG}}</div><a href="{{QR_RELAY_URL}}">qr</a>`,
		vars,
	)
	if strings.Contains(out, "{{QR_RELAY") {
		t.Fatalf("placeholder not replaced: %s", out)
	}
	if !strings.Contains(out, "/q/qr-vars-slug.png") || !strings.Contains(out, "<img src=") {
		t.Fatalf("rendered HTML missing QR: %s", out)
	}

	if err := db.Model(&relay).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	vars = ProjectHTMLVars(&project)
	if strings.TrimSpace(vars.QRRelayURL) != "" {
		t.Fatalf("paused relay should not render a QR URL, got %q", vars.QRRelayURL)
	}
}
