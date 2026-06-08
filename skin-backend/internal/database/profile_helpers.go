package database

import (
	"errors"
	"strings"

	"element-skin/backend/internal/model"

	"github.com/jackc/pgx/v5/pgconn"
)

func ProfileSummary(p model.Profile) map[string]any {
	return map[string]any{"id": p.ID, "name": p.Name, "model": p.TextureModel, "skin_hash": p.SkinHash, "cape_hash": p.CapeHash}
}

func ProfileModelKey(item map[string]any) map[string]any {
	if v, ok := item["texture_model"]; ok {
		item["model"] = v
	}
	return item
}

func IsProfileNameConflict(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return true
	}
	return err != nil && strings.Contains(err.Error(), "duplicate key")
}
