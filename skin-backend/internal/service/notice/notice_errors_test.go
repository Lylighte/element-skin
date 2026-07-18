package notice_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/permission"
	noticesvc "element-skin/backend/internal/service/notice"
	"element-skin/backend/internal/testutil"
)

func TestNoticeServicePropagatesDatabaseErrorsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	svc := noticesvc.Service{DB: db}
	ctx := context.Background()
	db.Close()
	user := noticeActor("notice-db-error-user", "notice.read.owned", "notice.dismiss.owned")
	actor := noticeManagerActor("admin-id")

	if _, err := svc.GetForUser(ctx, "notice-id", user); err == nil || err.Error() != "closed pool" {
		t.Fatalf("GetForUser database error=%v; want closed pool", err)
	}
	if err := svc.MarkRead(ctx, "notice-id", user); err == nil || err.Error() != "closed pool" {
		t.Fatalf("MarkRead database error=%v; want closed pool", err)
	}
	if err := svc.Dismiss(ctx, "notice-id", user); err == nil || err.Error() != "closed pool" {
		t.Fatalf("Dismiss database error=%v; want closed pool", err)
	}
	if _, err := svc.Create(ctx, actor, noticesvc.CreateInput{Title: "DB Error", Summary: "summary"}); err == nil || err.Error() != "closed pool" {
		t.Fatalf("Create database error=%v; want closed pool", err)
	}
	if _, err := svc.Patch(ctx, actor, "notice-id", noticesvc.PatchInput{Title: ptrString("DB Error")}); err == nil || err.Error() != "closed pool" {
		t.Fatalf("Patch database error=%v; want closed pool", err)
	}
	if err := svc.Delete(ctx, actor, "notice-id"); err == nil || err.Error() != "closed pool" {
		t.Fatalf("Delete database error=%v; want closed pool", err)
	}
	if err := svc.DeleteExpired(ctx, permission.SystemMaintenanceActor(), database.NowMS()); err == nil || err.Error() != "closed pool" {
		t.Fatalf("DeleteExpired database error=%v; want closed pool", err)
	}
}
