package permission_test

import (
	"testing"

	"element-skin/backend/internal/permission"
)

func TestActorHasExactly(t *testing.T) {
	bits := permission.NewBitSet(len(permission.Definitions))
	def := permission.MustDefinitionByCode("profile.create.owned")
	bits.Set(def.BitIndex)
	actor := permission.Actor{Permissions: bits}

	if !actor.Has(def) {
		t.Fatal("actor should have permission when bit is set")
	}
	otherDef := permission.MustDefinitionByCode("notice.create.any")
	if actor.Has(otherDef) {
		t.Fatal("actor should not have permission when bit is not set")
	}
}

func TestActorRequireExactly(t *testing.T) {
	bits := permission.NewBitSet(len(permission.Definitions))
	def := permission.MustDefinitionByCode("texture.delete.owned")
	bits.Set(def.BitIndex)
	actor := permission.Actor{Permissions: bits}

	if err := actor.Require(def); err != nil {
		t.Fatalf("Require should not return error when bit is set: %v", err)
	}
	otherDef := permission.MustDefinitionByCode("account.ban.any")
	if err := actor.Require(otherDef); err != permission.ErrForbidden {
		t.Fatalf("Require should return ErrForbidden when bit is not set: %v", err)
	}
}

func TestActorPermissionCodesExactly(t *testing.T) {
	bits := permission.NewBitSet(len(permission.Definitions))
	codes := []string{"profile.read.owned", "texture.create.owned"}
	for _, code := range codes {
		bits.Set(permission.MustDefinitionByCode(code).BitIndex)
	}
	actor := permission.Actor{
		SubjectID:         "user:test-1",
		UserID:            "test-1",
		SessionKind:       permission.SessionKindWeb,
		Entrypoint:        permission.EntrypointDashboard,
		DelegationID:      "grant-abc",
		DelegatedClientID: "client-xyz",
		Permissions:       bits,
	}

	got := actor.PermissionCodes()
	if len(got) != 2 || got[0] != "profile.read.owned" || got[1] != "texture.create.owned" {
		t.Fatalf("PermissionCodes mismatch: %#v", got)
	}
}

func TestActorPermissionCodesEmpty(t *testing.T) {
	actor := permission.Actor{
		UserID:      "empty-user",
		Permissions: permission.NewBitSet(len(permission.Definitions)),
	}
	got := actor.PermissionCodes()
	if len(got) != 0 {
		t.Fatalf("empty actor should return empty codes: %#v", got)
	}
}

func TestSystemMaintenanceActorExactly(t *testing.T) {
	actor := permission.SystemMaintenanceActor()
	if actor.SubjectID != "system:maintenance" ||
		actor.UserID != "" ||
		actor.SessionKind != permission.SessionKindSystem ||
		actor.Entrypoint != permission.EntrypointMaintenance {
		t.Fatalf("system maintenance actor identity mismatch: %#v", actor)
	}
	got := actor.PermissionCodes()
	want := []string{
		"notice.create.system",
		"notice.delete.system",
		"yggdrasil_session.delete.system",
		"audit.archive.system",
		"cache.invalidate.system",
		"oauth_grant.delete.system",
	}
	if len(got) != len(want) {
		t.Fatalf("system maintenance permissions=%#v; want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("system maintenance permission[%d]=%q; want %q in %#v", i, got[i], want[i], got)
		}
	}
	if actor.Has(permission.MustDefinitionByCode("notice.create.any")) {
		t.Fatal("system maintenance actor must not include non-system notice create permission")
	}
}
