package yggdrasil

import (
	"context"
	"errors"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
)

func (y Yggdrasil) Validate(ctx context.Context, access, client string) error {
	t, err := y.Redis.GetYggToken(ctx, access)
	if errors.Is(err, redisstore.ErrCacheMiss) {
		return yggErr(403, "ForbiddenOperationException", "Invalid token.")
	}
	if err != nil {
		return err
	}
	if (client != "" && client != t.ClientToken) || database.NowMS()-t.CreatedAt > int64(tokenTTL/time.Millisecond) {
		return yggErr(403, "ForbiddenOperationException", "Invalid token.")
	}
	if err := y.requireYggPermission(ctx, t.UserID, yggSessionValidatePermission); err != nil {
		return err
	}
	if ok, err := y.tokenProfileOwned(ctx, t); err != nil {
		return err
	} else if !ok {
		return yggErr(403, "ForbiddenOperationException", "Invalid token.")
	}
	return nil
}

func (y Yggdrasil) Token(ctx context.Context, access string) (model.Token, error) {
	token, err := y.Redis.GetYggToken(ctx, access)
	if errors.Is(err, redisstore.ErrCacheMiss) {
		return model.Token{}, yggErr(401, "Unauthorized", "Invalid token")
	}
	if err != nil {
		return model.Token{}, err
	}
	if ok, err := y.tokenProfileOwned(ctx, token); err != nil {
		return model.Token{}, err
	} else if !ok {
		return model.Token{}, yggErr(401, "Unauthorized", "Invalid token")
	}
	return token, nil
}

func (y Yggdrasil) Invalidate(ctx context.Context, access string) error {
	if access == "" {
		return nil
	}
	t, err := y.Redis.GetYggToken(ctx, access)
	if errors.Is(err, redisstore.ErrCacheMiss) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := y.requireYggPermission(ctx, t.UserID, yggSessionInvalidatePermission); err != nil {
		return err
	}
	return y.Redis.DeleteYggToken(ctx, access)
}

func (y Yggdrasil) tokenProfileOwned(ctx context.Context, token model.Token) (bool, error) {
	if token.ProfileID == nil {
		return true, nil
	}
	return y.DB.Profiles.VerifyOwnership(ctx, token.UserID, *token.ProfileID)
}

func yggUserPayload(u model.User) map[string]any {
	return map[string]any{"id": u.ID, "properties": []map[string]any{{"name": "preferredLanguage", "value": u.PreferredLanguage}}}
}
