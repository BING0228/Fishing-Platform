package chattools

import (
	"fmt"
	"testing"

	"spearhook/models"
)

func TestCreateProjectBindsQrRelay(t *testing.T) {
	db := newTestDB(t)
	owner := mustCreate(t, db, &models.User{Username: "qr-owner", Password: "x", Role: models.UserRoleOperator})
	other := mustCreate(t, db, &models.User{Username: "qr-other", Password: "x", Role: models.UserRoleOperator})

	page := mustCreate(t, db, &models.PhishingPage{
		Name:      "qr-page",
		URL:       "https://example.com/qr",
		Html:      `<html><body>{{QR_RELAY_IMG}}{{QR_RELAY_URL}}</body></html>`,
		CreatedBy: owner.ID,
	})
	relay := mustCreate(t, db, &models.QrRelay{
		Name:      "test-relay",
		Slug:      "qr-bind-relay",
		TokenHash: "hash",
		Enabled:   true,
		CreatedBy: owner.ID,
	})

	res := Execute(operatorExec(*owner), "create_project", fmt.Sprintf(
		`{"name":"qr-bind-demo","phishing_page_id":%d,"qr_relay_id":%d,"run_mode":"local"}`,
		page.ID, relay.ID,
	))
	if !res.OK {
		t.Fatalf("create_project failed: %+v", res)
	}

	var project models.Project
	if err := db.Where("name = ?", "qr-bind-demo").First(&project).Error; err != nil {
		t.Fatal(err)
	}
	if project.QrRelayID == nil || *project.QrRelayID != relay.ID {
		t.Fatalf("project qr_relay_id = %v, want %d", project.QrRelayID, relay.ID)
	}

	otherRelay := mustCreate(t, db, &models.QrRelay{
		Name:      "other-relay",
		Slug:      "qr-bind-other",
		TokenHash: "hash",
		Enabled:   true,
		CreatedBy: other.ID,
	})
	res = Execute(operatorExec(*owner), "create_project", fmt.Sprintf(
		`{"name":"qr-bind-denied","phishing_page_id":%d,"qr_relay_id":%d,"run_mode":"local"}`,
		page.ID, otherRelay.ID,
	))
	if res.OK {
		t.Fatal("operator must not bind another operator's QR relay")
	}
}
