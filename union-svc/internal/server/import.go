package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"element-skin/union-svc/internal/bridge"
	"element-skin/union-svc/internal/oauth"
	"element-skin/union-svc/internal/union"
)

func (s *Server) handleListProfiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	username := r.URL.Query().Get("username")
	if username == "" {
		writeDetail(w, http.StatusBadRequest, "username is required")
		return
	}

	items, err := s.bridge.ListProfiles(r.Context(), username)
	if err != nil {
		s.logger.Error("failed to list union profiles", "error", err)
		if errors.Is(err, union.ErrUnionNotConfigured) {
			writeDetail(w, http.StatusServiceUnavailable, "union hub is not configured")
			return
		}
		writeDetail(w, http.StatusBadGateway, "failed to list union profiles")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
}

func (s *Server) handleImportProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req bridge.ImportProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDetail(w, http.StatusBadRequest, "invalid json")
		return
	}

	if req.Name == "" {
		writeDetail(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Model == "" {
		req.Model = "default"
	}
	if req.Model != "default" && req.Model != "slim" {
		writeDetail(w, http.StatusBadRequest, "model must be default or slim")
		return
	}

	profile, err := s.bridge.ImportProfile(r.Context(), req)
	if err != nil {
		s.logger.Error("failed to import profile", "error", err)
		var apiErr *bridge.APIError
		if errors.As(err, &apiErr) {
			writeDetail(w, apiErr.Status, apiErr.Detail)
			return
		}
		if errors.Is(err, oauth.ErrNoToken) {
			writeDetail(w, http.StatusUnauthorized, "no stored oauth token; authorize first")
			return
		}
		writeDetail(w, http.StatusInternalServerError, "failed to import profile")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":   profile.ID,
		"name": profile.Name,
	})
}

func writeDetail(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"detail": detail})
}
