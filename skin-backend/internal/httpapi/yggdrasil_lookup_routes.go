package httpapi

import (
	"net/http"
	"strings"

	"element-skin/backend/internal/service"
	"element-skin/backend/internal/util"
)

func (r *Router) yggHasJoined(w http.ResponseWriter, req *http.Request) {
	username := req.URL.Query().Get("username")
	serverID := req.URL.Query().Get("serverId")
	res, status, err := r.ygg.HasJoined(req.Context(), username, serverID)
	if err != nil {
		util.Error(w, err)
		return
	}
	if status == 204 {
		resp, err := (service.Fallback{DB: r.db}).HasJoined(req.Context(), username, serverID, req.URL.Query().Get("ip"))
		if err != nil {
			util.Error(w, err)
			return
		}
		if writeFallback(w, resp) {
			return
		}
		w.WriteHeader(204)
		return
	}
	util.JSON(w, status, res)
}

func (r *Router) yggProfile(w http.ResponseWriter, req *http.Request) {
	unsigned := req.URL.Query().Get("unsigned") != "false"
	res, status, err := r.ygg.Profile(req.Context(), req.PathValue("uuid"), unsigned)
	if err != nil {
		util.Error(w, err)
		return
	}
	if status == 204 {
		resp, err := (service.Fallback{DB: r.db}).GetProfile(req.Context(), req.PathValue("uuid"), unsigned)
		if err != nil {
			util.Error(w, err)
			return
		}
		if writeFallback(w, resp) {
			return
		}
		w.WriteHeader(204)
		return
	}
	util.JSON(w, 200, res)
}

func writeFallback(w http.ResponseWriter, resp *service.FallbackResponse) bool {
	if resp == nil {
		return false
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	status := resp.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write(resp.Body)
	return true
}

func (r *Router) lookupName(w http.ResponseWriter, req *http.Request) {
	playerName := req.PathValue("playerName")
	res, status, err := r.ygg.LookupName(req.Context(), playerName)
	if err != nil {
		util.Error(w, err)
		return
	}
	if status == 204 {
		var resp *service.FallbackResponse
		if strings.HasPrefix(req.URL.Path, "/api/minecraft/profile/lookup/name/") || strings.HasPrefix(req.URL.Path, "/minecraft/profile/lookup/name/") {
			resp, err = (service.Fallback{DB: r.db}).ServicesLookup(req.Context(), playerName)
		} else {
			resp, err = (service.Fallback{DB: r.db}).GetProfileByName(req.Context(), playerName)
		}
		if err != nil {
			util.Error(w, err)
			return
		}
		if writeFallback(w, resp) {
			return
		}
		w.WriteHeader(204)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) lookupNames(w http.ResponseWriter, req *http.Request) {
	var names []string
	if err := decodeJSON(req, &names); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Request body must be an array"})
		return
	}
	profiles, err := r.db.SearchProfilesByNames(req.Context(), names, 100)
	if err != nil {
		util.Error(w, err)
		return
	}
	out := make([]map[string]any, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, map[string]any{"id": p.ID, "name": p.Name})
	}
	found := map[string]bool{}
	for _, p := range profiles {
		found[strings.ToLower(p.Name)] = true
	}
	missing := make([]string, 0, len(names))
	for _, name := range names {
		if !found[strings.ToLower(name)] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		fallbackProfiles, err := (service.Fallback{DB: r.db}).BulkLookup(req.Context(), missing)
		if err != nil {
			util.Error(w, err)
			return
		}
		if len(fallbackProfiles) > 0 {
			out = append(out, fallbackProfiles...)
		}
	}
	util.JSON(w, 200, out)
}
