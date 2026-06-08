package httpapi

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"element-skin/backend/internal/service"
	"element-skin/backend/internal/util"
)

func (r *Router) adminInvites(w http.ResponseWriter, req *http.Request) {
	lastCreated, lastCode, err := cursorCreatedHash(req.URL.Query().Get("cursor"), "last_code")
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid cursor"})
		return
	}
	res, err := r.db.ListInvites(req.Context(), util.ClampLimit(req.URL.Query().Get("limit"), 15), lastCreated, lastCode)
	if err != nil {
		util.Error(w, err)
		return
	}
	res["next_cursor"] = util.EncodeCursor(asMap(res["next_key"]))
	delete(res, "next_key")
	util.JSON(w, 200, res)
}

func (r *Router) adminCreateInvite(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	code, _ := body["code"].(string)
	if code == "" {
		id, err := util.GenerateUUIDNoDash()
		if err != nil {
			util.Error(w, err)
			return
		}
		code = id + id[:8]
	}
	if len(code) < 4 {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invite code too short"})
		return
	}
	total := 1
	if v, ok := body["total_uses"].(float64); ok {
		total = int(v)
	}
	note, _ := body["note"].(string)
	if err := r.db.CreateInvite(req.Context(), code, total, note); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"code": code, "total_uses": total, "note": note})
}

func (r *Router) adminDeleteInvite(w http.ResponseWriter, req *http.Request) {
	if err := r.db.DeleteInvite(req.Context(), req.PathValue("code")); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

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

func (r *Router) adminUploadCarousel(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseMultipartForm(6 << 20); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid multipart form"})
		return
	}
	file, header, err := req.FormFile("file")
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "file is required"})
		return
	}
	defer file.Close()
	ext := strings.ToLower(filepath.Ext(header.Filename))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp":
	default:
		util.Error(w, util.HTTPError{Status: 400, Detail: "Unsupported file format"})
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, 5*1024*1024+1))
	if err != nil {
		util.Error(w, err)
		return
	}
	if len(data) > 5*1024*1024 {
		util.Error(w, util.HTTPError{Status: 400, Detail: "File too large"})
		return
	}
	if err := os.MkdirAll(r.cfg.CarouselDir, 0o755); err != nil {
		util.Error(w, err)
		return
	}
	id, err := util.GenerateUUIDNoDash()
	if err != nil {
		util.Error(w, err)
		return
	}
	filename := id + ext
	if err := os.WriteFile(filepath.Join(r.cfg.CarouselDir, filename), data, 0o644); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"filename": filename})
}

func (r *Router) adminDeleteCarousel(w http.ResponseWriter, req *http.Request) {
	filename := filepath.Base(req.PathValue("filename"))
	if filename == "." || filename == string(filepath.Separator) || filename == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid filename"})
		return
	}
	err := os.Remove(filepath.Join(r.cfg.CarouselDir, filename))
	if err != nil && !os.IsNotExist(err) {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) adminGetSiteSettings(w http.ResponseWriter, req *http.Request) {
	res, err := (service.Settings{DB: r.db}).GetGroup(req.Context(), "site")
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) adminSaveSiteSettings(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := (service.Settings{DB: r.db}).SaveGroup(req.Context(), "site", body); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) adminGetSettingsGroup(w http.ResponseWriter, req *http.Request) {
	res, err := (service.Settings{DB: r.db}).GetGroup(req.Context(), req.PathValue("group"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) adminSaveSettingsGroup(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := (service.Settings{DB: r.db}).SaveGroup(req.Context(), req.PathValue("group"), body); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}
