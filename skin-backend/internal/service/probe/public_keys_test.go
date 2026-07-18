package probe_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	fallbacksvc "element-skin/backend/internal/service/fallback"
	"element-skin/backend/internal/service/probe"
	"element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/testutil"
)

func TestProbePublicKeysDiscoverySuccessSkipsServicesFallbackAndDeduplicates(t *testing.T) {
	fixture := testutil.NewPublicKeyFixture(t)
	server := newPublicKeyProbeServer(t, fixture, true)
	defer server.Close()
	db, _, redis := testutil.NewTestAppWithRedisTB(t)
	ctx := context.Background()
	saveProbeEndpoints(t, db, redis, server.URL, 2)

	svc := probe.New(db, redis)
	svc.Client = server.Client()
	if err := svc.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if got := server.callsFor("/"); got != 1 {
		t.Fatalf("discovery calls=%d, want 1 for duplicate public-key source", got)
	}
	if got := server.callsFor("/publickeys/"); got != 0 {
		t.Fatalf("services public-key calls=%d, want 0 after successful discovery", got)
	}
	assertCachedPublicKeys(t, ctx, db, redis, model.YggdrasilPublicKeys{
		ProfilePropertyKeys:   []model.YggdrasilPublicKey{{PublicKey: fixture.DERBase64}},
		PlayerCertificateKeys: []model.YggdrasilPublicKey{{PublicKey: fixture.DERBase64}},
	})
}

func TestProbePublicKeysFallsBackToServicesWithoutChangingHealthCheck(t *testing.T) {
	fixture := testutil.NewPublicKeyFixture(t)
	server := newPublicKeyProbeServer(t, fixture, false)
	defer server.Close()
	db, _, redis := testutil.NewTestAppWithRedisTB(t)
	ctx := context.Background()
	saveProbeEndpoints(t, db, redis, server.URL, 1)

	svc := probe.New(db, redis)
	svc.Client = server.Client()
	checkedAt := time.Now().Truncate(time.Millisecond)
	svc.Now = func() time.Time { return checkedAt }
	if err := svc.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if got := server.callsFor("/"); got != 1 {
		t.Fatalf("discovery calls=%d, want 1", got)
	}
	if got := server.callsFor("/publickeys/"); got != 1 {
		t.Fatalf("services public-key calls=%d, want 1 after discovery parse failure", got)
	}
	if got := server.callsFor("/minecraft/profile/lookup/name/" + probe.TestName); got != 1 {
		t.Fatalf("services health calls=%d, want unchanged lookup request", got)
	}
	samples, err := redis.GetProbeHistory(ctx, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 1 || samples[0].Services != probe.StatusUp || samples[0].CheckedAt != checkedAt.UnixMilli() {
		t.Fatalf("services health sample mismatch: %#v", samples)
	}
	assertCachedPublicKeys(t, ctx, db, redis, model.YggdrasilPublicKeys{
		ProfilePropertyKeys:   []model.YggdrasilPublicKey{{PublicKey: fixture.DERBase64}},
		PlayerCertificateKeys: []model.YggdrasilPublicKey{{PublicKey: fixture.DERBase64}},
	})
}

func TestProbePublicKeysFailurePreservesPreviousCache(t *testing.T) {
	fixture := testutil.NewPublicKeyFixture(t)
	server := newPublicKeyProbeServer(t, fixture, false)
	server.failServicesPublicKeys = true
	defer server.Close()
	db, _, redis := testutil.NewTestAppWithRedisTB(t)
	ctx := context.Background()
	saveProbeEndpoints(t, db, redis, server.URL, 1)
	endpoints, err := db.Fallbacks.ListEndpoints(ctx)
	if err != nil {
		t.Fatal(err)
	}
	source := fallbacksvc.PublicKeySources(endpoints)[0]
	old := model.YggdrasilPublicKeys{
		ProfilePropertyKeys:   []model.YggdrasilPublicKey{{PublicKey: "old-profile"}},
		PlayerCertificateKeys: []model.YggdrasilPublicKey{{PublicKey: "old-certificate"}},
	}
	if err := redis.SetFallbackPublicKeys(ctx, source.ID, old, time.Hour); err != nil {
		t.Fatal(err)
	}

	svc := probe.New(db, redis)
	svc.Client = server.Client()
	if err := svc.Run(ctx); err != nil {
		t.Fatal(err)
	}
	assertCachedPublicKeys(t, ctx, db, redis, old)
	if discoveryCalls, servicesCalls := server.callsFor("/"), server.callsFor("/publickeys/"); discoveryCalls != 1 || servicesCalls != 1 {
		t.Fatalf("failed key request calls=(%d,%d), want one discovery and one services fallback", discoveryCalls, servicesCalls)
	}
	samples, err := redis.GetProbeHistory(ctx, time.Time{})
	if err != nil || len(samples) != 1 || samples[0].Services != probe.StatusUp {
		t.Fatalf("key failure must not change services health: samples=%#v err=%v", samples, err)
	}
}

type publicKeyProbeServer struct {
	*httptest.Server
	mu                     sync.Mutex
	calls                  map[string]int
	fixture                testutil.PublicKeyFixture
	discoveryOK            bool
	failServicesPublicKeys bool
}

func newPublicKeyProbeServer(t *testing.T, fixture testutil.PublicKeyFixture, discoveryOK bool) *publicKeyProbeServer {
	t.Helper()
	server := &publicKeyProbeServer{calls: map[string]int{}, fixture: fixture, discoveryOK: discoveryOK}
	server.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		server.mu.Lock()
		server.calls[req.URL.Path]++
		server.mu.Unlock()
		switch req.URL.Path {
		case "/":
			if discoveryOK {
				_ = json.NewEncoder(w).Encode(map[string]any{"signaturePublickey": fixture.PEM})
				return
			}
			_, _ = w.Write([]byte(`{"signaturePublickey":"invalid"}`))
		case "/publickeys/":
			if server.failServicesPublicKeys {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			_ = json.NewEncoder(w).Encode(model.YggdrasilPublicKeys{
				ProfilePropertyKeys:   []model.YggdrasilPublicKey{{PublicKey: fixture.DERBase64}},
				PlayerCertificateKeys: []model.YggdrasilPublicKey{{PublicKey: fixture.DERBase64}},
			})
		case "/session/minecraft/profile/" + probe.TestUUID,
			"/users/profiles/minecraft/" + probe.TestName,
			"/minecraft/profile/lookup/name/" + probe.TestName:
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return server
}

func (s *publicKeyProbeServer) callsFor(path string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls[path]
}

func saveProbeEndpoints(t *testing.T, db *database.DB, redis redisstore.Store, serverURL string, count int) {
	t.Helper()
	endpoints := make([]any, 0, count)
	for index := 0; index < count; index++ {
		endpoints = append(endpoints, map[string]any{
			"priority":     index + 1,
			"session_url":  serverURL,
			"account_url":  serverURL,
			"services_url": serverURL,
			"note":         "endpoint",
		})
	}
	if err := (settings.Settings{DB: db, Redis: redis}).SaveGroup(t.Context(), "fallback", map[string]any{"fallbacks": endpoints}); err != nil {
		t.Fatal(err)
	}
}

func assertCachedPublicKeys(t *testing.T, ctx context.Context, db *database.DB, redis redisstore.Store, want model.YggdrasilPublicKeys) {
	t.Helper()
	endpoints, err := db.Fallbacks.ListEndpoints(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sources := fallbacksvc.PublicKeySources(endpoints)
	if len(sources) != 1 {
		t.Fatalf("source count=%d, want 1: %#v", len(sources), sources)
	}
	got, err := redis.GetFallbackPublicKeys(ctx, []string{sources[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, map[string]model.YggdrasilPublicKeys{sources[0].ID: want}) {
		t.Fatalf("cached keys mismatch:\n got=%#v\nwant=%#v", got, map[string]model.YggdrasilPublicKeys{sources[0].ID: want})
	}
}
