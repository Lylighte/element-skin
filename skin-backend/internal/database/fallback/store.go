package fallback

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Endpoint struct {
	ID              int
	Priority        int
	SessionURL      string
	AccountURL      string
	ServicesURL     string
	CacheTTL        int
	SkinDomains     string
	EnableProfile   bool
	EnableHasJoined bool
	EnableWhitelist bool
	Note            string
}

type Store struct {
	Pool *pgxpool.Pool
}

func IsEndpointNotFound(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == "23503" &&
		pgErr.ConstraintName == "whitelisted_users_endpoint_id_fkey"
}

func (s Store) ListEndpoints(ctx context.Context) ([]map[string]any, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id,priority,session_url,account_url,services_url,cache_ttl,skin_domains,enable_profile,enable_hasjoined,enable_whitelist,note FROM fallback_endpoints ORDER BY priority,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var e Endpoint
		if err := rows.Scan(&e.ID, &e.Priority, &e.SessionURL, &e.AccountURL, &e.ServicesURL, &e.CacheTTL, &e.SkinDomains, &e.EnableProfile, &e.EnableHasJoined, &e.EnableWhitelist, &e.Note); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"id": e.ID, "priority": e.Priority, "session_url": e.SessionURL, "account_url": e.AccountURL,
			"services_url": e.ServicesURL, "cache_ttl": e.CacheTTL, "skin_domains": e.SkinDomains,
			"enable_profile": e.EnableProfile, "enable_hasjoined": e.EnableHasJoined,
			"enable_whitelist": e.EnableWhitelist, "note": e.Note,
		})
	}
	return out, rows.Err()
}

func (s Store) SaveEndpoints(ctx context.Context, endpoints []Endpoint) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM fallback_endpoints`); err != nil {
		return err
	}
	for _, e := range endpoints {
		if _, err := tx.Exec(ctx, `
			INSERT INTO fallback_endpoints (priority,session_url,account_url,services_url,cache_ttl,skin_domains,enable_profile,enable_hasjoined,enable_whitelist,note)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		`, e.Priority, e.SessionURL, e.AccountURL, e.ServicesURL, e.CacheTTL, e.SkinDomains, e.EnableProfile, e.EnableHasJoined, e.EnableWhitelist, e.Note); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s Store) CollectSkinDomains(ctx context.Context) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `SELECT skin_domains FROM fallback_endpoints ORDER BY priority,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[string]bool{}
	var out []string
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		for _, part := range strings.Split(raw, ",") {
			d := strings.TrimSpace(part)
			if d != "" && !seen[d] {
				seen[d] = true
				out = append(out, d)
			}
		}
	}
	return out, rows.Err()
}

func (s Store) PrimaryEndpoint(ctx context.Context) (map[string]any, error) {
	eps, err := s.ListEndpoints(ctx)
	if err != nil || len(eps) == 0 {
		return nil, err
	}
	return eps[0], nil
}

func (s Store) AddWhitelistUser(ctx context.Context, username string, endpointID int) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO whitelisted_users (username,endpoint_id,created_at) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`, username, endpointID, time.Now().UnixMilli())
	return err
}

func (s Store) IsUserInWhitelist(ctx context.Context, username string, endpointID int) (bool, error) {
	var one int
	err := s.Pool.QueryRow(ctx, `SELECT 1 FROM whitelisted_users WHERE username=$1 AND endpoint_id=$2`, username, endpointID).Scan(&one)
	if IsNoRows(err) {
		return false, nil
	}
	return err == nil, err
}

func (s Store) ListWhitelistUsers(ctx context.Context, endpointID int) ([]map[string]any, error) {
	rows, err := s.Pool.Query(ctx, `SELECT username,created_at FROM whitelisted_users WHERE endpoint_id=$1 ORDER BY username`, endpointID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var username string
		var createdAt int64
		if err := rows.Scan(&username, &createdAt); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"username": username, "created_at": createdAt})
	}
	return out, rows.Err()
}

func (s Store) RemoveWhitelistUser(ctx context.Context, username string, endpointID int) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM whitelisted_users WHERE username=$1 AND endpoint_id=$2`, username, endpointID)
	return err
}
