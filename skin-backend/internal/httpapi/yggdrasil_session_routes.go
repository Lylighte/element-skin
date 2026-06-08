package httpapi

import (
	"net/http"

	"element-skin/backend/internal/service"
	"element-skin/backend/internal/util"
)

func (r *Router) yggJoin(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := r.ygg.Join(req.Context(), body["accessToken"], body["selectedProfile"], body["serverId"], req.RemoteAddr); err != nil {
		util.Error(w, err)
		return
	}
	w.WriteHeader(204)
}

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
