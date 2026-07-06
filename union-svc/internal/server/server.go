package server

import (
	"embed"
	"log/slog"
	"net/http"
	"time"

	"element-skin/union-svc/internal/bridge"
	"element-skin/union-svc/internal/config"
	"element-skin/union-svc/internal/oauth"
	"element-skin/union-svc/internal/union"
)

//go:embed static/index.html
var staticFiles embed.FS

func indexHTML() []byte {
	b, _ := staticFiles.ReadFile("static/index.html")
	return b
}

// Server is the union-svc HTTP server.
type Server struct {
	cfg         config.Config
	manager     *oauth.Manager
	unionClient *union.Client
	bridge      *bridge.Bridge
	stateStore  *StateStore
	httpClient  *http.Client
	logger      *slog.Logger
	mux         *http.ServeMux
}

// New creates a Server from configuration, opening the token and state stores.
func New(cfg config.Config, logger *slog.Logger) (*Server, error) {
	httpClient := &http.Client{Timeout: 30 * time.Second}

	manager, err := oauth.NewManager(cfg, httpClient)
	if err != nil {
		return nil, err
	}

	stateStore, err := OpenStateStore(cfg.Storage.Path)
	if err != nil {
		_ = manager.Close()
		return nil, err
	}

	unionClient, err := union.NewClient(cfg, httpClient)
	if err != nil {
		_ = stateStore.Close()
		_ = manager.Close()
		return nil, err
	}

	b := bridge.New(cfg.Elementskin.BaseURL, unionClient, manager, httpClient)

	s := &Server{
		cfg:         cfg,
		manager:     manager,
		unionClient: unionClient,
		bridge:      b,
		stateStore:  stateStore,
		httpClient:  httpClient,
		logger:      logger,
		mux:         http.NewServeMux(),
	}
	s.routes()
	return s, nil
}

func (s *Server) routes() {
	s.mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	s.mux.HandleFunc("/oauth/authorize", s.handleAuthorize)
	s.mux.HandleFunc("/oauth/callback", s.handleCallback)
	s.mux.HandleFunc("/api/profiles", s.handleListProfiles)
	s.mux.HandleFunc("/api/profiles/import", s.handleImportProfile)
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexHTML())
	})
}

// Handler returns the server's http.Handler.
func (s *Server) Handler() http.Handler {
	return s.mux
}

// Close closes the underlying stores.
func (s *Server) Close() error {
	var first error
	if err := s.manager.Close(); err != nil {
		first = err
	}
	if err := s.unionClient.Close(); err != nil && first == nil {
		first = err
	}
	if err := s.stateStore.Close(); err != nil && first == nil {
		first = err
	}
	return first
}
