package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	s := New(0)
	ts := httptest.NewServer(s.httpServer.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	tests := []struct {
		header string
		want   string
	}{
		{"X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "DENY"},
		{"Referrer-Policy", "no-referrer"},
	}

	for _, tt := range tests {
		if got := resp.Header.Get(tt.header); got != tt.want {
			t.Errorf("header %s = %q, want %q", tt.header, got, tt.want)
		}
	}
}

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

func TestIndexHandler(t *testing.T) {
	s := New(0)
	ts := httptest.NewServer(s.httpServer.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Header.Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", got)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	if !strings.Contains(string(body), "<title>healthd</title>") {
		t.Error("index HTML missing expected title")
	}
}

func TestStaticAssets(t *testing.T) {
	s := New(0)
	ts := httptest.NewServer(s.httpServer.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/static/app.js")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Header.Get("Content-Type"); !strings.Contains(got, "javascript") {
		t.Errorf("Content-Type = %q, want javascript", got)
	}

	resp, err = http.Get(ts.URL + "/static/app.css")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Header.Get("Content-Type"); !strings.Contains(got, "css") {
		t.Errorf("Content-Type = %q, want css", got)
	}
}

func TestNotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	w := httptest.NewRecorder()
	indexHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestIndexMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()
	indexHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestInfoHandler(t *testing.T) {
	w := httptest.NewRecorder()
	infoHandler(w, httptest.NewRequest(http.MethodGet, "/api/v1/info", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := strings.TrimSpace(w.Body.String()); got != `{"service":"healthd","version":"dev"}` {
		t.Errorf("body = %s", got)
	}
}
