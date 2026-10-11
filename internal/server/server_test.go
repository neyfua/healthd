package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	s := New(0)
	tests := []struct {
		name       string
		method     string
		wantStatus int
		wantBody   string
	}{
		{"GET returns ok", http.MethodGet, http.StatusOK, `{"status":"ok"}`},
		{"POST not allowed", http.MethodPost, http.StatusMethodNotAllowed, ""},
		{"PUT not allowed", http.MethodPut, http.StatusMethodNotAllowed, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/health", nil)
			w := httptest.NewRecorder()
			s.healthHandler(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && w.Body.String() != tt.wantBody {
				t.Errorf("body = %s, want %s", w.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestReadinessHandler(t *testing.T) {
	s := New(0)
	tests := []struct {
		name       string
		ready      bool
		wantStatus int
		wantBody   string
	}{
		{"ready", true, http.StatusOK, `{"status":"ok"}`},
		{"not ready", false, http.StatusServiceUnavailable, `{"status":"unavailable"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s.ready.Store(tt.ready)
			w := httptest.NewRecorder()
			s.readinessHandler(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))
			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if w.Body.String() != tt.wantBody {
				t.Errorf("body = %s, want %s", w.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestProbesRoutedThroughMux(t *testing.T) {
	for _, path := range []string{"/health", "/livez", "/readyz"} {
		t.Run("GET "+path, func(t *testing.T) {
			s := New(0)
			ts := httptest.NewServer(s.httpServer.Handler)
			defer ts.Close()

			resp, err := http.Get(ts.URL + path)
			if err != nil {
				t.Fatalf("GET %s: %v", path, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("GET %s status = %d, want %d", path, resp.StatusCode, http.StatusOK)
			}
		})
	}
}

func TestRootHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	rootHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %s, want application/json", w.Header().Get("Content-Type"))
	}
}

func TestNotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	w := httptest.NewRecorder()
	rootHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}
