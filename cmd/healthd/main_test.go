package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckHealth(t *testing.T) {
	t.Run("healthy server passes", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != healthPath {
				t.Errorf("path = %s, want %s", r.URL.Path, healthPath)
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		if err := checkHealthURL(ts.URL + healthPath); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("unhealthy status fails", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer ts.Close()

		if err := checkHealthURL(ts.URL + healthPath); err == nil {
			t.Fatal("want error for non-200 status")
		}
	})

	t.Run("unreachable server fails", func(t *testing.T) {
		if err := checkHealthURL("http://127.0.0.1:1" + healthPath); err == nil {
			t.Fatal("want error for unreachable endpoint")
		}
	})
}

func TestPortFromEnv(t *testing.T) {
	t.Run("default port", func(t *testing.T) {
		t.Setenv("PORT", "")
		port, err := portFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if port != 8080 {
			t.Errorf("port = %d, want 8080", port)
		}
	})

	t.Run("valid port", func(t *testing.T) {
		t.Setenv("PORT", "9090")
		port, err := portFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if port != 9090 {
			t.Errorf("port = %d, want 9090", port)
		}
	})

	t.Run("invalid port rejected", func(t *testing.T) {
		t.Setenv("PORT", "70000")
		if _, err := portFromEnv(); err == nil {
			t.Fatal("want error for invalid port")
		}
	})
}
