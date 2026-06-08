package httpapi

import (
	"net/http"

	"element-skin/backend/internal/util"
)

func (r *Router) yggMetadata(w http.ResponseWriter, req *http.Request) {
	res, err := r.ygg.Metadata(req.Context())
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) yggAuthenticate(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		ClientToken string `json:"clientToken"`
		RequestUser bool   `json:"requestUser"`
	}
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	res, err := r.ygg.Authenticate(req.Context(), body.Username, body.Password, body.ClientToken, body.RequestUser)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) yggRefresh(w http.ResponseWriter, req *http.Request) {
	var body struct {
		AccessToken     string         `json:"accessToken"`
		ClientToken     string         `json:"clientToken"`
		RequestUser     bool           `json:"requestUser"`
		SelectedProfile map[string]any `json:"selectedProfile"`
	}
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	selected := ""
	if body.SelectedProfile != nil {
		selected, _ = body.SelectedProfile["id"].(string)
	}
	res, err := r.ygg.Refresh(req.Context(), body.AccessToken, body.ClientToken, selected, body.RequestUser)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) yggValidate(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := r.ygg.Validate(req.Context(), body["accessToken"], body["clientToken"]); err != nil {
		util.Error(w, err)
		return
	}
	w.WriteHeader(204)
}

func (r *Router) yggInvalidate(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if body["accessToken"] != "" {
		_ = r.db.DeleteToken(req.Context(), body["accessToken"])
	}
	w.WriteHeader(204)
}

func (r *Router) yggSignout(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(204)
}
