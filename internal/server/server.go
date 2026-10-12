package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/neyfua/healthd/internal/clinic"
)

// Server wraps HTTP server with graceful shutdown
type Server struct {
	httpServer *http.Server
	ready      atomic.Bool
	clinic     *clinic.Store
}

// New creates a new server on the given port
func New(port int) *Server {
	s := &Server{clinic: clinic.NewStore()}
	s.ready.Store(true)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.healthHandler)
	mux.HandleFunc("/livez", s.healthHandler)
	mux.HandleFunc("/readyz", s.readinessHandler)
	mux.HandleFunc("/api/v1/appointments", s.appointmentsHandler)
	mux.HandleFunc("/api/v1/appointments/", s.appointmentHandler)
	mux.HandleFunc("/", rootHandler)

	s.httpServer = &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           securityHeaders(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
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

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
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

// appointmentsHandler serves GET (list) and POST (create) on the collection.
func (s *Server) appointmentsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.clinic.List())
	case http.MethodPost:
		// Limit body size to prevent resource exhaustion
		const maxBody = 1 << 20 // 1 MB
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)

		var in clinic.CreateInput
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()

		if err := dec.Decode(&in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
			return
		}
		// Reject any trailing data after the first JSON object
		if err := dec.Decode(&struct{}{}); err != io.EOF {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unexpected trailing data"})
			return
		}

		a, err := s.clinic.Create(in)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, a)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// appointmentHandler serves GET on /api/v1/appointments/{id}.
func (s *Server) appointmentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Path[len("/api/v1/appointments/"):]
	if id == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	a, err := s.clinic.Get(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
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
