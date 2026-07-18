package microsoft

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/util"
)

func (h Handler) ImportProfile(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	res, err := h.workflow.Import(req.Context(), shared.CurrentActor(req), body["ms_token"])
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}
