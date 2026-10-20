package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/neyfua/healthd/internal/clinic"
)

func TestAppointmentsFilterByStatus(t *testing.T) {
	s := New(0)
	ts := httptest.NewServer(s.httpServer.Handler)
	defer ts.Close()

	// Cancel one seeded appointment so both statuses exist.
	resp, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/appointments/apt-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	del, err := http.DefaultClient.Do(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	del.Body.Close()

	t.Run("scheduled filter", func(t *testing.T) {
		got := fetchIDs(t, ts.URL+"/api/v1/appointments?status=scheduled")
		if len(got) != 1 || got[0] != "apt-2" {
			t.Errorf("scheduled ids = %v, want [apt-2]", got)
		}
	})

	t.Run("cancelled filter", func(t *testing.T) {
		got := fetchIDs(t, ts.URL+"/api/v1/appointments?status=cancelled")
		if len(got) != 1 || got[0] != "apt-1" {
			t.Errorf("cancelled ids = %v, want [apt-1]", got)
		}
	})

	t.Run("unknown status rejected", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/appointments?status=bogus")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", resp.StatusCode)
		}
	})
}

func fetchIDs(t *testing.T, url string) []string {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var list []clinic.Appointment
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	ids := make([]string, 0, len(list))
	for _, a := range list {
		ids = append(ids, a.ID)
	}
	return ids
}

func TestAppointmentsHandler(t *testing.T) {
	s := New(0)
	ts := httptest.NewServer(s.httpServer.Handler)
	defer ts.Close()

	t.Run("GET returns list", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/appointments")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("want 200, got %d", resp.StatusCode)
		}

		var list []clinic.Appointment
		if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}
		if len(list) != 2 {
			t.Errorf("want 2 seeded appointments, got %d", len(list))
		}
	})

	t.Run("POST creates appointment", func(t *testing.T) {
		future := time.Now().Add(48 * time.Hour).Format(time.RFC3339)
		body, _ := json.Marshal(clinic.CreateInput{
			Patient: "Test Patient",
			Reason:  "Test Reason",
			Time:    future,
		})

		resp, err := http.Post(ts.URL+"/api/v1/appointments", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("want 201, got %d", resp.StatusCode)
		}

		var a clinic.Appointment
		if err := json.NewDecoder(resp.Body).Decode(&a); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}
		if a.Patient != "Test Patient" {
			t.Errorf("want Test Patient, got %s", a.Patient)
		}
	})

	t.Run("POST rejects invalid bodies", func(t *testing.T) {
		for _, body := range []string{
			`{}`,
			`{"patient":"A","reason":"B","time":"2030-01-01T00:00:00Z","extra":true}`,
			`{"patient":"A","reason":"B","time":"2030-01-01T00:00:00Z"} {}`,
			`{"patient":"A","reason":"B","time":"2030-01-01T00:00:00Z"}` + string(bytes.Repeat([]byte(" "), 1<<20)),
		} {
			resp, err := http.Post(ts.URL+"/api/v1/appointments", "application/json", bytes.NewBufferString(body))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("body %q: want 400, got %d", body[:min(len(body), 80)], resp.StatusCode)
			}
		}
	})

	t.Run("POST rejects non-JSON content type", func(t *testing.T) {
		resp, err := http.Post(ts.URL+"/api/v1/appointments", "text/plain", bytes.NewBufferString(`{"patient":"A","reason":"B","time":"2030-01-01T00:00:00Z"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnsupportedMediaType {
			t.Errorf("want 415, got %d", resp.StatusCode)
		}
	})

	t.Run("POST rejects oversized fields", func(t *testing.T) {
		body, _ := json.Marshal(clinic.CreateInput{
			Patient: strings.Repeat("a", 500),
			Reason:  "Test Reason",
			Time:    time.Now().Add(48 * time.Hour).Format(time.RFC3339),
		})

		resp, err := http.Post(ts.URL+"/api/v1/appointments", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("want 400, got %d", resp.StatusCode)
		}
	})

	t.Run("PUT not allowed", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/v1/appointments", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("want 405, got %d", resp.StatusCode)
		}
	})
}

func TestAppointmentHandler(t *testing.T) {
	s := New(0)
	ts := httptest.NewServer(s.httpServer.Handler)
	defer ts.Close()

	t.Run("GET existing id", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/appointments/apt-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("want 200, got %d", resp.StatusCode)
		}
	})

	t.Run("GET missing id returns 404", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/appointments/missing")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("want 404, got %d", resp.StatusCode)
		}
	})

	t.Run("DELETE cancels appointment", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/appointments/apt-1", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d", resp.StatusCode)
		}

		var a clinic.Appointment
		if err := json.NewDecoder(resp.Body).Decode(&a); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}
		if a.Status != clinic.StatusCancelled {
			t.Errorf("status = %s, want cancelled", a.Status)
		}
	})

	t.Run("DELETE twice returns 409", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/appointments/apt-1", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusConflict {
			t.Errorf("want 409, got %d", resp.StatusCode)
		}
	})

	t.Run("DELETE missing id returns 404", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/appointments/missing", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("want 404, got %d", resp.StatusCode)
		}
	})

	t.Run("PATCH not allowed", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/v1/appointments/apt-2", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("want 405, got %d", resp.StatusCode)
		}
	})
}

func TestMetricsHandler(t *testing.T) {
	s := New(0)
	ts := httptest.NewServer(s.httpServer.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	resp, err = http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	out := string(body)

	for _, want := range []string{
		"# TYPE healthd_http_requests_total counter",
		`healthd_http_requests_total{method="GET",route="/health",code="200"}`,
		"# TYPE healthd_appointments gauge",
		`healthd_appointments{status="scheduled"} 2`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("metrics output missing %q\ngot:\n%s", want, out)
		}
	}
}
