package admin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/testutil"
)

func TestInviteRoutesCreateInvitePersistsExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)

	req := httptest.NewRequest(http.MethodPost, "/admin/invites", strings.NewReader(`{"code":"route-invite","total_uses":2,"note":"Route Invite"}`))
	rec := httptest.NewRecorder()
	h.CreateInvite(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"code\":\"route-invite\",\"note\":\"Route Invite\",\"total_uses\":2}\n" {
		t.Fatalf("create invite response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	invite, err := db.Invites.Get(req.Context(), "route-invite")
	if err != nil || invite == nil || invite.Code != "route-invite" || invite.Note != "Route Invite" || invite.TotalUses == nil || *invite.TotalUses != 2 {
		t.Fatalf("created invite state mismatch: invite=%#v err=%v", invite, err)
	}
}

func TestInviteRoutesListAndDeleteExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)
	if err := db.Invites.Create(context.Background(), "route-list-invite", 3, "List Invite"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/invites?limit=1", nil)
	rec := httptest.NewRecorder()
	h.Invites(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"code":"route-list-invite"`) || !strings.Contains(rec.Body.String(), `"page_size":1`) {
		t.Fatalf("invite list response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/admin/invites/route-list-invite", nil)
	req.SetPathValue("code", "route-list-invite")
	rec = httptest.NewRecorder()
	h.DeleteInvite(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("delete invite response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if invite, err := db.Invites.Get(req.Context(), "route-list-invite"); err != nil || invite != nil {
		t.Fatalf("invite should be deleted: invite=%#v err=%v", invite, err)
	}
}

func TestInviteRoutesRejectInvalidInputsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.New(testutil.TestConfig(), db, nil)

	req := httptest.NewRequest(http.MethodGet, "/admin/invites?cursor=not-base64", nil)
	rec := httptest.NewRecorder()
	h.Invites(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"Invalid cursor\"}\n" {
		t.Fatalf("invite list invalid cursor mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/invites", strings.NewReader(`{`))
	rec = httptest.NewRecorder()
	h.CreateInvite(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invalid json\"}\n" {
		t.Fatalf("invite create bad json mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/invites", strings.NewReader(`{"code":"abc","total_uses":5,"note":"too short"}`))
	rec = httptest.NewRecorder()
	h.CreateInvite(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invite code too short\"}\n" {
		t.Fatalf("invite short code mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if invite, err := db.Invites.Get(req.Context(), "abc"); err != nil || invite != nil {
		t.Fatalf("short invite code should not persist: invite=%#v err=%v", invite, err)
	}

	if err := db.Invites.Create(context.Background(), "existing-invite", 1, "Existing"); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/admin/invites", strings.NewReader(`{"code":"existing-invite","total_uses":2}`))
	rec = httptest.NewRecorder()
	h.CreateInvite(rec, req)
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
		t.Fatalf("duplicate invite should use generic internal error envelope: status=%d body=%q", rec.Code, rec.Body.String())
	}
	existing, err := db.Invites.Get(req.Context(), "existing-invite")
	if err != nil || existing == nil || existing.TotalUses == nil || *existing.TotalUses != 1 || existing.Note != "Existing" {
		t.Fatalf("duplicate invite should not mutate existing row: invite=%#v err=%v", existing, err)
	}
}
