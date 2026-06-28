package site_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"element-skin/backend/internal/httpapi/site"
	"element-skin/backend/internal/permission"
	sitepkg "element-skin/backend/internal/service/site"
	"element-skin/backend/internal/testutil"
)

func TestHandlerAuthRequestsUserAccess(t *testing.T) {
	var required []permission.Definition
	h := site.New(testutil.TestConfig(), nil, sitepkg.Site{}, func(next http.HandlerFunc, defs ...permission.Definition) http.HandlerFunc {
		required = defs
		return next
	})
	wrapped := h.Auth(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	rec := httptest.NewRecorder()
	wrapped(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusNoContent || len(required) != 0 {
		t.Fatalf("site Auth required permissions mismatch: status=%d required=%d", rec.Code, len(required))
	}
}
