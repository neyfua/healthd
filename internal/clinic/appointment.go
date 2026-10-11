package clinic

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrNotFound     = errors.New("appointment not found")
)

// Status values
const (
	StatusScheduled = "scheduled"
	StatusCancelled = "cancelled"
)

// Appointment represents a clinic appointment
type Appointment struct {
	ID      string    `json:"id"`
	Patient string    `json:"patient"`
	Reason  string    `json:"reason"`
	Time    time.Time `json:"time"`
	Status  string    `json:"status"`
}

// CreateInput is the request body for creating an appointment
type CreateInput struct {
	Patient string `json:"patient"`
	Reason  string `json:"reason"`
	Time    string `json:"time"` // RFC3339
}

// Store holds appointments in memory
type Store struct {
	mu     sync.RWMutex
	nextID int
	byID   map[string]Appointment
}

// NewStore creates a seeded in-memory store
func NewStore() *Store {
	s := &Store{
		nextID: 3,
		byID: map[string]Appointment{
			"apt-1": {
				ID:      "apt-1",
				Patient: "Alex Rivera",
				Reason:  "Annual checkup",
				Time:    time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC),
				Status:  StatusScheduled,
			},
			"apt-2": {
				ID:      "apt-2",
				Patient: "Sam Chen",
				Reason:  "Follow-up",
				Time:    time.Date(2026, 10, 16, 14, 30, 0, 0, time.UTC),
				Status:  StatusScheduled,
			},
		},
	}
	return s
}

// List returns all appointments sorted by time
func (s *Store) List() []Appointment {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Appointment, 0, len(s.byID))
	for _, a := range s.byID {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	return out
}

// Get returns an appointment by ID or ErrNotFound
func (s *Store) Get(id string) (Appointment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	a, ok := s.byID[id]
	if !ok {
		return Appointment{}, ErrNotFound
	}
	return a, nil
}

// Create validates input and adds a new appointment
func (s *Store) Create(in CreateInput) (Appointment, error) {
	in.Patient = strings.TrimSpace(in.Patient)
	in.Reason = strings.TrimSpace(in.Reason)
	if in.Patient == "" || in.Reason == "" || in.Time == "" {
		return Appointment{}, ErrInvalidInput
	}
	t, err := time.Parse(time.RFC3339, in.Time)
	if err != nil || !t.After(time.Now().Add(-time.Minute)) {
		return Appointment{}, ErrInvalidInput
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id := "apt-" + strconv.Itoa(s.nextID)
	s.nextID++

	a := Appointment{
		ID:      id,
		Patient: in.Patient,
		Reason:  in.Reason,
		Time:    t,
		Status:  StatusScheduled,
	}
	s.byID[id] = a
	return a, nil
}
