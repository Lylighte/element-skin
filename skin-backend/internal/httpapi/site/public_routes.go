package site

import (
	"net/http"
	"time"

	"element-skin/backend/internal/util"
)

func (h Handler) PublicLibrary(w http.ResponseWriter, req *http.Request) {
	limit := util.ClampLimit(req.URL.Query().Get("limit"))
	res, err := h.site.PublicLibrary(req.Context(), req.URL.Query().Get("cursor"), limit, req.URL.Query().Get("texture_type"), req.URL.Query().Get("q"), req.URL.Query().Get("sort"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) PublicSettings(w http.ResponseWriter, req *http.Request) {
	res, err := h.public.PublicSettings(req.Context())
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) PublicHomepageMedia(w http.ResponseWriter, req *http.Request) {
	items, err := h.public.HomepageMedia(req.Context())
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, items)
}

func (h Handler) PublicFallbackStatus(w http.ResponseWriter, req *http.Request) {
	res, err := h.public.FallbackStatus(req.Context(), time.Now())
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}
