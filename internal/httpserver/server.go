package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/thiagomartins/hackaton-processing/internal/config"
)

// ReadinessChecker validates whether critical dependencies are ready.
type ReadinessChecker func(ctx context.Context) error

type statusResponse struct {
	Status    string    `json:"status"`
	Service   string    `json:"service"`
	Timestamp time.Time `json:"timestamp"`
	Error     string    `json:"error,omitempty"`
}

// Server wraps the service HTTP API.
type Server struct {
	server          *http.Server
	logger          *slog.Logger
	readinessCheck  ReadinessChecker
	readinessTimout time.Duration
}

// New builds a new HTTP server for operational endpoints.
func New(cfg config.HTTPConfig, logger *slog.Logger, readinessChecker ReadinessChecker) *Server {
	s := &Server{
		server: &http.Server{
			Addr:         ":" + cfg.Port,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
		},
		logger:          logger,
		readinessCheck:  readinessChecker,
		readinessTimout: 2 * time.Second,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /ready", s.readinessHandler)
	s.server.Handler = mux

	return s
}

// Start runs the HTTP server.
func (s *Server) Start() error {
	s.logger.Info("starting HTTP server", "addr", s.server.Addr)
	return s.server.ListenAndServe()
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, statusResponse{
		Status:    "UP",
		Service:   "processing-service",
		Timestamp: time.Now().UTC(),
	})
}

func (s *Server) readinessHandler(w http.ResponseWriter, req *http.Request) {
	if s.readinessCheck != nil {
		ctx, cancel := context.WithTimeout(req.Context(), s.readinessTimout)
		defer cancel()

		if err := s.readinessCheck(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, statusResponse{
				Status:    "NOT_READY",
				Service:   "processing-service",
				Timestamp: time.Now().UTC(),
				Error:     err.Error(),
			})
			return
		}
	}

	writeJSON(w, http.StatusOK, statusResponse{
		Status:    "READY",
		Service:   "processing-service",
		Timestamp: time.Now().UTC(),
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
