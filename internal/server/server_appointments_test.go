package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/neyfua/healthd/internal/clinic"
)

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
}
