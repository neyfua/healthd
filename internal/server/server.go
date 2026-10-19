package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/neyfua/healthd/internal/clinic"
)

// Server wraps HTTP server with graceful shutdown
type Server struct {
	httpServer *http.Server
	ready      atomic.Bool
	clinic     *clinic.Store
	metrics    *metrics
}

// New creates a new server on the given port
func New(port int) *Server {
	s := &Server{clinic: clinic.NewStore(), metrics: newMetrics()}
	s.ready.Store(true)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.healthHandler)
	mux.HandleFunc("/livez", s.healthHandler)
	mux.HandleFunc("/readyz", s.readinessHandler)
	mux.HandleFunc("/metrics", s.metricsHandler)
	mux.HandleFunc("/api/v1/appointments", s.appointmentsHandler)
	mux.HandleFunc("/api/v1/appointments/", s.appointmentHandler)
	mux.HandleFunc("/", rootHandler)

	s.httpServer = &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           securityHeaders(s.metrics.middleware(mux)),
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

// appointmentHandler serves GET and DELETE on /api/v1/appointments/{id}.
func (s *Server) appointmentHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getAppointment(w, r)
	case http.MethodDelete:
		s.cancelAppointment(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) getAppointment(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) cancelAppointment(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/v1/appointments/"):]
	if id == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	a, err := s.clinic.Cancel(id)
	switch err {
	case nil:
		writeJSON(w, http.StatusOK, a)
	case clinic.ErrNotFound:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	case clinic.ErrConflict:
		writeJSON(w, http.StatusConflict, map[string]string{"error": "already cancelled"})
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// metrics holds cumulative request counters keyed by method, route, status.
type metrics struct {
	mu     sync.Mutex
	counts map[string]int64
}

// newMetrics returns an empty metrics registry.
func newMetrics() *metrics {
	return &metrics{counts: make(map[string]int64)}
}

func (m *metrics) routeKey(path string) string {
	switch {
	case path == "/health", path == "/livez", path == "/readyz":
		return path
	case path == "/metrics":
		return path
	case path == "/api/v1/appointments":
		return path
	case strings.HasPrefix(path, "/api/v1/appointments/"):
		return "/api/v1/appointments/{id}"
	case path == "/":
		return path
	default:
		return "/other"
	}
}

func (m *metrics) record(method, route string, code int) {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions:
	default:
		method = "OTHER"
	}
	key := fmt.Sprintf("%s|%s|%d", method, route, code)
	m.mu.Lock()
	m.counts[key]++
	m.mu.Unlock()
}

func (m *metrics) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		route := m.routeKey(r.URL.Path)
		next.ServeHTTP(rw, r)
		m.record(r.Method, route, rw.status)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (m *metrics) handler(w http.ResponseWriter, scheduled, cancelled int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var sb strings.Builder
	sb.WriteString("# HELP healthd_http_requests_total Total HTTP requests handled by route.\n")
	sb.WriteString("# TYPE healthd_http_requests_total counter\n")
	keys := make([]string, 0, len(m.counts))
	for k := range m.counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		parts := strings.SplitN(k, "|", 3)
		sb.WriteString(fmt.Sprintf("healthd_http_requests_total{method=%q,route=%q,code=%q} %d\n",
			parts[0], parts[1], parts[2], m.counts[k]))
	}
	sb.WriteString(fmt.Sprintf("# HELP healthd_appointments Total appointments by status.\n# TYPE healthd_appointments gauge\nhealthd_appointments{status=%q} %d\nhealthd_appointments{status=%q} %d\n",
		clinic.StatusScheduled, scheduled, clinic.StatusCancelled, cancelled))
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(sb.String()))
}

func (s *Server) metricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.metrics.handler(w, int64(s.clinic.CountByStatus(clinic.StatusScheduled)), int64(s.clinic.CountByStatus(clinic.StatusCancelled)))
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
