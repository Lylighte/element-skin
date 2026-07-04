package yggdrasil

import "context"

func (y Yggdrasil) Signout(ctx context.Context, username, password string) error {
	u, _, err := y.verifyCredentials(ctx, username, password)
	if err != nil {
		return err
	}
	if u == nil {
		return yggErr(403, "ForbiddenOperationException", "Invalid credentials. Invalid username or password.")
	}
	if err := y.requireYggPermission(ctx, u.ID, yggSessionSignoutPermission); err != nil {
		return err
	}
	return y.Redis.DeleteYggTokensByUser(ctx, u.ID)
}
