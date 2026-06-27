package integration_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestNoticeHTTPUserAndAdminFlowsExactly(t *testing.T) {
	db, h := testutil.NewTestApp(t)
	admin := testutil.CreateUser(t, db, "notice-http-admin@test.com", "Password123", "NoticeHTTPAdmin", true)
	user := testutil.CreateUser(t, db, "notice-http-user@test.com", "Password123", "NoticeHTTPUser", false)
	adminToken, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, admin.ID, true, time.Hour)
	userToken, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, user.ID, false, time.Hour)
	adminCookie := &http.Cookie{Name: "access_token", Value: adminToken}
	userCookie := &http.Cookie{Name: "access_token", Value: userToken}

	forbiddenAdmin := doJSON(t, h, "GET", "/admin/notices", nil, userCookie)
	if forbiddenAdmin.Code != http.StatusForbidden || forbiddenAdmin.Body.String() != "{\"detail\":\"admin required\"}\n" {
		t.Fatalf("non-admin notice list mismatch: status=%d body=%s", forbiddenAdmin.Code, forbiddenAdmin.Body.String())
	}
	unauthenticated := doJSON(t, h, "GET", "/notices", nil)
	if unauthenticated.Code != http.StatusUnauthorized || unauthenticated.Body.String() != "{\"detail\":\"not authenticated\"}\n" {
		t.Fatalf("unauthenticated notice list mismatch: status=%d body=%s", unauthenticated.Code, unauthenticated.Body.String())
	}

	create := doJSON(t, h, "POST", "/admin/notices", map[string]any{
		"type":             "announcement",
		"title":            "Developer center",
		"summary":          "OAuth application registration is coming.",
		"content_markdown": "Full **OAuth** announcement.",
		"display_mode":     "detail",
		"level":            "warning",
		"link_text":        "Open details",
		"link_url":         "/notifications/dev-center",
		"audience":         "users",
		"enabled":          true,
		"pinned":           true,
		"dismissible":      true,
	}, adminCookie)
	if create.Code != http.StatusOK {
		t.Fatalf("create notice status=%d body=%s", create.Code, create.Body.String())
	}
	created := parseJSON(t, create)
	noticeID := created["id"].(string)
	if noticeID == "" ||
		created["title"] != "Developer center" ||
		created["summary"] != "OAuth application registration is coming." ||
		created["content_markdown"] != "Full **OAuth** announcement." ||
		created["display_mode"] != "detail" ||
		created["level"] != "warning" ||
		created["link_url"] != "/notifications/dev-center" ||
		created["audience"] != "users" ||
		created["enabled"] != true ||
		created["pinned"] != true ||
		created["dismissible"] != true ||
		created["created_by"] != admin.ID {
		t.Fatalf("created notice body mismatch: %#v", created)
	}

	badCreate := doJSON(t, h, "POST", "/admin/notices", map[string]any{
		"title":            "Broken",
		"content_markdown": "Body",
		"display_mode":     "detail",
	}, adminCookie)
	if badCreate.Code != http.StatusBadRequest || badCreate.Body.String() != "{\"detail\":\"summary is required for detail notices\"}\n" {
		t.Fatalf("bad notice create mismatch: status=%d body=%s", badCreate.Code, badCreate.Body.String())
	}
	var brokenCount int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM notices WHERE title='Broken'`).Scan(&brokenCount); err != nil {
		t.Fatal(err)
	}
	if brokenCount != 0 {
		t.Fatalf("invalid create should persist 0 rows, got %d", brokenCount)
	}

	list := doJSON(t, h, "GET", "/notices?type=announcement&dashboard=true&limit=5", nil, userCookie)
	if list.Code != http.StatusOK {
		t.Fatalf("user notice list status=%d body=%s", list.Code, list.Body.String())
	}
	listBody := parseJSON(t, list)
	items := listBody["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("user notice list should contain one item: %#v", listBody)
	}
	item := items[0].(map[string]any)
	if item["id"] != noticeID || item["read"] != false || item["title"] != "Developer center" || item["summary"] != "OAuth application registration is coming." {
		t.Fatalf("user notice list item mismatch: %#v", item)
	}

	detail := doJSON(t, h, "GET", "/notices/"+noticeID, nil, userCookie)
	if detail.Code != http.StatusOK {
		t.Fatalf("notice detail status=%d body=%s", detail.Code, detail.Body.String())
	}
	detailBody := parseJSON(t, detail)
	if detailBody["id"] != noticeID || detailBody["read"] != true || detailBody["read_at"] == nil || detailBody["content_markdown"] != "Full **OAuth** announcement." {
		t.Fatalf("notice detail should mark read and return full body: %#v", detailBody)
	}
	var readCount int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM notice_receipts WHERE notice_id=$1 AND user_id=$2 AND read_at IS NOT NULL`, noticeID, user.ID).Scan(&readCount); err != nil {
		t.Fatal(err)
	}
	if readCount != 1 {
		t.Fatalf("detail should create exactly one read receipt, got %d", readCount)
	}

	read := doJSON(t, h, "POST", "/notices/"+noticeID+"/read", nil, userCookie)
	if read.Code != http.StatusNoContent || read.Body.String() != "" {
		t.Fatalf("mark read mismatch: status=%d body=%s", read.Code, read.Body.String())
	}
	dismiss := doJSON(t, h, "POST", "/notices/"+noticeID+"/dismiss", nil, userCookie)
	if dismiss.Code != http.StatusNoContent || dismiss.Body.String() != "" {
		t.Fatalf("dismiss mismatch: status=%d body=%s", dismiss.Code, dismiss.Body.String())
	}
	afterDismiss := doJSON(t, h, "GET", "/notices?type=announcement&dashboard=true", nil, userCookie)
	if afterDismiss.Code != http.StatusOK {
		t.Fatalf("after dismiss list status=%d body=%s", afterDismiss.Code, afterDismiss.Body.String())
	}
	if dismissedItems := parseJSON(t, afterDismiss)["items"].([]any); len(dismissedItems) != 0 {
		t.Fatalf("dismissed notice should be hidden from dashboard list: %#v", dismissedItems)
	}

	patch := doRawJSON(t, h, "PATCH", "/admin/notices/"+noticeID, `{"title":"Updated notice","summary":"Updated summary","content_markdown":"Updated body","ends_at":null}`, adminCookie)
	if patch.Code != http.StatusOK {
		t.Fatalf("patch notice status=%d body=%s", patch.Code, patch.Body.String())
	}
	patchBody := parseJSON(t, patch)
	if patchBody["title"] != "Updated notice" || patchBody["summary"] != "Updated summary" || patchBody["content_markdown"] != "Updated body" || patchBody["ends_at"] != nil {
		t.Fatalf("patched notice body mismatch: %#v", patchBody)
	}

	adminList := doJSON(t, h, "GET", "/admin/notices?status=enabled", nil, adminCookie)
	if adminList.Code != http.StatusOK {
		t.Fatalf("admin notice list status=%d body=%s", adminList.Code, adminList.Body.String())
	}
	adminItems := parseJSON(t, adminList)["items"].([]any)
	if len(adminItems) != 1 || adminItems[0].(map[string]any)["id"] != noticeID || adminItems[0].(map[string]any)["title"] != "Updated notice" {
		t.Fatalf("admin notice list mismatch: %#v", adminItems)
	}

	del := doJSON(t, h, "DELETE", "/admin/notices/"+noticeID, nil, adminCookie)
	if del.Code != http.StatusNoContent || del.Body.String() != "" {
		t.Fatalf("delete notice mismatch: status=%d body=%s", del.Code, del.Body.String())
	}
	if row, err := db.Notices.Get(context.Background(), noticeID); err != nil || row != nil {
		t.Fatalf("deleted notice should be gone: row=%#v err=%v", row, err)
	}
	var receipts int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM notice_receipts WHERE notice_id=$1`, noticeID).Scan(&receipts); err != nil {
		t.Fatal(err)
	}
	if receipts != 0 {
		t.Fatalf("delete should cascade receipts, got %d", receipts)
	}
	deletedDetail := doJSON(t, h, "GET", "/notices/"+noticeID, nil, userCookie)
	if deletedDetail.Code != http.StatusNotFound || deletedDetail.Body.String() != "{\"detail\":\"notice not found\"}\n" {
		t.Fatalf("deleted detail mismatch: status=%d body=%s", deletedDetail.Code, deletedDetail.Body.String())
	}
}

func TestNoticeHTTPAudienceStatusAndPatchValidationExactly(t *testing.T) {
	db, h := testutil.NewTestApp(t)
	admin := testutil.CreateUser(t, db, "notice-http-audience-admin@test.com", "Password123", "NoticeAudienceAdmin", true)
	user := testutil.CreateUser(t, db, "notice-http-audience-user@test.com", "Password123", "NoticeAudienceUser", false)
	adminToken, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, admin.ID, true, time.Hour)
	userToken, _ := util.CreateAccessToken(testutil.TestConfig().JWTSecret, user.ID, false, time.Hour)
	adminCookie := &http.Cookie{Name: "access_token", Value: adminToken}
	userCookie := &http.Cookie{Name: "access_token", Value: userToken}
	now := database.NowMS()

	adminOnly := doJSON(t, h, "POST", "/admin/notices", map[string]any{
		"title":            "Admin only",
		"content_markdown": "Admin body",
		"audience":         "admins",
	}, adminCookie)
	if adminOnly.Code != http.StatusOK {
		t.Fatalf("admin-only create status=%d body=%s", adminOnly.Code, adminOnly.Body.String())
	}
	adminOnlyID := parseJSON(t, adminOnly)["id"].(string)
	normalDetail := doJSON(t, h, "GET", "/notices/"+adminOnlyID, nil, userCookie)
	if normalDetail.Code != http.StatusNotFound || normalDetail.Body.String() != "{\"detail\":\"notice not found\"}\n" {
		t.Fatalf("normal user should not see admin notice: status=%d body=%s", normalDetail.Code, normalDetail.Body.String())
	}
	adminDetail := doJSON(t, h, "GET", "/notices/"+adminOnlyID, nil, adminCookie)
	if adminDetail.Code != http.StatusOK || parseJSON(t, adminDetail)["id"] != adminOnlyID {
		t.Fatalf("admin should see admin notice: status=%d body=%s", adminDetail.Code, adminDetail.Body.String())
	}

	disabled := doJSON(t, h, "POST", "/admin/notices", map[string]any{
		"title":            "Disabled notice",
		"content_markdown": "Disabled body",
		"enabled":          false,
	}, adminCookie)
	if disabled.Code != http.StatusOK {
		t.Fatalf("disabled create status=%d body=%s", disabled.Code, disabled.Body.String())
	}
	expired := doJSON(t, h, "POST", "/admin/notices", map[string]any{
		"title":            "Expired notice",
		"content_markdown": "Expired body",
		"ends_at":          now - 1,
	}, adminCookie)
	if expired.Code != http.StatusOK {
		t.Fatalf("expired create status=%d body=%s", expired.Code, expired.Body.String())
	}
	scheduled := doJSON(t, h, "POST", "/admin/notices", map[string]any{
		"title":            "Scheduled notice",
		"content_markdown": "Scheduled body",
		"starts_at":        now + 3_600_000,
	}, adminCookie)
	if scheduled.Code != http.StatusOK {
		t.Fatalf("scheduled create status=%d body=%s", scheduled.Code, scheduled.Body.String())
	}
	if items := parseJSON(t, doJSON(t, h, "GET", "/admin/notices?status=disabled", nil, adminCookie))["items"].([]any); len(items) != 1 || items[0].(map[string]any)["title"] != "Disabled notice" {
		t.Fatalf("disabled status list mismatch: %#v", items)
	}
	if items := parseJSON(t, doJSON(t, h, "GET", "/admin/notices?status=expired", nil, adminCookie))["items"].([]any); len(items) != 1 || items[0].(map[string]any)["title"] != "Expired notice" {
		t.Fatalf("expired status list mismatch: %#v", items)
	}
	if items := parseJSON(t, doJSON(t, h, "GET", "/admin/notices?status=scheduled", nil, adminCookie))["items"].([]any); len(items) != 1 || items[0].(map[string]any)["title"] != "Scheduled notice" {
		t.Fatalf("scheduled status list mismatch: %#v", items)
	}

	badStatus := doJSON(t, h, "GET", "/admin/notices?status=bogus", nil, adminCookie)
	if badStatus.Code != http.StatusBadRequest || badStatus.Body.String() != "{\"detail\":\"invalid status\"}\n" {
		t.Fatalf("bad status mismatch: status=%d body=%s", badStatus.Code, badStatus.Body.String())
	}
	badPatch := doRawJSON(t, h, "PATCH", "/admin/notices/"+adminOnlyID, `{"link_url":"javascript:alert(1)","link_text":"Bad"}`, adminCookie)
	if badPatch.Code != http.StatusBadRequest || badPatch.Body.String() != "{\"detail\":\"invalid link_url\"}\n" {
		t.Fatalf("bad patch mismatch: status=%d body=%s", badPatch.Code, badPatch.Body.String())
	}
	row, err := db.Notices.Get(context.Background(), adminOnlyID)
	if err != nil || row == nil || row.LinkURL != "" || row.LinkText != "" {
		t.Fatalf("failed patch must not mutate notice: row=%#v err=%v", row, err)
	}

	for _, raw := range []string{adminOnlyID, parseJSON(t, disabled)["id"].(string), parseJSON(t, expired)["id"].(string), parseJSON(t, scheduled)["id"].(string)} {
		if strings.TrimSpace(raw) == "" {
			t.Fatalf("created notice id should not be empty")
		}
	}
}
