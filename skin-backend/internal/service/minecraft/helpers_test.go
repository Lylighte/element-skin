package minecraft

import (
	"reflect"
	"testing"
)

func TestFirstTexturesPropertyHandlesMissingMalformedAndOrderedProperties(t *testing.T) {
	if got := firstTexturesProperty(map[string]any{}); got != nil {
		t.Fatalf("missing properties should return nil, got %#v", got)
	}
	if got := firstTexturesProperty(map[string]any{"properties": []any{map[string]any{"name": "textures"}}}); got != nil {
		t.Fatalf("malformed properties type should return nil, got %#v", got)
	}
	if got := firstTexturesProperty(map[string]any{"properties": []map[string]any{{"name": "profile"}}}); got != nil {
		t.Fatalf("properties without textures should return nil, got %#v", got)
	}
	want := map[string]any{"name": "textures", "value": "abc", "signature": "sig"}
	got := firstTexturesProperty(map[string]any{"properties": []map[string]any{{"name": "profile"}, want}})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("textures property mismatch:\n got=%#v\nwant=%#v", got, want)
	}
}
