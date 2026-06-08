package profile

import (
	"context"

	"element-skin/backend/internal/model"
)

type rowsCompat interface {
	Close()
	Next() bool
	Scan(...any) error
	Err() error
}

func (s Store) ListByUser(ctx context.Context, userID string, limit int, lastID string) (map[string]any, error) {
	actual := limit + 1
	var rows rowsCompat
	var err error
	if lastID != "" {
		rows, err = s.Pool.Query(ctx, `SELECT id,user_id,name,texture_model,skin_hash,cape_hash FROM profiles WHERE user_id=$1 AND id>$2 ORDER BY id LIMIT $3`, userID, lastID, actual)
	} else {
		rows, err = s.Pool.Query(ctx, `SELECT id,user_id,name,texture_model,skin_hash,cape_hash FROM profiles WHERE user_id=$1 ORDER BY id LIMIT $2`, userID, actual)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var got []model.Profile
	for rows.Next() {
		var p model.Profile
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.TextureModel, &p.SkinHash, &p.CapeHash); err != nil {
			return nil, err
		}
		got = append(got, p)
	}
	hasNext := len(got) > limit
	items := got
	if hasNext {
		items = got[:limit]
	}
	var next any
	if hasNext {
		next = map[string]any{"last_id": got[limit-1].ID}
	}
	out := make([]map[string]any, 0, len(items))
	for _, p := range items {
		out = append(out, Summary(p))
	}
	return map[string]any{"items": out, "has_next": hasNext, "next_key": next, "page_size": len(out)}, rows.Err()
}

func (s Store) ListAll(ctx context.Context, limit int, lastID, query string) (map[string]any, error) {
	actual := limit + 1
	var rows rowsCompat
	var err error
	if query != "" {
		pat := "%" + query + "%"
		if lastID != "" {
			rows, err = s.Pool.Query(ctx, `SELECT p.id,p.user_id,p.name,p.texture_model,p.skin_hash,p.cape_hash,u.email,u.display_name FROM profiles p JOIN users u ON p.user_id=u.id WHERE (p.name ILIKE $1 OR u.email ILIKE $1 OR u.display_name ILIKE $1) AND p.id>$2 ORDER BY p.id LIMIT $3`, pat, lastID, actual)
		} else {
			rows, err = s.Pool.Query(ctx, `SELECT p.id,p.user_id,p.name,p.texture_model,p.skin_hash,p.cape_hash,u.email,u.display_name FROM profiles p JOIN users u ON p.user_id=u.id WHERE (p.name ILIKE $1 OR u.email ILIKE $1 OR u.display_name ILIKE $1) ORDER BY p.id LIMIT $2`, pat, actual)
		}
	} else if lastID != "" {
		rows, err = s.Pool.Query(ctx, `SELECT p.id,p.user_id,p.name,p.texture_model,p.skin_hash,p.cape_hash,u.email,u.display_name FROM profiles p JOIN users u ON p.user_id=u.id WHERE p.id>$1 ORDER BY p.id LIMIT $2`, lastID, actual)
	} else {
		rows, err = s.Pool.Query(ctx, `SELECT p.id,p.user_id,p.name,p.texture_model,p.skin_hash,p.cape_hash,u.email,u.display_name FROM profiles p JOIN users u ON p.user_id=u.id ORDER BY p.id LIMIT $1`, actual)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var got []map[string]any
	for rows.Next() {
		var p model.Profile
		var ownerEmail, ownerName string
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.TextureModel, &p.SkinHash, &p.CapeHash, &ownerEmail, &ownerName); err != nil {
			return nil, err
		}
		item := Summary(p)
		item["user_id"] = p.UserID
		item["owner_email"] = ownerEmail
		item["owner_display_name"] = ownerName
		got = append(got, item)
	}
	hasNext := len(got) > limit
	items := got
	if hasNext {
		items = got[:limit]
	}
	var next any
	if hasNext {
		next = map[string]any{"last_id": got[limit-1]["id"]}
	}
	return map[string]any{"items": items, "has_next": hasNext, "next_key": next, "page_size": len(items)}, rows.Err()
}
