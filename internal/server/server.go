package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

// Server wraps HTTP server with graceful shutdown
type Server struct {
	httpServer *http.Server
	ready      atomic.Bool
}

// New creates a new server on the given port
func New(port int) *Server {
	s := &Server{}
	s.ready.Store(true)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.healthHandler)
	mux.HandleFunc("/livez", s.healthHandler)
	mux.HandleFunc("/readyz", s.readinessHandler)
	mux.HandleFunc("/", rootHandler)

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return s
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// readinessHandler reports whether the server is still accepting traffic.
// It returns 503 once graceful shutdown has begun so load balancers stop
// routing new requests while in-flight ones drain.
func (s *Server) readinessHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if !s.ready.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"unavailable"}`))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"service":"healthd","version":"dev"}`))
}

// Start runs the server (blocks)
func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server before the context deadline.
// It first marks the server as not ready so /readyz returns 503,
// allowing load balancers to drain connections before the process exits.
func (s *Server) Shutdown(ctx context.Context) error {
	s.ready.Store(false)
	log.Println("server marked not ready, draining connections")
	return s.httpServer.Shutdown(ctx)
}
