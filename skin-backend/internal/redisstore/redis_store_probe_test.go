package redisstore

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestRedisStoreProbeHistoryAppendTrimFilterAndInvalidateExactly(t *testing.T) {
	store, _ := newTestRedisStore(t)
	ctx := context.Background()
	base := time.Now()
	old := ProbeSample{EndpointID: 1, Note: "old", SessionURL: "https://session.old", AccountURL: "https://account.old", ServicesURL: "https://services.old", CheckedAt: base.Add(-2 * time.Hour).UnixMilli(), Session: "up", Account: "up", Services: "up"}
	mid := ProbeSample{EndpointID: 2, Note: "mid", SessionURL: "https://session.mid", AccountURL: "https://account.mid", ServicesURL: "https://services.mid", CheckedAt: base.Add(-30 * time.Minute).UnixMilli(), Session: "down", Account: "up", Services: "up"}
	fresh := ProbeSample{EndpointID: 3, Note: "fresh", SessionURL: "https://session.fresh", AccountURL: "https://account.fresh", ServicesURL: "https://services.fresh", CheckedAt: base.UnixMilli(), Session: "up", Account: "down", Services: "down"}

	if err := store.AppendProbeSamples(ctx, nil, time.Hour); err != nil {
		t.Fatal(err)
	}
	empty, err := store.GetProbeHistory(ctx, time.Time{})
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty probe history mismatch: samples=%#v err=%v", empty, err)
	}
	if err := store.AppendProbeSamples(ctx, []ProbeSample{old, mid, fresh}, time.Hour); err != nil {
		t.Fatal(err)
	}
	all, err := store.GetProbeHistory(ctx, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(all, []ProbeSample{mid, fresh}) {
		t.Fatalf("probe history order/retention mismatch:\n got=%#v\nwant=%#v", all, []ProbeSample{mid, fresh})
	}
	recent, err := store.GetProbeHistory(ctx, base.Add(-20*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(recent, []ProbeSample{fresh}) {
		t.Fatalf("probe recent history mismatch:\n got=%#v\nwant=%#v", recent, []ProbeSample{fresh})
	}

	newest := ProbeSample{EndpointID: 4, Note: "newest", CheckedAt: base.Add(time.Minute).UnixMilli(), Session: "up", Account: "up", Services: "down"}
	if err := store.AppendProbeSamples(ctx, []ProbeSample{newest}, time.Hour); err != nil {
		t.Fatal(err)
	}
	trimmed, err := store.GetProbeHistory(ctx, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(trimmed, []ProbeSample{mid, fresh, newest}) {
		t.Fatalf("probe retention trim mismatch:\n got=%#v\nwant=%#v", trimmed, []ProbeSample{newest})
	}
	if err := store.InvalidateProbeHistory(ctx); err != nil {
		t.Fatal(err)
	}
	cleared, err := store.GetProbeHistory(ctx, time.Time{})
	if err != nil || len(cleared) != 0 {
		t.Fatalf("cleared probe history mismatch: samples=%#v err=%v", cleared, err)
	}
}
