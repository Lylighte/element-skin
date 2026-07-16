package microsoft

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/util"
)

func (h Handler) GetProfile(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	result, err := h.workflow.Preview(req.Context(), shared.CurrentActor(req), body["ms_token"])
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{
		"profile":      result.Profile,
		"has_game":     result.HasGame,
		"import_token": result.ImportToken,
	})
}
