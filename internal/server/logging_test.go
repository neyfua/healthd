package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAccessLogEmitsStructuredRecord(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	s := New(0)
	s.logger = logger
	ts := httptest.NewServer(s.httpServer.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/appointments/apt-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("expected an access log record")
	}

	var record map[string]any
	if err := json.Unmarshal([]byte(line), &record); err != nil {
		t.Fatalf("log line is not JSON: %v\n%s", err, line)
	}

	for key, want := range map[string]any{
		"method": "GET",
		"route":  "/api/v1/appointments/{id}",
		"status": float64(http.StatusOK),
	} {
		if got := record[key]; got != want {
			t.Errorf("log field %s = %v, want %v", key, got, want)
		}
	}
	if _, ok := record["duration_ms"]; !ok {
		t.Error("log record missing duration_ms")
	}
}

func TestAccessLogRecordsErrorStatus(t *testing.T) {
	var buf bytes.Buffer
	s := New(0)
	s.logger = slog.New(slog.NewJSONHandler(&buf, nil))
	ts := httptest.NewServer(s.httpServer.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/appointments/missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if !strings.Contains(buf.String(), `"status":404`) {
		t.Errorf("expected 404 in log output, got %s", buf.String())
	}
}
