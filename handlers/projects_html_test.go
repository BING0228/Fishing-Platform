package handlers

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"fishing-platform-backend/config"
	"fishing-platform-backend/models"
	"fishing-platform-backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupProjectHTMLTest(t *testing.T) (*gorm.DB, models.User, models.User) {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	db, err := gorm.Open(sqlite.Open(filepath.Join(dir, "proj.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := models.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	config.DB = db
	owner := models.User{Username: "ops", Password: "x", Role: models.UserRoleOperator}
	other := models.User{Username: "other", Password: "x", Role: models.UserRoleOperator}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	return db, owner, other
}

func postCreateProject(t *testing.T, router *gin.Engine, fields map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/projects/", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	router.ServeHTTP(rec, req)
	return rec
}

func TestCreateProjectFromPhishingPageCopiesPlaceholders(t *testing.T) {
	db, owner, _ := setupProjectHTMLTest(t)
	page := models.PhishingPage{
		Name:      "portal-login",
		URL:       "https://origin.example/login",
		Html:      `<form action="/login"><input name="email"><input type="password" name="pwd"></form>`,
		CreatedBy: owner.ID,
	}
	if err := db.Create(&page).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/projects/", func(c *gin.Context) {
		c.Set("user", owner)
		CreateProject(c)
	})

	rec := postCreateProject(t, router, map[string]string{
		"name":             "demo-page",
		"run_mode":         "local",
		"frontend_route":   "/",
		"port":             "6001",
		"phishing_page_id": strconv.FormatUint(uint64(page.ID), 10),
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	var created models.Project
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.LoginURL != "https://origin.example/login" {
		t.Fatalf("login url not copied: %q", created.LoginURL)
	}
	raw, err := os.ReadFile(created.HtmlFilePath)
	if err != nil {
		t.Fatal(err)
	}
	stored := string(raw)
	if !strings.Contains(stored, utils.PlaceholderSubmitURL) {
		t.Fatalf("stored HTML missing placeholder: %s", stored)
	}
	if strings.Contains(stored, `action="/login"`) {
		t.Fatalf("stored HTML still points at origin login: %s", stored)
	}

	rendered, err := utils.RenderProjectHTMLFile(created.HtmlFilePath, projectHTMLVars(&created))
	if err != nil {
		t.Fatal(err)
	}
	out := string(rendered)
	if !strings.Contains(out, `action="/api/submit"`) {
		t.Fatalf("build HTML missing container submit: %s", out)
	}
	if strings.Contains(out, "{{SUBMIT_URL}}") {
		t.Fatalf("placeholder left after render: %s", out)
	}
}

func TestCreateProjectRejectsForeignPhishingPage(t *testing.T) {
	db, owner, other := setupProjectHTMLTest(t)
	page := models.PhishingPage{
		Name:      "secret",
		Html:      `<form><input name="username"><input type="password" name="password"></form>`,
		CreatedBy: other.ID,
	}
	if err := db.Create(&page).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/projects/", func(c *gin.Context) {
		c.Set("user", owner)
		CreateProject(c)
	})

	rec := postCreateProject(t, router, map[string]string{
		"name":             "nope",
		"run_mode":         "local",
		"frontend_route":   "/",
		"port":             "6002",
		"phishing_page_id": strconv.FormatUint(uint64(page.ID), 10),
	})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign page status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCreateProjectRequiresHTML(t *testing.T) {
	_, owner, _ := setupProjectHTMLTest(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/projects/", func(c *gin.Context) {
		c.Set("user", owner)
		CreateProject(c)
	})
	rec := postCreateProject(t, router, map[string]string{
		"name":           "empty-html",
		"run_mode":       "local",
		"frontend_route": "/",
		"port":           "6003",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing html status=%d body=%s", rec.Code, rec.Body.String())
	}
}
