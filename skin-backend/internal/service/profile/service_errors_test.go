package profile_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/testutil"
)

func TestProfileServiceClosedDatabaseReturnsExactDependencyErrors(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newProfileService(db)
	actor := testUserActor("closed-profile-user")
	db.Close()

	checks := []struct {
		name string
		call func() error
	}{
		{name: "create profile", call: func() error {
			result, err := svc.CreateProfile(ctx, actor, "ClosedProfile", "default")
			if result != nil {
				t.Fatalf("CreateProfile closed database returned result=%#v", result)
			}
			return err
		}},
		{name: "list profiles", call: func() error {
			result, err := svc.ListMyProfiles(ctx, actor, "", 10)
			if result != nil {
				t.Fatalf("ListMyProfiles closed database returned result=%#v", result)
			}
			return err
		}},
		{name: "update profile", call: func() error {
			return svc.UpdateProfile(ctx, actor, "closed-profile", "ClosedProfile")
		}},
		{name: "delete profile", call: func() error {
			return svc.DeleteProfile(ctx, actor, "closed-profile")
		}},
		{name: "clear profile texture", call: func() error {
			return svc.ClearProfileTexture(ctx, actor, "closed-profile", "skin")
		}},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); !closedPoolError(err) {
				t.Fatalf("%s closed database error=%v; want closed pool", tc.name, err)
			}
		})
	}
}

func TestProfileServiceAdminClosedDatabaseReturnsExactDependencyErrors(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newProfileService(db)
	actor := testActorWithCodes("closed-profile-admin", "profile.read.any", "profile.update.any", "profile.delete.any")
	db.Close()

	checks := []struct {
		name string
		call func() error
	}{
		{name: "list all profiles", call: func() error {
			result, err := svc.ListAllProfiles(ctx, actor, "", 10, "")
			if result != nil {
				t.Fatalf("ListAllProfiles closed database returned result=%#v", result)
			}
			return err
		}},
		{name: "list profiles by user", call: func() error {
			result, err := svc.ListProfilesByUser(ctx, actor, "closed-profile-user", "", 10)
			if result != nil {
				t.Fatalf("ListProfilesByUser closed database returned result=%#v", result)
			}
			return err
		}},
		{name: "update any profile", call: func() error {
			return svc.UpdateAnyProfile(ctx, actor, "closed-profile", "ClosedProfile")
		}},
		{name: "delete profile by id", call: func() error {
			return svc.DeleteProfileByID(ctx, actor, "closed-profile")
		}},
		{name: "set profile texture", call: func() error {
			hash := "closed-profile-skin"
			return svc.SetProfileTexture(ctx, actor, "closed-profile", "skin", &hash)
		}},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); !closedPoolError(err) {
				t.Fatalf("%s closed database error=%v; want closed pool", tc.name, err)
			}
		})
	}
}
