package texture_test

import (
	"context"
	"strings"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/permission"
	settingssvc "element-skin/backend/internal/service/settings"
	texturesvc "element-skin/backend/internal/service/texture"
	"element-skin/backend/internal/testutil"
)

func ptrString(s string) *string {
	return &s
}

func texturePublicActor() permission.Actor {
	return permission.GuestActor()
}

func assertServicePublicUsage(t *testing.T, svc texturesvc.LibraryService, hash string, want int64) {
	t.Helper()
	public, err := svc.PublicLibrary(context.Background(), texturePublicActor(), "", 10, "skin", hash, "most_used")
	if err != nil {
		t.Fatalf("PublicLibrary(%s): %v", hash, err)
	}
	items := public["items"].([]map[string]any)
	if len(items) != 1 || items[0]["hash"] != hash || items[0]["usage_count"] != want {
		t.Fatalf("PublicLibrary usage for %s = %#v; want usage_count=%d", hash, public, want)
	}
}

func newLibraryService(db *database.DB) texturesvc.LibraryService {
	redis := testutil.NewMemoryRedis()
	return texturesvc.LibraryService{DB: db, Settings: settingssvc.Settings{DB: db, Redis: redis}}
}

func textureUserActor(userID string) permission.Actor {
	bits := permission.NewBitSet(len(permission.Definitions))
	for _, role := range permission.Roles {
		if role.ID != permission.RoleUser {
			continue
		}
		for _, def := range role.Permissions {
			bits.Set(def.BitIndex)
		}
	}
	return permission.Actor{
		SubjectID:   "user:" + userID,
		UserID:      userID,
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointDashboard,
		Permissions: bits,
	}
}

func closedPoolError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "closed pool")
}
