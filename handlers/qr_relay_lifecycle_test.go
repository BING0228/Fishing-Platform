package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"spearhook/config"
	"spearhook/models"
	"spearhook/utils"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type qrTestEnv struct {
	router *gin.Engine
	db     *gorm.DB
	a      models.User
	b      models.User
}

func newQrTestEnv(t *testing.T) *qrTestEnv {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "qr-life.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir()) // isolate data/qr_relays
	if err := models.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	config.DB = db

	a := models.User{Username: "qr-a", Password: "x", Role: models.UserRoleOperator}
	b := models.User{Username: "qr-b", Password: "x", Role: models.UserRoleOperator}
	if err := db.Create(&a).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&b).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if c.GetHeader("X-Test-User") == "b" {
			c.Set("user", b)
		} else {
			c.Set("user", a)
		}
		c.Next()
	})
	router.POST("/qr-relays", CreateQrRelay)
	router.GET("/qr-relays/:id/frames/", ListQrRelayFrames)
	router.GET("/qr-relays/:id/frames/:fid/", ServeQrRelayFrameImage)
	router.POST("/qr-relays/:id/frames/:fid/promote/", PromoteQrRelayFrame)
	router.POST("/qr-relays/:id/rotate-token/", RotateQrRelayToken)
	router.POST("/qr-relays/:id/rotate-slug/", RotateQrRelaySlug)
	router.POST("/qr-relays/:id/enabled/", SetQrRelayEnabled)
	router.POST("/api/qr-relay/frames/", UploadQrRelayFrame)
	router.POST("/api/qr-relay/heartbeat/", HeartbeatQrRelay)
	router.GET("/q/:slug", ServeQrRelayImage)

	return &qrTestEnv{router: router, db: db, a: a, b: b}
}

func (e *qrTestEnv) do(t *testing.T, method, path, user string, body io.Reader, contentType string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if user == "a" {
		req.Header.Set("X-Test-User", "a")
	} else if user == "b" {
		req.Header.Set("X-Test-User", "b")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

func (e *qrTestEnv) upload(t *testing.T, token string, data []byte, payload string, extraFile bool) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("image", "qr.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(data)
	if payload != "" {
		_ = w.WriteField("payload", payload)
	}
	if extraFile {
		extra, _ := w.CreateFormFile("other", "x.txt")
		_, _ = extra.Write([]byte("nope"))
	}
	_ = w.Close()
	return e.do(t, http.MethodPost, "/api/qr-relay/frames/", "", &buf, w.FormDataContentType(),
		map[string]string{"Authorization": "Bearer " + token})
}

func (e *qrTestEnv) createRelay(t *testing.T) (id uint, slug, token, publicPath string) {
	t.Helper()
	rec := e.do(t, http.MethodPost, "/qr-relays", "a", strings.NewReader(`{"name":"lifecycle"}`), "application/json", nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID          uint   `json:"id"`
		Slug        string `json:"slug"`
		UploadToken string `json:"upload_token"`
		PublicPath  string `json:"public_path"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 || created.Slug == "" || created.UploadToken == "" {
		t.Fatalf("created=%+v", created)
	}
	return created.ID, created.Slug, created.UploadToken, created.PublicPath
}

func TestQrRelayLifecycleAndSecurity(t *testing.T) {
	e := newQrTestEnv(t)
	id, slug, token, publicPath := e.createRelay(t)

	frame1 := append(append([]byte{}, utils.MinimalPNG()...), 'A')
	frame2 := append(append([]byte{}, utils.MinimalPNG()...), 'B')

	// Wrong token.
	if rec := e.upload(t, "wrong", frame1, "", false); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token status=%d", rec.Code)
	}

	// Frame 1.
	if rec := e.upload(t, token, frame1, "first", false); rec.Code != http.StatusOK {
		t.Fatalf("frame1 status=%d body=%s", rec.Code, rec.Body.String())
	}
	pub := e.do(t, http.MethodGet, publicPath, "", nil, "", nil)
	if pub.Code != http.StatusOK || !bytes.Equal(pub.Body.Bytes(), frame1) {
		t.Fatalf("public frame1 status=%d match=%v", pub.Code, bytes.Equal(pub.Body.Bytes(), frame1))
	}

	// Ownership: B cannot list, serve frames, promote, rotate, or pause.
	framesPath := fmt.Sprintf("/qr-relays/%d/frames/", id)
	if rec := e.do(t, http.MethodGet, framesPath, "b", nil, "", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("B list status=%d", rec.Code)
	}
	if rec := e.do(t, http.MethodPost, fmt.Sprintf("/qr-relays/%d/rotate-token/", id), "b", nil, "", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("B rotate token status=%d", rec.Code)
	}
	if rec := e.do(t, http.MethodPost, fmt.Sprintf("/qr-relays/%d/enabled/", id), "b", strings.NewReader(`{"enabled":false}`), "application/json", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("B pause status=%d", rec.Code)
	}

	// Rotate token: old is rejected, new works.
	rot := e.do(t, http.MethodPost, fmt.Sprintf("/qr-relays/%d/rotate-token/", id), "a", nil, "", nil)
	if rot.Code != http.StatusOK {
		t.Fatalf("rotate token status=%d body=%s", rot.Code, rot.Body.String())
	}
	var rotated struct {
		UploadToken string `json:"upload_token"`
	}
	if err := json.Unmarshal(rot.Body.Bytes(), &rotated); err != nil {
		t.Fatal(err)
	}
	if rotated.UploadToken == "" || rotated.UploadToken == token {
		t.Fatalf("new token missing/unchanged: %q", rotated.UploadToken)
	}
	if rec := e.upload(t, token, frame1, "", false); rec.Code != http.StatusUnauthorized {
		t.Fatalf("old token status=%d want 401", rec.Code)
	}
	if rec := e.upload(t, rotated.UploadToken, frame2, "second", false); rec.Code != http.StatusOK {
		t.Fatalf("frame2 status=%d body=%s", rec.Code, rec.Body.String())
	}

	// History list (newest first) and promote the older frame.
	list := e.do(t, http.MethodGet, framesPath, "a", nil, "", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list status=%d", list.Code)
	}
	var frames []struct {
		ID      uint   `json:"id"`
		Payload string `json:"payload"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &frames); err != nil {
		t.Fatal(err)
	}
	if len(frames) != 2 {
		t.Fatalf("frames=%d want 2", len(frames))
	}
	var firstFrameID uint
	for _, f := range frames {
		if f.Payload == "first" {
			firstFrameID = f.ID
		}
	}
	if firstFrameID == 0 {
		t.Fatalf("first frame not in list: %+v", frames)
	}
	if rec := e.do(t, http.MethodGet, fmt.Sprintf("/qr-relays/%d/frames/%d/", id, firstFrameID), "b", nil, "", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("B frame image status=%d", rec.Code)
	}
	if rec := e.do(t, http.MethodGet, fmt.Sprintf("/qr-relays/%d/frames/%d/", id, firstFrameID), "a", nil, "", nil); rec.Code != http.StatusOK {
		t.Fatalf("A frame image status=%d", rec.Code)
	}
	if rec := e.do(t, http.MethodPost, fmt.Sprintf("/qr-relays/%d/frames/%d/promote/", id, firstFrameID), "a", nil, "", nil); rec.Code != http.StatusOK {
		t.Fatalf("promote status=%d body=%s", rec.Code, rec.Body.String())
	}
	if pub := e.do(t, http.MethodGet, publicPath, "", nil, "", nil); pub.Code != http.StatusOK || !bytes.Equal(pub.Body.Bytes(), frame1) {
		t.Fatalf("public after promote status=%d match=%v", pub.Code, bytes.Equal(pub.Body.Bytes(), frame1))
	}

	// Pause: public 404, upload/heartbeat 403; re-enable restores hosting.
	if rec := e.do(t, http.MethodPost, fmt.Sprintf("/qr-relays/%d/enabled/", id), "a", strings.NewReader(`{"enabled":false}`), "application/json", nil); rec.Code != http.StatusOK {
		t.Fatalf("pause status=%d", rec.Code)
	}
	if pub := e.do(t, http.MethodGet, publicPath, "", nil, "", nil); pub.Code != http.StatusNotFound {
		t.Fatalf("paused public status=%d want 404", pub.Code)
	}
	if rec := e.upload(t, rotated.UploadToken, frame1, "", false); rec.Code != http.StatusForbidden {
		t.Fatalf("paused upload status=%d want 403", rec.Code)
	}
	hb := e.do(t, http.MethodPost, "/api/qr-relay/heartbeat/", "", nil, "", map[string]string{"X-QR-Relay-Token": rotated.UploadToken})
	if hb.Code != http.StatusForbidden {
		t.Fatalf("paused heartbeat status=%d want 403", hb.Code)
	}
	if rec := e.do(t, http.MethodPost, fmt.Sprintf("/qr-relays/%d/enabled/", id), "a", strings.NewReader(`{"enabled":true}`), "application/json", nil); rec.Code != http.StatusOK {
		t.Fatalf("resume status=%d", rec.Code)
	}

	// Rotate slug: old public path 404, new path 200.
	slugRot := e.do(t, http.MethodPost, fmt.Sprintf("/qr-relays/%d/rotate-slug/", id), "a", nil, "", nil)
	if slugRot.Code != http.StatusOK {
		t.Fatalf("rotate slug status=%d", slugRot.Code)
	}
	var newSlug struct {
		Slug string `json:"slug"`
	}
	if err := json.Unmarshal(slugRot.Body.Bytes(), &newSlug); err != nil {
		t.Fatal(err)
	}
	if newSlug.Slug == "" || newSlug.Slug == slug {
		t.Fatalf("new slug=%q old=%q", newSlug.Slug, slug)
	}
	if pub := e.do(t, http.MethodGet, publicPath, "", nil, "", nil); pub.Code != http.StatusNotFound {
		t.Fatalf("old slug status=%d want 404", pub.Code)
	}
	if pub := e.do(t, http.MethodGet, utils.QRRelayPublicPath(newSlug.Slug), "", nil, "", nil); pub.Code != http.StatusOK {
		t.Fatalf("new slug status=%d", pub.Code)
	}

	// Extra file parts are rejected.
	if rec := e.upload(t, rotated.UploadToken, frame1, "", true); rec.Code != http.StatusBadRequest {
		t.Fatalf("extra file status=%d want 400", rec.Code)
	}
}
