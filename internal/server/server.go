package server

import (
	"fmt"
	"log"
	"net/http"

	"github.com/fullstack-pw/cks-terminal-mgmt/internal/config"
	"github.com/fullstack-pw/cks-terminal-mgmt/internal/terminal"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	cfg     *config.Config
	manager *terminal.Manager
	mux     *http.ServeMux
}

func New(cfg *config.Config) *Server {
	s := &Server{
		cfg:     cfg,
		manager: terminal.NewManager(cfg.SSHKeyPath, cfg.SSHUser),
		mux:     http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/terminal", s.manager.HandleTerminal)
	s.mux.Handle("/metrics", promhttp.Handler())
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","active_sessions":%d}`, s.manager.ActiveSessions())
}

func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	log.Printf("Starting cks-terminal-mgmt on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) Shutdown() {
	s.manager.Cleanup()
}
