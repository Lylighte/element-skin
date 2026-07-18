package user

import "github.com/jackc/pgx/v5/pgxpool"

type Store struct {
	Pool *pgxpool.Pool
}
