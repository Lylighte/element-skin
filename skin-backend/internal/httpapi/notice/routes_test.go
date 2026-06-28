package notice_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi/notice"
	"element-skin/backend/internal/httpapi/shared"
	noticesvc "element-skin/backend/internal/service/notice"
	"element-skin/backend/internal/testutil"
)

func TestNoticeRoutesListDetailReadDismissExactFlow(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	admin := testutil.CreateUser(t, db, "notice-route-admin@test.com", "Password123", "NoticeRouteAdmin", true)
	user := testutil.CreateUser(t, db, "notice-route-user@test.com", "Password123", "NoticeRouteUser", false)
	svc := noticesvc.Service{DB: db}
	h := notice.New(db, passAuth)

	first, err := svc.Create(t.Context(), noticesvc.CreateInput{
		Title:           "Route notice",
		Summary:         "Route summary",
		ContentMarkdown: "Route **body**",
		DisplayMode:     noticesvc.DisplayDetail,
		Level:           noticesvc.LevelWarning,
	}, admin.ID)
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	h.List(rec, userRequest(http.MethodGet, "/notices?limit=1&include_read=false&type=announcement", user.ID, false))
	if rec.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", rec.Code, rec.Body.String())
	}
	listBody := decodeBody(t, rec)
	items := listBody["items"].([]any)
	if listBody["page_size"] != float64(1) || len(items) != 1 {
		t.Fatalf("list page mismatch: %#v", listBody)
	}
	item := items[0].(map[string]any)
	if item["id"] != first.ID || item["title"] != "Route notice" || item["read"] != false || item["level"] != "warning" {
		t.Fatalf("list item mismatch: %#v", item)
	}

	req := userRequest(http.MethodGet, "/notices/"+first.ID, user.ID, false)
	req.SetPathValue("id", first.ID)
	rec = httptest.NewRecorder()
	h.Detail(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("detail status=%d body=%s", rec.Code, rec.Body.String())
	}
	detail := decodeBody(t, rec)
	if detail["id"] != first.ID || detail["read"] != true || detail["read_at"] == nil || detail["content_markdown"] != "Route **body**" {
		t.Fatalf("detail body mismatch: %#v", detail)
	}

	second, err := svc.Create(t.Context(), noticesvc.CreateInput{Title: "Read me", Summary: "Read summary"}, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	req = userRequest(http.MethodPost, "/notices/"+second.ID+"/read", user.ID, false)
	req.SetPathValue("id", second.ID)
	rec = httptest.NewRecorder()
	h.MarkRead(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.String() != "" {
		t.Fatalf("mark read mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if countReceipt(t, db, second.ID, user.ID, "read_at IS NOT NULL") != 1 {
		t.Fatalf("mark read should create exactly one read receipt")
	}

	third, err := svc.Create(t.Context(), noticesvc.CreateInput{Title: "Dismiss me", Summary: "Dismiss summary"}, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	req = userRequest(http.MethodPost, "/notices/"+third.ID+"/dismiss", user.ID, false)
	req.SetPathValue("id", third.ID)
	rec = httptest.NewRecorder()
	h.Dismiss(rec, req)
	if rec.Code != http.StatusNoContent || rec.Body.String() != "" {
		t.Fatalf("dismiss mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if countReceipt(t, db, third.ID, user.ID, "dismissed_at IS NOT NULL") != 1 {
		t.Fatalf("dismiss should create exactly one dismissed receipt")
	}
	rec = httptest.NewRecorder()
	h.List(rec, userRequest(http.MethodGet, "/notices?include_read=true", user.ID, false))
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "Dismiss me") {
		t.Fatalf("dismissed notice should be hidden from list: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestNoticeRoutesErrorsAndAuthWrapperExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	user := testutil.CreateUser(t, db, "notice-route-error-user@test.com", "Password123", "NoticeRouteErrorUser", false)
	calledAuth := false
	h := notice.New(db, func(next http.HandlerFunc, admin bool) http.HandlerFunc {
		calledAuth = true
		if admin {
			t.Fatalf("notice auth wrapper should not require admin")
		}
		return func(w http.ResponseWriter, req *http.Request) {
			next(w, req.WithContext(shared.WithUser(req.Context(), user.ID, false)))
		}
	})

	rec := httptest.NewRecorder()
	h.Auth(h.List)(rec, httptest.NewRequest(http.MethodGet, "/notices?limit=1", nil))
	if !calledAuth || rec.Code != http.StatusOK {
		t.Fatalf("auth wrapper mismatch: called=%v status=%d body=%q", calledAuth, rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.List(rec, userRequest(http.MethodGet, "/notices?type=bogus", user.ID, false))
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invalid type\"}\n" {
		t.Fatalf("invalid list type mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req := userRequest(http.MethodGet, "/notices/missing", user.ID, false)
	req.SetPathValue("id", "missing")
	rec = httptest.NewRecorder()
	h.Detail(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"notice not found\"}\n" {
		t.Fatalf("missing detail mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func userRequest(method, target, userID string, isAdmin bool) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	return req.WithContext(shared.WithUser(req.Context(), userID, isAdmin))
}

func passAuth(next http.HandlerFunc, _ bool) http.HandlerFunc {
	return next
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
	return body
}

func countReceipt(t *testing.T, db *database.DB, noticeID, userID, predicate string) int {
	t.Helper()
	var count int
	q := fmt.Sprintf(`SELECT COUNT(*) FROM notice_receipts WHERE notice_id=$1 AND user_id=$2 AND %s`, predicate)
	if err := db.Pool.QueryRow(context.Background(), q, noticeID, userID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
