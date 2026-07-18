package microsoft_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"element-skin/backend/internal/httpapi/microsoft"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/testutil"
)

func TestHandlerAuthRequestsUserAccessAndKeepsStateStore(t *testing.T) {
	var required []permission.Definition
	states := redisstore.NewMemoryStore()
	h := microsoft.New(testutil.TestConfig(), nil, settings.Settings{Redis: testutil.NewMemoryRedis()}, func(next http.HandlerFunc, defs ...permission.Definition) http.HandlerFunc {
		required = defs
		return next
	}, states)
	wrapped := h.Auth(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	rec := httptest.NewRecorder()
	wrapped(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusNoContent || len(required) != 0 {
		t.Fatalf("microsoft Auth required permissions mismatch: status=%d required=%d", rec.Code, len(required))
	}
}
