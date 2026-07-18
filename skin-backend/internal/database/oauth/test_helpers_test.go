package oauth_test

import (
	"context"
	"errors"
	"sort"
	"testing"

	"element-skin/backend/internal/database"
	core "element-skin/backend/internal/permission"

	"github.com/jackc/pgx/v5/pgconn"
)

func permissionIDs(codes ...string) []int64 {
	ids := make([]int64, 0, len(codes))
	for _, code := range codes {
		ids = append(ids, int64(core.MustDefinitionByCode(code).ID))
	}
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})
	return ids
}

func assertOAuthGrantExists(t *testing.T, db *database.DB, grantID string, want bool) {
	t.Helper()
	var count int
	if err := db.Pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM delegated_permission_grants WHERE id=$1
	`, grantID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if got := count == 1; got != want {
		t.Fatalf("grant %s exists=%v want %v", grantID, got, want)
	}
}

func assertOAuthDependencyCount(t *testing.T, db *database.DB, grantID string, want int) {
	t.Helper()
	var grantPermissions, refreshTokens, codes, codePermissions int
	if err := db.Pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM delegated_grant_permissions WHERE grant_id=$1
	`, grantID).Scan(&grantPermissions); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM oauth_refresh_tokens WHERE grant_id=$1
	`, grantID).Scan(&refreshTokens); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM oauth_authorization_codes WHERE grant_id=$1
	`, grantID).Scan(&codes); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(context.Background(), `
		SELECT COUNT(*)
		FROM oauth_authorization_code_permissions
		WHERE code_hash IN (SELECT code_hash FROM oauth_authorization_codes WHERE grant_id=$1)
	`, grantID).Scan(&codePermissions); err != nil {
		t.Fatal(err)
	}
	got := grantPermissions + refreshTokens + codes + codePermissions
	if got != want {
		t.Fatalf("grant %s dependency count=%d want %d (grant_permissions=%d refresh=%d codes=%d code_permissions=%d)",
			grantID, got, want, grantPermissions, refreshTokens, codes, codePermissions)
	}
}

func assertPgCode(t *testing.T, err error, code string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != code {
		t.Fatalf("PostgreSQL error code mismatch: err=%v code=%q want %q", err, pgErrCode(pgErr), code)
	}
}

func pgErrCode(err *pgconn.PgError) string {
	if err == nil {
		return ""
	}
	return err.Code
}

func assertPermissionSubjectAbsent(t *testing.T, db *database.DB, subjectID string) {
	t.Helper()
	var count int
	if err := db.Pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM permission_subjects WHERE id=$1
	`, subjectID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("permission subject %q count=%d, want 0", subjectID, count)
	}
}
