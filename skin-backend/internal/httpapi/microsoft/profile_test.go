package microsoft

import "testing"

func TestVerifiedMicrosoftProfileKeepsOnlyVerifiedProfileFields(t *testing.T) {
	verified := verifiedMicrosoftProfile(map[string]any{
		"has_game": false,
		"profile": map[string]any{
			"id":    "profile-id",
			"name":  "Player",
			"skins": []any{"skin"},
		},
	})
	if verified["id"] != "profile-id" || verified["name"] != "Player" {
		t.Fatalf("unexpected verified profile identity: %#v", verified)
	}
	if _, ok := verified["has_game"]; ok {
		t.Fatalf("verified profile should not include flow metadata: %#v", verified)
	}
	if capes, ok := verified["capes"].([]any); !ok || len(capes) != 0 {
		t.Fatalf("missing capes should become an empty list: %#v", verified["capes"])
	}
}
