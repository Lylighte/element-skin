package yggdrasil

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	fallbacksvc "element-skin/backend/internal/service/fallback"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/util"
)

func (h Handler) HasJoined(w http.ResponseWriter, req *http.Request) {
	username := req.URL.Query().Get("username")
	serverID := req.URL.Query().Get("serverId")
	res, status, err := h.ygg.HasJoined(req.Context(), username, serverID)
	if err != nil {
		util.Error(w, err)
		return
	}
	if status == 204 {
		resp, err := h.fallback.HasJoined(req.Context(), username, serverID, req.URL.Query().Get("ip"))
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

func (h Handler) Profile(w http.ResponseWriter, req *http.Request) {
	unsigned := req.URL.Query().Get("unsigned") != "false"
	res, status, err := h.ygg.Profile(req.Context(), req.PathValue("uuid"), unsigned)
	if err != nil {
		util.Error(w, err)
		return
	}
	if status == 204 {
		resp, err := h.fallback.GetProfile(req.Context(), req.PathValue("uuid"), unsigned)
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

func writeFallback(w http.ResponseWriter, resp *fallbacksvc.FallbackResponse) bool {
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

func (h Handler) LookupName(w http.ResponseWriter, req *http.Request) {
	h.lookupName(w, req, yggsvc.LookupAccount)
}

func (h Handler) LookupServicesName(w http.ResponseWriter, req *http.Request) {
	h.lookupName(w, req, yggsvc.LookupServices)
}

func (h Handler) lookupName(w http.ResponseWriter, req *http.Request, source yggsvc.LookupSource) {
	res, fallback, found, err := h.lookup.NameResponse(req.Context(), req.PathValue("playerName"), source)
	if err != nil {
		util.Error(w, err)
		return
	}
	if !found {
		w.WriteHeader(204)
		return
	}
	if fallback != nil {
		writeFallback(w, fallback)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) LookupNames(w http.ResponseWriter, req *http.Request) {
	h.lookupNames(w, req, false)
}

func (h Handler) LookupServicesNames(w http.ResponseWriter, req *http.Request) {
	h.lookupNames(w, req, true)
}

const maxServicesBulkNames = 10

func validateServicesBulkNames(names []string) bool {
	if len(names) == 0 || len(names) > maxServicesBulkNames {
		return false
	}
	for _, name := range names {
		if !util.ValidProfileName(name) {
			return false
		}
	}
	return true
}

func (h Handler) lookupNames(w http.ResponseWriter, req *http.Request, services bool) {
	var names []string
	if err := shared.DecodeJSON(req, &names); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Object: "request", Operation: "decode", Reason: "invalid"})
		return
	}
	if services && !validateServicesBulkNames(names) {
		util.Error(w, util.HTTPError{Status: 400, Object: "request", Operation: "validate", Reason: "invalid"})
		return
	}
	var profiles []map[string]any
	var err error
	if services {
		profiles, err = h.lookup.ServicesNames(req.Context(), names)
	} else {
		profiles, err = h.lookup.Names(req.Context(), names)
	}
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, profiles)
}
