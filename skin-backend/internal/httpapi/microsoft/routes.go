package microsoft

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/util"
)

func (h Handler) AuthURL(w http.ResponseWriter, req *http.Request) {
	result, err := h.workflow.Start(req.Context(), shared.CurrentActor(req))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{
		"auth_url": result.AuthorizationURL,
		"state":    result.State,
	})
}

func (h Handler) Callback(w http.ResponseWriter, req *http.Request) {
	if errText := req.URL.Query().Get("error"); errText != "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Authorization failed: " + errText})
		return
	}
	code := req.URL.Query().Get("code")
	state := req.URL.Query().Get("state")
	if code == "" || state == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Missing code or state parameter"})
		return
	}
	redirectURL, err := h.workflow.Complete(req.Context(), code, state)
	if err != nil {
		util.Error(w, err)
		return
	}
	http.Redirect(w, req, redirectURL, http.StatusFound)
}
