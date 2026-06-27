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
	ID                  string  `json:"id"`
	Type                string  `json:"type"`
	Title               string  `json:"title"`
	StoragePath         string  `json:"storage_path"`
	OverlayOpacityLight float64 `json:"overlay_opacity_light"`
	OverlayOpacityDark  float64 `json:"overlay_opacity_dark"`
	StartYaw            float64 `json:"start_yaw"`
	StartPitch          float64 `json:"start_pitch"`
	YawSpeedDPS         float64 `json:"yaw_speed_dps"`
	PitchSpeedDPS       float64 `json:"pitch_speed_dps"`
	SortOrder           int     `json:"sort_order"`
	Enabled             bool    `json:"enabled"`
	DurationMS          int     `json:"duration_ms"`
	CreatedAt           int64   `json:"created_at"`
	UpdatedAt           int64   `json:"updated_at"`
}

type Notice struct {
	ID              string  `json:"id"`
	Type            string  `json:"type"`
	Title           string  `json:"title"`
	Summary         string  `json:"summary"`
	ContentMarkdown string  `json:"content_markdown"`
	DisplayMode     string  `json:"display_mode"`
	Level           string  `json:"level"`
	LinkText        string  `json:"link_text"`
	LinkURL         string  `json:"link_url"`
	Audience        string  `json:"audience"`
	Enabled         bool    `json:"enabled"`
	Pinned          bool    `json:"pinned"`
	Dismissible     bool    `json:"dismissible"`
	StartsAt        *int64  `json:"starts_at"`
	EndsAt          *int64  `json:"ends_at"`
	CreatedBy       *string `json:"created_by"`
	CreatedAt       int64   `json:"created_at"`
	UpdatedAt       int64   `json:"updated_at"`
}

type NoticeView struct {
	Notice
	Read        bool   `json:"read"`
	ReadAt      *int64 `json:"read_at"`
	DismissedAt *int64 `json:"dismissed_at"`
}
