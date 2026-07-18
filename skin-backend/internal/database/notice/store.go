package notice

import "github.com/jackc/pgx/v5/pgxpool"

type Store struct {
	Pool *pgxpool.Pool
}

type UserListOptions struct {
	UserID               string
	CanReadAdminAudience bool
	Type                 string
	Limit                int
	Now                  int64
	IncludeRead          bool
	LastPinned           *bool
	LastCreated          *int64
	LastID               string
}

type AdminListOptions struct {
	Type        string
	Status      string
	Limit       int
	Now         int64
	LastPinned  *bool
	LastCreated *int64
	LastID      string
}
