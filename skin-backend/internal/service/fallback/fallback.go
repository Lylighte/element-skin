package fallback

import (
	"net/http"

	"element-skin/backend/internal/database"
)

type Fallback struct {
	DB     *database.DB
	Client *http.Client
}

type FallbackResponse struct {
	Status int
	Body   []byte
}
