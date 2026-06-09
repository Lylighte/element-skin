package site_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/site"
	"element-skin/backend/internal/redisstore"
	sitesvc "element-skin/backend/internal/service/site"
	"element-skin/backend/internal/testutil"
)

func TestPublicRoutesCarouselListsOnlyImagesExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.CarouselDir = t.TempDir()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	if err := os.WriteFile(cfg.CarouselDir+"\\hero.webp", []byte("img"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.CarouselDir+"\\notes.txt", []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/public/carousel", nil)
	rec := httptest.NewRecorder()
	h.PublicCarousel(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "[\"hero.webp\"]\n" {
		t.Fatalf("public carousel should list only images exactly: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestPublicRoutesRedisErrorDoesNotFallback(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cache := redisstore.NewMemoryStore()
	cache.Err = errors.New("redis down")
	h := site.NewWithRedis(cfg, db, cache, sitesvc.Site{DB: db, Cfg: cfg, Redis: cache}, nil)

	rec := httptest.NewRecorder()
	h.PublicSettings(rec, httptest.NewRequest(http.MethodGet, "/public/settings", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("public settings redis error should fail, got %d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.PublicCarousel(rec, httptest.NewRequest(http.MethodGet, "/public/carousel", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("public carousel redis error should fail, got %d body=%q", rec.Code, rec.Body.String())
	}
}

func TestPublicRoutesSettingsAndLibraryExactResponses(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	user := testutil.CreateUser(t, db, "public-routes@test.com", "Password123", "PublicRoutes", false)
	if err := db.Textures.AddToLibrary(t.Context(), user.ID, "public_route_hash", "skin", "Public Route Texture", true, "default"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/public/settings", nil)
	rec := httptest.NewRecorder()
	h.PublicSettings(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"site_name"`) || !strings.Contains(rec.Body.String(), `"enable_skin_library"`) || !strings.Contains(rec.Body.String(), `"easter_eggs"`) {
		t.Fatalf("public settings response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/public/skin-library?texture_type=skin&q=Public%20Route", nil)
	rec = httptest.NewRecorder()
	h.PublicLibrary(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"hash":"public_route_hash"`) || !strings.Contains(rec.Body.String(), `"name":"Public Route Texture"`) {
		t.Fatalf("public library response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}
