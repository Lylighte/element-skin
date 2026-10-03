package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/testutil"
)

func TestProtocolLookupAliasesReturnOneCanonicalProfileForCaseVariants(t *testing.T) {
	db, handler := testutil.NewTestApp(t)
	owner := testutil.CreateUser(t, db, "protocol-case@test.com", "Password123", "ProtocolCase", false)
	profile := testutil.CreateProfile(t, db, owner.ID, "0123456789abcdef0123456789abcdef", "CasePlayer")
	want := `{"id":"` + profile.ID + `","name":"CasePlayer"}`
	for _, path := range []string{
		"/api/users/profiles/minecraft/caseplayer",
		"/users/profiles/minecraft/CASEPLAYER",
		"/api/profiles/minecraft/caseplayer",
		"/api/minecraft/profile/lookup/name/CASEPLAYER",
		"/minecraft/profile/lookup/name/caseplayer",
		"/minecraftservices/minecraft/profile/lookup/name/caseplayer",
	} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			if rec.Code != http.StatusOK || rec.Body.String() != want+"\n" {
				t.Fatalf("lookup status=%d body=%q; want canonical profile %s", rec.Code, rec.Body.String(), want)
			}
		})
	}
	for _, path := range []string{
		"/api/profiles/minecraft",
		"/api/minecraft/profile/lookup/bulk/byname",
		"/minecraftservices/minecraft/profile/lookup/bulk/byname",
	} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`["caseplayer","CASEPLAYER"]`)))
			if rec.Code != http.StatusOK || rec.Body.String() != "["+want+"]\n" {
				t.Fatalf("bulk lookup status=%d body=%q; want one canonical profile", rec.Code, rec.Body.String())
			}
		})
	}
}
