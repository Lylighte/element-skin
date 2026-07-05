package shared

import (
	"context"
	"net/http"

	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

type AuthFunc func(http.HandlerFunc, ...permission.Definition) http.HandlerFunc

type ctxKey string

const (
	actorKey ctxKey = "actor"
)

func WithActor(ctx context.Context, actor permission.Actor) context.Context {
	return context.WithValue(ctx, actorKey, actor)
}

func WithActorPermissions(ctx context.Context, userID string, definitions ...permission.Definition) context.Context {
	bits := permission.NewBitSet(len(permission.Definitions))
	for _, def := range definitions {
		bits.Set(def.BitIndex)
	}
	return WithActor(ctx, permission.Actor{
		SubjectID:   "user:" + userID,
		UserID:      userID,
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointDashboard,
		Permissions: bits,
	})
}

func CurrentActor(req *http.Request) permission.Actor {
	actor, _ := req.Context().Value(actorKey).(permission.Actor)
	return actor
}

func CurrentUserID(req *http.Request) string {
	return CurrentActor(req).UserID
}

func RequirePermission(req *http.Request, def permission.Definition) error {
	if CurrentActor(req).Has(def) {
		return nil
	}
	return util.HTTPError{Status: http.StatusForbidden, Detail: "permission denied"}
}
