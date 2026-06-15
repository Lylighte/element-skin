package site

import (
	"errors"
	"net/http"
	"time"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
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
	if cached, err := h.redis.GetPublicSettings(req.Context()); err == nil {
		util.JSON(w, 200, cached)
		return
	} else if !errors.Is(err, redisstore.ErrCacheMiss) {
		util.Error(w, err)
		return
	}
	res, err := h.settings.Public(req.Context(), h.cfg.SiteURL, h.cfg.APIURL)
	if err != nil {
		util.Error(w, err)
		return
	}
	if err := h.redis.SetPublicSettings(req.Context(), res, time.Duration(h.cfg.PublicCacheTTL)*time.Second); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) PublicHomepageMedia(w http.ResponseWriter, req *http.Request) {
	if cached, err := h.redis.GetPublicHomepageMedia(req.Context()); err == nil {
		util.JSON(w, 200, cached)
		return
	} else if !errors.Is(err, redisstore.ErrCacheMiss) {
		util.Error(w, err)
		return
	}
	items, err := h.db.HomepageMedia.List(req.Context(), true)
	if err != nil {
		util.Error(w, err)
		return
	}
	if items == nil {
		items = []model.HomepageMedia{}
	}
	if err := h.redis.SetPublicHomepageMedia(req.Context(), items, time.Duration(h.cfg.PublicCacheTTL)*time.Second); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, items)
}
