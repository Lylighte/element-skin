package yggdrasil

import (
	"net/http"

	fallbacksvc "element-skin/backend/internal/service/fallback"
)

func WriteFallbackForTest(w http.ResponseWriter, resp *fallbacksvc.FallbackResponse) bool {
	return writeFallback(w, resp)
}
