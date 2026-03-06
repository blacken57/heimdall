package api

import (
	"net/http"

	"github.com/blacken57/heimdall/internal/config"
	"github.com/blacken57/heimdall/internal/db"
)

type Server struct {
	cfg     *config.Config
	db      *db.DB
	handler http.Handler
}

func New(cfg *config.Config, database *db.DB) *Server {
	s := &Server{cfg: cfg, db: database}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/partials/services", s.handlePartialServices)
	mux.HandleFunc("/partials/day-detail", s.handleDayDetail)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	mux.HandleFunc("/", s.handleIndex)

	if cfg.BasicAuthEnabled() {
		s.handler = basicAuth(cfg.HeimdallUser, cfg.HeimdallPassword, mux)
	} else {
		s.handler = mux
	}

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

func basicAuth(user, password string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// /health is always open.
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		u, p, ok := r.BasicAuth()
		if !ok || u != user || p != password {
			w.Header().Set("WWW-Authenticate", `Basic realm="Heimdall"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
