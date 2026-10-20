package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"spearhook/models"
	"spearhook/utils"

	"github.com/gin-gonic/gin"
)

func TestSearchProfilesRoundTrip(t *testing.T) {
	db := newChatTestDB(t)
	_ = utils.InitEncryption("test-encryption-key-for-search-profiles-123")
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/search-settings/", GetSearchSettings)
	router.PUT("/api/search-settings/", UpdateSearchSettings)
	router.POST("/api/search-settings/test/", TestSearchSettings)

	decode := func(w *httptest.ResponseRecorder) map[string]any {
		t.Helper()
		var out map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode %s: %v", w.Body.String(), err)
		}
		return out
	}
	doGet := func() map[string]any {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/search-settings/", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("GET status %d: %s", w.Code, w.Body.String())
		}
		return decode(w)
	}
	doPut := func(body string) map[string]any {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/search-settings/", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("PUT status %d: %s", w.Code, w.Body.String())
		}
		return decode(w)
	}
	doPutStatus := func(body string, status int) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/search-settings/", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		if w.Code != status {
			t.Fatalf("PUT status = %d, want %d: %s", w.Code, status, w.Body.String())
		}
	}
	doPost := func(body string) map[string]any {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/search-settings/test/", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("POST status %d: %s", w.Code, w.Body.String())
		}
		return decode(w)
	}

	initial := doGet()
	profiles, _ := initial["profiles"].([]any)
	if len(profiles) != 1 {
		t.Fatalf("initial profiles = %d, want 1", len(profiles))
	}
	first, _ := profiles[0].(map[string]any)
	if first["provider"] != "auto" || first["enabled"] != true {
		t.Fatalf("initial profile = %+v", first)
	}
	providers, _ := initial["providers"].([]any)
	if len(providers) != len(utils.SearchProviders()) {
		t.Fatalf("provider catalog = %d rows", len(providers))
	}

	saved := doPut(`{"profiles":[
		{"id":"p1","name":"DeepSeek","enabled":true,"provider":"deepseek","api_key":"sk-test-secret","count":7},
		{"id":"p2","name":"Brave","enabled":true,"provider":"brave","api_key":"brave-secret","count":5},
		{"id":"p3","name":"Native","enabled":false,"provider":"auto","count":10}
	]}`)
	savedProfiles, _ := saved["profiles"].([]any)
	if len(savedProfiles) != 3 {
		t.Fatalf("saved profiles = %d, want 3", len(savedProfiles))
	}
	byID := map[string]map[string]any{}
	for _, raw := range savedProfiles {
		p, _ := raw.(map[string]any)
		byID[p["id"].(string)] = p
	}
	if byID["p1"]["base_url"] != "https://api.deepseek.com" {
		t.Fatalf("p1 base_url = %v", byID["p1"]["base_url"])
	}
	if byID["p1"]["api_key_set"] != true {
		t.Fatal("p1 should have an API key")
	}
	if masked, _ := byID["p1"]["api_key"].(string); !strings.Contains(masked, "*") || strings.Contains(masked, "test") {
		t.Fatalf("p1 api_key should be masked, got %q", masked)
	}
	if byID["p1"]["count"] != float64(7) || byID["p2"]["count"] != float64(5) {
		t.Fatalf("counts = %v / %v", byID["p1"]["count"], byID["p2"]["count"])
	}
	if byID["p2"]["enabled"] != true || byID["p1"]["enabled"] != true {
		t.Fatal("both standalone profiles should remain enabled")
	}
	if byID["p3"]["base_url"] != "" {
		t.Fatalf("auto base_url = %v", byID["p3"]["base_url"])
	}

	var stored []models.SearchProfile
	if err := db.Order("id ASC").Find(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if len(stored) != 3 {
		t.Fatalf("stored profiles = %d", len(stored))
	}
	storedByID := map[string]models.SearchProfile{}
	for _, row := range stored {
		storedByID[row.ID] = row
	}
	if storedByID["p1"].APIKey == "" || storedByID["p1"].APIKey == "sk-test-secret" {
		t.Fatal("p1 API key must be encrypted at rest")
	}
	originalKey := storedByID["p1"].APIKey

	// A masked key must keep the stored secret for the matching profile.
	doPut(`{"profiles":[
		{"id":"p1","name":"DeepSeek","enabled":true,"provider":"deepseek","api_key":"sk-********","count":9},
		{"id":"p2","name":"Brave","enabled":true,"provider":"brave","api_key":"sk-********","count":5}
	]}`)
	var kept models.SearchProfile
	if err := db.Where("id = ?", "p1").First(&kept).Error; err != nil {
		t.Fatal(err)
	}
	if kept.APIKey != originalKey {
		t.Fatal("masked key overwrote the stored secret")
	}
	if kept.Count != 9 {
		t.Fatalf("kept count = %d, want 9", kept.Count)
	}

	// A masked key with no matching existing profile is ambiguous.
	doPutStatus(`{"profiles":[{"id":"new","name":"New","enabled":true,"provider":"brave","api_key":"sk-********"}]}`, http.StatusBadRequest)

	// Deleting every profile is rejected; disable instead.
	doPutStatus(`{"profiles":[]}`, http.StatusBadRequest)

	// Auto with no enabled AI profile returns a structured failure.
	autoTest := doPost(`{"provider":"auto"}`)
	if autoTest["ok"] != false || !strings.Contains(autoTest["error"].(string), "No AI profile") {
		t.Fatalf("auto test = %+v", autoTest)
	}

	// A missing saved id is a 404.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/search-settings/test/", strings.NewReader(`{"id":"missing"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("missing id status = %d, want 404", w.Code)
	}

	// A standalone provider without a key reports a structured failure.
	braveTest := doPost(`{"provider":"brave","count":3}`)
	if braveTest["ok"] != false || !strings.Contains(braveTest["error"].(string), "API key") {
		t.Fatalf("brave test = %+v", braveTest)
	}
}
