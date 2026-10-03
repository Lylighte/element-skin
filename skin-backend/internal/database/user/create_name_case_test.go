package user_test

import (
	"reflect"
	"testing"

	profilestore "element-skin/backend/internal/database/profile"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
)

func TestCreateWithProfileCaseConflictPreservesUsersProfilesAndInvite(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := t.Context()
	owner := testutil.CreateUser(t, db, "create-case-owner@test.com", "Password123", "CreateCaseOwner", false)
	reserved := testutil.CreateProfile(t, db, owner.ID, "create_case_reserved", "CasePlayer")
	if err := db.Invites.Create(ctx, "CASE_INVITE", testutil.Pointer(1), "case conflict"); err != nil {
		t.Fatal(err)
	}
	before, err := db.Invites.Get(ctx, "CASE_INVITE")
	if err != nil || before == nil || before.UsedCount != 0 || before.UsedBy != nil {
		t.Fatalf("initial invite=%#v err=%v", before, err)
	}
	rejected := model.User{ID: "create_case_rejected_user", Email: "create-case-rejected@test.com", Password: "hash", DisplayName: "CaseRejected"}
	profile := model.Profile{ID: "create_case_rejected_profile", UserID: rejected.ID, Name: "caseplayer", TextureModel: "default"}
	if err := db.Users.CreateWithProfile(ctx, rejected, profile, "CASE_INVITE", rejected.Email); !profilestore.IsNameConflict(err) {
		t.Fatalf("case conflict error=%v; want profile name uniqueness violation", err)
	}
	if got, err := db.Users.GetByID(ctx, rejected.ID); err != nil || got != nil {
		t.Fatalf("rejected creation left user=%#v err=%v", got, err)
	}
	if got, err := db.Users.GetByEmail(ctx, rejected.Email); err != nil || got != nil {
		t.Fatalf("rejected creation left email=%#v err=%v", got, err)
	}
	if got, err := db.Profiles.GetByID(ctx, profile.ID); err != nil || got != nil {
		t.Fatalf("rejected creation left profile=%#v err=%v", got, err)
	}
	if got, err := db.Profiles.GetByID(ctx, reserved.ID); err != nil || !reflect.DeepEqual(got, &reserved) {
		t.Fatalf("rejected creation changed reserved profile=%#v err=%v", got, err)
	}
	if after, err := db.Invites.Get(ctx, "CASE_INVITE"); err != nil || !reflect.DeepEqual(after, before) {
		t.Fatalf("rejected creation consumed invite=%#v err=%v; want %#v", after, err, before)
	}
	if count, err := db.Users.Count(ctx); err != nil || count != 1 {
		t.Fatalf("rejected creation changed user count=%d err=%v; want 1", count, err)
	}
}
