package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"zoamel/pokesensei/internal/config"
)

type Server struct {
	cfg        *config.Config
	log        *slog.Logger
	mux        *http.ServeMux
	httpServer *http.Server
}

func New(cfg *config.Config, log *slog.Logger) *Server {
	s := &Server{
		cfg: cfg,
		log: log,
		mux: http.NewServeMux(),
	}

	s.httpServer = &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      s.withMiddleware(s.mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

func (s *Server) Handle(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, handler)
}

func (s *Server) Handler() http.Handler {
	return s.httpServer.Handler
}

func (s *Server) Start() error {
	s.log.Info("server starting", slog.String("addr", s.httpServer.Addr))
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
