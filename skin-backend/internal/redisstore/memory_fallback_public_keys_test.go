package redisstore

import (
	"context"
	"reflect"
	"testing"
	"time"

	"element-skin/backend/internal/model"
)

func TestMemoryFallbackPublicKeysCacheTTLAndInvalidationExact(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	now := time.Unix(100, 0)
	store.now = func() time.Time { return now }
	keys := model.YggdrasilPublicKeys{
		ProfilePropertyKeys:   []model.YggdrasilPublicKey{{PublicKey: "profile"}},
		PlayerCertificateKeys: []model.YggdrasilPublicKey{{PublicKey: "certificate"}},
	}
	if err := store.SetFallbackPublicKeys(ctx, "source-a", keys, time.Minute); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetFallbackPublicKeys(ctx, []string{"source-a", "source-a", "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, map[string]model.YggdrasilPublicKeys{"source-a": keys}) {
		t.Fatalf("cached keys mismatch:\n got=%#v\nwant=%#v", got, map[string]model.YggdrasilPublicKeys{"source-a": keys})
	}

	now = now.Add(time.Minute)
	got, err = store.GetFallbackPublicKeys(ctx, []string{"source-a"})
	if err != nil || len(got) != 0 {
		t.Fatalf("expired cache mismatch: got=%#v err=%v", got, err)
	}

	if err := store.SetFallbackPublicKeys(ctx, "source-a", keys, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := store.SetFallbackPublicKeys(ctx, "source-b", keys, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := store.InvalidateFallbackPublicKeys(ctx); err != nil {
		t.Fatal(err)
	}
	got, err = store.GetFallbackPublicKeys(ctx, []string{"source-a", "source-b"})
	if err != nil || len(got) != 0 || store.Len() != 0 {
		t.Fatalf("invalidated cache mismatch: got=%#v len=%d err=%v", got, store.Len(), err)
	}
}
