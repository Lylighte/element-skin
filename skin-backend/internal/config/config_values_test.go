package config

import (
	"testing"
)

func TestConfigLookupAndIntegerCoercionExactValues(t *testing.T) {
	clearConfigEnv(t)
	raw := rawConfig{
		"nested": map[string]any{
			"int":     3,
			"int64":   int64(4),
			"float64": float64(5.9),
			"string":  "6",
			"bad":     true,
			"text":    "value",
			"nil":     nil,
		},
	}
	for _, tc := range []struct {
		key  string
		want int
	}{
		{key: "nested.int", want: 3},
		{key: "nested.int64", want: 4},
		{key: "nested.float64", want: 5},
		{key: "nested.string", want: 6},
		{key: "nested.bad", want: 9},
		{key: "nested.missing", want: 9},
		{key: "nested.nil", want: 9},
		{key: "nested.text.child", want: 9},
	} {
		if got := getInt(raw, tc.key, 9); got != tc.want {
			t.Fatalf("getInt(%q)=%d; want %d", tc.key, got, tc.want)
		}
	}
	if got := getString(raw, "nested.text", "fallback"); got != "value" {
		t.Fatalf("getString text=%q; want value", got)
	}
	if got := getString(raw, "nested.int", "fallback"); got != "fallback" {
		t.Fatalf("getString non-string=%q; want fallback", got)
	}
	if value, ok := lookup(raw, "nested.text"); !ok || value != "value" {
		t.Fatalf("lookup existing=%#v,%v; want value,true", value, ok)
	}
	if value, ok := lookup(raw, "nested.missing"); ok || value != nil {
		t.Fatalf("lookup missing=%#v,%v; want nil,false", value, ok)
	}
	setRaw(raw, "created.child.value", "ok")
	if value, ok := lookup(raw, "created.child.value"); !ok || value != "ok" {
		t.Fatalf("setRaw created value=%#v,%v; want ok,true", value, ok)
	}
}

func TestPostgresDSNEscapesStructuredFieldsExactly(t *testing.T) {
	got := postgresDSN("db.internal", "5432", "skin user", "p@ss/word", "skin db", "require")
	want := "postgresql://skin%20user:p%40ss%2Fword@db.internal:5432/skin%20db?sslmode=require"
	if got != want {
		t.Fatalf("postgresDSN()=%q; want %q", got, want)
	}
}
