package imports

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"

	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/util"
)

type microsoftSettingsStub struct {
	values map[string]string
	err    error
}

func (s microsoftSettingsStub) Get(_ context.Context, key, fallback string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	if value, ok := s.values[key]; ok {
		return value, nil
	}
	return fallback, nil
}

func TestMicrosoftWorkflowStateKindsOwnershipAndConsumptionExactly(t *testing.T) {
	ctx := context.Background()
	states := redisstore.NewMemoryStore()
	workflow := MicrosoftImportWorkflow{States: states}
	if err := states.SetState(ctx, "token", map[string]any{
		"kind": microsoftStateKindProfile, "user_id": "user-id",
	}, time.Minute); err != nil {
		t.Fatal(err)
	}

	session, err := workflow.popState(ctx, "token", microsoftStateKindProfile, "invalid")
	if err != nil || !reflect.DeepEqual(session, map[string]any{"kind": microsoftStateKindProfile, "user_id": "user-id"}) {
		t.Fatalf("pop state=(%#v, %v), want exact session", session, err)
	}
	_, err = workflow.popState(ctx, "token", microsoftStateKindProfile, "invalid")
	assertMicrosoftHTTPError(t, err, http.StatusBadRequest, "invalid")
	if err := requireMicrosoftStateOwner(session, "user-id", "denied"); err != nil {
		t.Fatalf("matching owner error=%v, want nil", err)
	}
	assertMicrosoftHTTPError(t, requireMicrosoftStateOwner(session, "other", "denied"), http.StatusForbidden, "denied")
}

func TestMicrosoftWorkflowRejectsMissingPermissionsBeforeStateMutationExactly(t *testing.T) {
	ctx := context.Background()
	states := redisstore.NewMemoryStore()
	workflow := MicrosoftImportWorkflow{
		APIURL:   "https://api.example",
		SiteURL:  "https://site.example",
		Settings: microsoftSettingsStub{values: map[string]string{"microsoft_client_id": "client-id"}},
		States:   states,
	}
	actor := permission.Actor{UserID: "user-id", Permissions: permission.NewBitSet(len(permission.Definitions))}

	start, err := workflow.Start(ctx, actor)
	assertMicrosoftHTTPError(t, err, http.StatusForbidden, "permission denied")
	if start != (MicrosoftAuthStart{}) || states.Len() != 0 {
		t.Fatalf("denied start=(%#v, states=%d), want zero result and no state", start, states.Len())
	}
	for _, token := range []string{"profile-token", "import-token"} {
		if err := states.SetState(ctx, token, map[string]any{"kind": "untouched"}, time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	preview, err := workflow.Preview(ctx, actor, "profile-token")
	assertMicrosoftHTTPError(t, err, http.StatusForbidden, "permission denied")
	if !reflect.DeepEqual(preview, MicrosoftProfilePreview{}) {
		t.Fatalf("denied preview=%#v, want zero result", preview)
	}
	result, err := workflow.Import(ctx, actor, "import-token")
	assertMicrosoftHTTPError(t, err, http.StatusForbidden, "permission denied")
	if result != nil || states.Len() != 2 {
		t.Fatalf("denied import=(%#v, states=%d), want nil and untouched states", result, states.Len())
	}
}

func TestMicrosoftWorkflowRequiresCompleteConfigurationBeforeCreatingStateExactly(t *testing.T) {
	actor := microsoftActorWithPermissions("user-id", microsoftImportStartPermission)
	states := redisstore.NewMemoryStore()
	workflow := MicrosoftImportWorkflow{
		APIURL:   "https://api.example",
		SiteURL:  "https://site.example",
		Settings: microsoftSettingsStub{values: map[string]string{}},
		States:   states,
	}

	result, err := workflow.Start(context.Background(), actor)
	assertMicrosoftHTTPError(t, err, http.StatusServiceUnavailable, "Microsoft import is not configured")
	if result != (MicrosoftAuthStart{}) || states.Len() != 0 {
		t.Fatalf("unconfigured start=(%#v, states=%d), want zero result and no state", result, states.Len())
	}
}

func TestMicrosoftWorkflowPropagatesSettingsFailureBeforeCreatingStateExactly(t *testing.T) {
	want := errors.New("settings unavailable")
	states := redisstore.NewMemoryStore()
	workflow := MicrosoftImportWorkflow{Settings: microsoftSettingsStub{err: want}, States: states}
	_, err := workflow.Start(context.Background(), microsoftActorWithPermissions("user-id", microsoftImportStartPermission))
	if !errors.Is(err, want) || states.Len() != 0 {
		t.Fatalf("start error=%v states=%d, want exact settings error and no state", err, states.Len())
	}
}

func microsoftActorWithPermissions(userID string, defs ...permission.Definition) permission.Actor {
	bits := permission.NewBitSet(len(permission.Definitions))
	for _, def := range defs {
		bits.Set(def.BitIndex)
	}
	return permission.Actor{UserID: userID, Permissions: bits}
}

func assertMicrosoftHTTPError(t *testing.T, err error, status int, detail string) {
	t.Helper()
	var httpErr util.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != status || httpErr.Detail != detail {
		t.Fatalf("error=%#v, want HTTPError{%d, %q}", err, status, detail)
	}
}
