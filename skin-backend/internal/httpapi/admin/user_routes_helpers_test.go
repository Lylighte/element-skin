package admin_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"

	"github.com/jackc/pgx/v5"
)

var (
	testProtectedPermission = permission.MustDefinitionByCode("permission_protected.manage.any")
)

func withAdminActor(req *http.Request, userID string) *http.Request {
	return req.WithContext(shared.WithActorPermissions(req.Context(), userID, rolePermissions(permission.RoleAdmin)...))
}

func withProtectedActor(req *http.Request, userID string) *http.Request {
	defs := append([]permission.Definition{}, rolePermissions(permission.RoleAdmin)...)
	defs = append(defs, testProtectedPermission)
	return req.WithContext(shared.WithActorPermissions(req.Context(), userID, defs...))
}

func withAdminActorWithoutPermission(req *http.Request, userID, excludeCode string) *http.Request {
	defs := make([]permission.Definition, 0, len(rolePermissions(permission.RoleAdmin)))
	for _, def := range rolePermissions(permission.RoleAdmin) {
		if def.Code != excludeCode {
			defs = append(defs, def)
		}
	}
	return req.WithContext(shared.WithActorPermissions(req.Context(), userID, defs...))
}

func rolePermissions(roleID string) []permission.Definition {
	for _, role := range permission.Roles {
		if role.ID == roleID {
			return role.Permissions
		}
	}
	panic("missing role: " + roleID)
}

func waitForBlockedAdminMutation(
	t *testing.T,
	db interface {
		QueryRow(context.Context, string, ...any) pgx.Row
	},
	lockHolderPID int,
	result <-chan *httptest.ResponseRecorder,
) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		select {
		case rec := <-result:
			t.Fatalf("admin mutation completed before row-lock release: status=%d body=%q", rec.Code, rec.Body.String())
		default:
		}
		var waiting bool
		if err := db.QueryRow(t.Context(), `
			SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity
				WHERE $1 = ANY(pg_blocking_pids(pid))
			)
		`, lockHolderPID).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("admin mutation did not reach the expected row-lock wait")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

type authInvalidateFailRedis struct {
	redisstore.Store
	failAt  int
	userIDs []string
}

type repopulateDuringTransferRedis struct {
	redisstore.Store
	oldUsers map[string]redisstore.AuthUser
	userIDs  []string
}

func (r *repopulateDuringTransferRedis) InvalidateAuthUser(ctx context.Context, userID string) error {
	r.userIDs = append(r.userIDs, userID)
	if err := r.Store.InvalidateAuthUser(ctx, userID); err != nil {
		return err
	}
	if len(r.userIDs) == 2 {
		for _, oldUser := range r.oldUsers {
			if err := r.Store.SetAuthUser(ctx, oldUser, time.Minute); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *authInvalidateFailRedis) InvalidateAuthUser(ctx context.Context, userID string) error {
	r.userIDs = append(r.userIDs, userID)
	if len(r.userIDs) == r.failAt {
		return errors.New("auth cache invalidation failed")
	}
	return r.Store.InvalidateAuthUser(ctx, userID)
}

func strconvI64(v int64) string {
	return strconv.FormatInt(v, 10)
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
