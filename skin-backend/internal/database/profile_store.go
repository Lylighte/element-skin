package database

import (
	"context"

	"element-skin/backend/internal/database/profile"
	"element-skin/backend/internal/model"
)

func (db *DB) profileStore() profile.Store {
	return profile.Store{Pool: db.Pool}
}

func NormalizeProfileModel(m string) string {
	return profile.NormalizeModel(m)
}

func ProfileSummary(p model.Profile) map[string]any {
	return profile.Summary(p)
}

func ProfileModelKey(item map[string]any) map[string]any {
	return profile.ModelKey(item)
}

func IsProfileNameConflict(err error) bool {
	return profile.IsNameConflict(err)
}

func (db *DB) CreateProfile(ctx context.Context, p model.Profile) error {
	return db.profileStore().Create(ctx, p)
}

func (db *DB) GetProfileByID(ctx context.Context, id string) (*model.Profile, error) {
	return db.profileStore().GetByID(ctx, id)
}

func (db *DB) GetProfileByName(ctx context.Context, name string) (*model.Profile, error) {
	return db.profileStore().GetByName(ctx, name)
}

func (db *DB) GetProfilesByUser(ctx context.Context, userID string, limit int) ([]model.Profile, error) {
	return db.profileStore().GetByUser(ctx, userID, limit)
}

func (db *DB) VerifyProfileOwnership(ctx context.Context, userID, profileID string) (bool, error) {
	return db.profileStore().VerifyOwnership(ctx, userID, profileID)
}

func (db *DB) CountProfilesByUser(ctx context.Context, userID string) (int, error) {
	return db.profileStore().CountByUser(ctx, userID)
}

func (db *DB) UpdateProfileName(ctx context.Context, id, name string) (bool, error) {
	return db.profileStore().UpdateName(ctx, id, name)
}

func (db *DB) UpdateProfileSkin(ctx context.Context, id string, hash *string) error {
	return db.profileStore().UpdateSkin(ctx, id, hash)
}

func (db *DB) UpdateProfileCape(ctx context.Context, id string, hash *string) error {
	return db.profileStore().UpdateCape(ctx, id, hash)
}

func (db *DB) UpdateProfileModel(ctx context.Context, id, model string) error {
	return db.profileStore().UpdateModel(ctx, id, model)
}

func (db *DB) DeleteProfileCascade(ctx context.Context, id string) (bool, error) {
	return db.profileStore().DeleteCascade(ctx, id)
}

func (db *DB) SearchProfilesByNames(ctx context.Context, names []string, limit int) ([]model.Profile, error) {
	return db.profileStore().SearchByNames(ctx, names, limit)
}
