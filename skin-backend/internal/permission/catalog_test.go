package permission_test

import (
	"strings"
	"testing"

	"element-skin/backend/internal/permission"
)

func TestCatalogCodesAreStrictTriplesAndIDsMatchParts(t *testing.T) {
	seenCodes := map[string]struct{}{}
	seenBits := map[int]struct{}{}
	seenIDs := map[permission.ID]struct{}{}
	for _, def := range permission.Definitions {
		if parts := strings.Split(def.Code, "."); len(parts) != 3 {
			t.Fatalf("permission code %q is not a strict triple", def.Code)
		}
		wantCode := def.Resource.Code + "." + def.Action.Code + "." + def.Scope.Code
		if def.Code != wantCode {
			t.Fatalf("code mismatch: got %q want %q", def.Code, wantCode)
		}
		if !def.ID.Valid() || def.ID.ResourceID() != def.Resource.ID || def.ID.ActionID() != def.Action.ID || def.ID.ScopeID() != def.Scope.ID {
			t.Fatalf("id mismatch for %s: id=%#x resource=%d/%d action=%d/%d scope=%d/%d", def.Code, uint64(def.ID), def.ID.ResourceID(), def.Resource.ID, def.ID.ActionID(), def.Action.ID, def.ID.ScopeID(), def.Scope.ID)
		}
		if _, ok := seenCodes[def.Code]; ok {
			t.Fatalf("duplicate code %q", def.Code)
		}
		seenCodes[def.Code] = struct{}{}
		if _, ok := seenIDs[def.ID]; ok {
			t.Fatalf("duplicate id %#x", uint64(def.ID))
		}
		seenIDs[def.ID] = struct{}{}
		if _, ok := seenBits[def.BitIndex]; ok {
			t.Fatalf("duplicate bit index %d", def.BitIndex)
		}
		seenBits[def.BitIndex] = struct{}{}
	}
	if len(permission.Definitions) != len(seenBits) {
		t.Fatalf("bit indexes should be dense count=%d seen=%d", len(permission.Definitions), len(seenBits))
	}
	for i := range permission.Definitions {
		if _, ok := seenBits[i]; !ok {
			t.Fatalf("missing dense bit index %d", i)
		}
	}
	if _, ok := seenCodes["permission_protected.manage.any"]; !ok {
		t.Fatalf("permission_protected.manage.any is missing")
	}
	if _, ok := seenCodes["texture.update_visibility.owned"]; !ok {
		t.Fatalf("texture.update_visibility.owned is missing")
	}
	if _, ok := seenCodes["yggdrasil_server.join.bound_profile"]; !ok {
		t.Fatalf("yggdrasil_server.join.bound_profile is missing")
	}
}
