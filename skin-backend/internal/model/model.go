package model

type User struct {
	ID                string
	Email             string
	Password          string
	IsAdmin           bool
	IsSuperAdmin      bool
	PreferredLanguage string
	DisplayName       string
	CreatedAt         int64
	BannedUntil       *int64
	AvatarHash        *string
}

type Profile struct {
	ID           string
	UserID       string
	Name         string
	TextureModel string
	SkinHash     *string
	CapeHash     *string
}

type Token struct {
	AccessToken string
	ClientToken string
	UserID      string
	ProfileID   *string
	CreatedAt   int64
}

type Session struct {
	ServerID    string
	AccessToken string
	IP          *string
	CreatedAt   int64
}

type Invite struct {
	Code      string
	CreatedAt *int64
	UsedBy    *string
	TotalUses *int
	UsedCount int
	Note      string
}

type HomepageMedia struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Title       string         `json:"title"`
	StoragePath string         `json:"storage_path"`
	Config      map[string]any `json:"config"`
	SortOrder   int            `json:"sort_order"`
	Enabled     bool           `json:"enabled"`
	DurationMS  int            `json:"duration_ms"`
	CreatedAt   int64          `json:"created_at"`
	UpdatedAt   int64          `json:"updated_at"`
}
