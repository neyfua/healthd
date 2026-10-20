package clinic

import (
	"slices"
	"testing"
	"time"
)

func TestStore_List(t *testing.T) {
	s := NewStore()
	list := s.List()

	if len(list) != 2 {
		t.Fatalf("want 2 seeded appointments, got %d", len(list))
	}
}

func TestStore_Get(t *testing.T) {
	s := NewStore()

	a, err := s.Get("apt-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Patient != "Alex Rivera" {
		t.Errorf("want Alex Rivera, got %s", a.Patient)
	}

	_, err = s.Get("missing")
	if err != ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestStore_Create_Valid(t *testing.T) {
	s := NewStore()
	future := time.Now().Add(24 * time.Hour).Format(time.RFC3339)

	a, err := s.Create(CreateInput{
		Patient: "New Patient",
		Reason:  "Consultation",
		Time:    future,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.ID == "" {
		t.Error("want non-empty ID")
	}
	if a.Patient != "New Patient" {
		t.Errorf("want New Patient, got %s", a.Patient)
	}
	if a.Status != StatusScheduled {
		t.Errorf("want scheduled, got %s", a.Status)
	}

	list := s.List()
	if len(list) != 3 {
		t.Errorf("want 3 appointments after create, got %d", len(list))
	}
}

func TestStore_Create_InvalidMissingField(t *testing.T) {
	s := NewStore()
	future := time.Now().Add(24 * time.Hour).Format(time.RFC3339)

	tests := []CreateInput{
		{Patient: "", Reason: "Checkup", Time: future},
		{Patient: "Patient", Reason: "", Time: future},
		{Patient: "Patient", Reason: "Checkup", Time: ""},
	}

	for _, tc := range tests {
		_, err := s.Create(tc)
		if err != ErrInvalidInput {
			t.Errorf("want ErrInvalidInput for %#v, got %v", tc, err)
		}
	}
}

func TestStore_Create_InvalidTimeFormat(t *testing.T) {
	s := NewStore()
	_, err := s.Create(CreateInput{
		Patient: "Patient",
		Reason:  "Checkup",
		Time:    "not-a-time",
	})
	if err != ErrInvalidInput {
		t.Errorf("want ErrInvalidInput for bad time format, got %v", err)
	}
}

func TestStore_Create_InvalidTimeInPast(t *testing.T) {
	s := NewStore()
	past := time.Now().Add(-time.Hour).Format(time.RFC3339)
	_, err := s.Create(CreateInput{
		Patient: "Patient",
		Reason:  "Checkup",
		Time:    past,
	})
	if err != ErrInvalidInput {
		t.Errorf("want ErrInvalidInput for past time, got %v", err)
	}
}

func TestStore_Cancel(t *testing.T) {
	s := NewStore()

	a, err := s.Cancel("apt-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Status != StatusCancelled {
		t.Errorf("want cancelled, got %s", a.Status)
	}

	got, err := s.Get("apt-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != StatusCancelled {
		t.Errorf("stored status = %s, want cancelled", got.Status)
	}

	_, err = s.Cancel("apt-1")
	if err != ErrConflict {
		t.Errorf("want ErrConflict on second cancel, got %v", err)
	}

	_, err = s.Cancel("missing")
	if err != ErrNotFound {
		t.Errorf("want ErrNotFound for unknown id, got %v", err)
	}
}

func TestStore_CreateEnforcesCapacity(t *testing.T) {
	s := NewStore()
	in := CreateInput{
		Patient: "Test",
		Reason:  "Checkup",
		Time:    time.Now().Add(24 * time.Hour).Format(time.RFC3339),
	}

	for i := len(s.byID); i < maxAppointments; i++ {
		if _, err := s.Create(in); err != nil {
			t.Fatalf("create %d: unexpected error: %v", i, err)
		}
	}
	if _, err := s.Create(in); err != ErrCapacity {
		t.Errorf("want ErrCapacity, got %v", err)
	}
}

func TestStore_CountByStatus(t *testing.T) {
	s := NewStore()

	if got := s.CountByStatus(StatusScheduled); got != 2 {
		t.Errorf("scheduled = %d, want 2", got)
	}
	if got := s.CountByStatus(StatusCancelled); got != 0 {
		t.Errorf("cancelled = %d, want 0", got)
	}

	if _, err := s.Cancel("apt-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := s.CountByStatus(StatusScheduled); got != 1 {
		t.Errorf("scheduled = %d, want 1", got)
	}
	if got := s.CountByStatus(StatusCancelled); got != 1 {
		t.Errorf("cancelled = %d, want 1", got)
	}
}

func TestStore_List_DeterministicForEqualTimes(t *testing.T) {
	s := NewStore()
	same := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	// Insert out of ID order with identical times.
	for _, id := range []string{"b", "a", "c"} {
		s.byID[id] = Appointment{ID: id, Time: same, Status: StatusScheduled}
	}

	list := s.List()
	var ids []string
	for _, a := range list {
		if a.Time.Equal(same) {
			ids = append(ids, a.ID)
		}
	}
	want := []string{"a", "b", "c"}
	if !slices.Equal(ids, want) {
		t.Errorf("equal-time order = %v, want %v", ids, want)
	}
}
