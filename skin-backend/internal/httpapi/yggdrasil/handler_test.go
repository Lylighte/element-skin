package yggdrasil_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/shared"
	ygghttp "element-skin/backend/internal/httpapi/yggdrasil"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/service/settings"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestNewStoresConfigAndServicesByValue(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	h := ygghttp.New(cfg, db, redis, settings.Settings{DB: db, Redis: redis}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.Metadata(rec, req.WithContext(shared.WithActor(req.Context(), permission.GuestActor())))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "element-skin") {
		t.Fatalf("metadata via constructed handler mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}
