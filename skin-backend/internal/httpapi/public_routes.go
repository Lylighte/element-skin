package httpapi

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"element-skin/backend/internal/service"
	"element-skin/backend/internal/util"
)

func (r *Router) publicLibrary(w http.ResponseWriter, req *http.Request) {
	limit := util.ClampLimit(req.URL.Query().Get("limit"))
	res, err := r.site.PublicLibrary(req.Context(), req.URL.Query().Get("cursor"), limit, req.URL.Query().Get("texture_type"), req.URL.Query().Get("q"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) publicSettings(w http.ResponseWriter, req *http.Request) {
	res, err := (service.Settings{DB: r.db}).Public(req.Context(), r.cfg.SiteURL, r.cfg.APIURL)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) publicCarousel(w http.ResponseWriter, req *http.Request) {
	entries, err := os.ReadDir(r.cfg.CarouselDir)
	if os.IsNotExist(err) {
		util.JSON(w, 200, []string{})
		return
	}
	if err != nil {
		util.Error(w, err)
		return
	}
	var images []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		switch strings.ToLower(filepath.Ext(name)) {
		case ".png", ".jpg", ".jpeg", ".webp":
			images = append(images, name)
		}
	}
	if images == nil {
		images = []string{}
	}
	util.JSON(w, 200, images)
}
