package httpapi

import (
	"fmt"
	"net/http"
	"strings"

	"element-skin/backend/internal/util"
)

func (r *Router) adminOfficialWhitelist(w http.ResponseWriter, req *http.Request) {
	endpointID, err := parsePositiveInt(req.URL.Query().Get("endpoint_id"))
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "endpoint_id is required"})
		return
	}
	users, err := r.db.ListWhitelistUsers(req.Context(), endpointID)
	if err != nil {
		util.Error(w, err)
		return
	}
	if users == nil {
		users = []map[string]any{}
	}
	util.JSON(w, 200, map[string]any{"items": users})
}

func (r *Router) adminAddOfficialWhitelist(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	username := strings.TrimSpace(asString(body["username"]))
	endpointID, err := parsePositiveInt(fmt.Sprint(body["endpoint_id"]))
	if username == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "username is required"})
		return
	}
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "endpoint_id is required"})
		return
	}
	if err := r.db.AddWhitelistUser(req.Context(), username, endpointID); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) adminRemoveOfficialWhitelist(w http.ResponseWriter, req *http.Request) {
	endpointID, err := parsePositiveInt(req.URL.Query().Get("endpoint_id"))
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "endpoint_id is required"})
		return
	}
	if err := r.db.RemoveWhitelistUser(req.Context(), req.PathValue("username"), endpointID); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}
