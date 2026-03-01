package health

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type response struct {
	Status string `json:"status"`
}

// Server provides /healthz and /readyz HTTP endpoints.
type Server struct {
	Port   int
	Ready  func() bool
	Logger *slog.Logger
	server *http.Server
}

// NewServer creates a health server on the given port.
func NewServer(port int, ready func() bool, logger *slog.Logger) *Server {
	return &Server{
		Port:   port,
		Ready:  ready,
		Logger: logger,
	}
}

// Start runs the health server in a blocking manner.
// Call in a goroutine. Use Shutdown to stop.
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)

	s.server = &http.Server{
		Addr:              fmt.Sprintf(":%d", s.Port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	s.Logger.Info("health server starting", "port", s.Port)
	return s.server.ListenAndServe()
}

// Shutdown gracefully stops the health server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, response{Status: "ok"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	if s.Ready != nil && s.Ready() {
		writeJSON(w, http.StatusOK, response{Status: "ready"})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, response{Status: "not ready"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
