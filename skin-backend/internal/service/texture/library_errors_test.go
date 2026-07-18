package texture_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/testutil"
)

func TestTextureServiceClosedDatabaseReturnsExactDependencyErrors(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newLibraryService(db)
	actor := textureUserActor("closed-texture-user")
	db.Close()

	checks := []struct {
		name string
		call func() error
	}{
		{name: "apply texture", call: func() error {
			return svc.ApplyTextureToProfile(ctx, actor, "closed-profile", "closed-hash", "skin")
		}},
		{name: "texture detail", call: func() error {
			result, err := svc.TextureDetail(ctx, actor, "closed-hash", "skin")
			if result != nil {
				t.Fatalf("TextureDetail closed database returned result=%#v", result)
			}
			return err
		}},
		{name: "update texture note", call: func() error {
			result, err := svc.UpdateTexture(ctx, actor, "closed-hash", "skin", map[string]any{"note": "Closed"})
			if result != nil {
				t.Fatalf("UpdateTexture closed database returned result=%#v", result)
			}
			return err
		}},
		{name: "update texture empty body", call: func() error {
			result, err := svc.UpdateTexture(ctx, actor, "closed-hash", "skin", map[string]any{})
			if result != nil {
				t.Fatalf("UpdateTexture empty closed database returned result=%#v", result)
			}
			return err
		}},
		{name: "delete texture", call: func() error {
			return svc.DeleteTexture(ctx, actor, "closed-hash", "skin")
		}},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); !closedPoolError(err) {
				t.Fatalf("%s closed database error=%v; want closed pool", tc.name, err)
			}
		})
	}
}
