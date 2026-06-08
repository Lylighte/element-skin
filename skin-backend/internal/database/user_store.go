package database

import (
	"context"

	"element-skin/backend/internal/database/user"
	"element-skin/backend/internal/model"
)

func (db *DB) userStore() user.Store {
	return user.Store{Pool: db.Pool}
}

func (db *DB) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	return db.userStore().GetByEmail(ctx, email)
}

func (db *DB) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	return db.userStore().GetByID(ctx, id)
}

func (db *DB) CreateUser(ctx context.Context, u model.User) error {
	return db.userStore().Create(ctx, u)
}

func (db *DB) CreateUserWithProfile(ctx context.Context, u model.User, p model.Profile, inviteCode, usedBy string) error {
	return db.userStore().CreateWithProfile(ctx, u, p, inviteCode, usedBy)
}

func (db *DB) CountUsers(ctx context.Context) (int, error) {
	return db.userStore().Count(ctx)
}

func (db *DB) IsDisplayNameTaken(ctx context.Context, name string, exclude string) (bool, error) {
	return db.userStore().IsDisplayNameTaken(ctx, name, exclude)
}

func (db *DB) UpdateUser(ctx context.Context, id string, fields map[string]any) error {
	return db.userStore().Update(ctx, id, fields)
}

func (db *DB) UpdatePassword(ctx context.Context, id, hash string) error {
	return db.userStore().UpdatePassword(ctx, id, hash)
}

func (db *DB) UpdatePasswordAndRevokeRefresh(ctx context.Context, id, hash string) (bool, error) {
	return db.userStore().UpdatePasswordAndRevokeRefresh(ctx, id, hash)
}

func (db *DB) DeleteUser(ctx context.Context, id string) (bool, error) {
	return db.userStore().Delete(ctx, id)
}

func (db *DB) IsBanned(ctx context.Context, id string) (bool, error) {
	return db.userStore().IsBanned(ctx, id)
}
