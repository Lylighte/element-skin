package server

import (
	"encoding/json"
	"net/http"
)

// handleUnionHello is the public hello endpoint for the Union Hub. It returns
// a minimal acknowledgement so the Hub can confirm the member endpoint is
// reachable.
func (s *Server) handleUnionHello(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
	})
}

// handleUpdateList receives an updated server list from the Union Hub.
// Stub: returns 200; implementation will be added in a later todo.
func (s *Server) handleUpdateList(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// handleUpdatePrivateKey receives a new private-key version from the Union Hub.
// Stub: returns 200; implementation will be added in a later todo.
func (s *Server) handleUpdatePrivateKey(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// handleUpdateBackendKey receives a new member key from the Union Hub.
// Stub: returns 200; implementation will be added in a later todo.
func (s *Server) handleUpdateBackendKey(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// handleSync triggers a profile synchronization with the Union Hub.
// Stub: returns 200; implementation will be added in a later todo.
func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// handleQueryEmail resolves a profile name to an email address for the Union Hub.
// Stub: returns 204 No Content; implementation will be added in a later todo.
func (s *Server) handleQueryEmail(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// handleDiagnose answers a Hub diagnostic request with the echoed nonce.
// Stub: returns 200; implementation will be added in a later todo.
func (s *Server) handleDiagnose(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
