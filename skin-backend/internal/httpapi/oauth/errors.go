package oauth

import (
	"errors"
	"net/http"

	"element-skin/backend/internal/util"
)

type protocolErrorBody struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}

func writeProtocolError(w http.ResponseWriter, err error) {
	code, description, status := protocolError(err)
	if code == "invalid_client" {
		w.Header().Set("WWW-Authenticate", `Basic realm="oauth"`)
	}
	util.JSON(w, status, protocolErrorBody{
		Error:            code,
		ErrorDescription: description,
	})
}

func protocolError(err error) (string, string, int) {
	var httpErr util.HTTPError
	if !errors.As(err, &httpErr) {
		return "server_error", util.InternalServerErrorDetail, http.StatusInternalServerError
	}
	code := protocolErrorCode(httpErr)
	return code, httpErr.Detail, protocolErrorStatus(code, httpErr.Status)
}

func protocolErrorCode(err util.HTTPError) string {
	switch err.Detail {
	case "authorization_pending", "access_denied", "expired_token":
		return err.Detail
	case "unsupported grant_type":
		return "unsupported_grant_type"
	case "invalid client_id", "invalid client_secret":
		return "invalid_client"
	case "invalid authorization code", "invalid refresh_token", "invalid device_code", "invalid code_verifier":
		return "invalid_grant"
	case "scope is required", "invalid scope", "scope exceeds client permission limit":
		return "invalid_scope"
	case "client_credentials requires a confidential client":
		return "unauthorized_client"
	case "permission denied":
		return "access_denied"
	default:
		if err.Status == http.StatusForbidden {
			return "access_denied"
		}
		if err.Status >= http.StatusInternalServerError {
			return "server_error"
		}
		return "invalid_request"
	}
}

func protocolErrorStatus(code string, fallback int) int {
	switch code {
	case "invalid_client":
		return http.StatusUnauthorized
	case "server_error":
		return http.StatusInternalServerError
	default:
		if fallback >= http.StatusInternalServerError {
			return http.StatusInternalServerError
		}
		return http.StatusBadRequest
	}
}
