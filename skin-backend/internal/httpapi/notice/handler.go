package notice

import (
	"net/http"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi/shared"
	noticesvc "element-skin/backend/internal/service/notice"
)

type Handler struct {
	auth    shared.AuthFunc
	notices noticesvc.Service
}

func New(db *database.DB, auth shared.AuthFunc) Handler {
	return Handler{auth: auth, notices: noticesvc.Service{DB: db}}
}

func (h Handler) Auth(next http.HandlerFunc) http.HandlerFunc {
	return h.auth(next)
}
