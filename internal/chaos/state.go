package chaos

import (
	"context"
	"sync"
)

type HTTPState struct {
	LatencyMS int     `json:"latency_ms"`
	ErrorRate float64 `json:"error_rate"`
	Status    int     `json:"status"`
}

type ResourcesState struct {
	MemoryLeakMBPerMin int `json:"memory_leak_mb_per_min"`
}

// AllowOverbooking is the only chaos switch affecting domain logic rather than
// HTTP responses; it travels through BookOptions to skip the capacity rule in CheckSeats.
type BookingState struct {
	AllowOverbooking bool `json:"allow_overbooking"`
}

type State struct {
	HTTP      HTTPState      `json:"http"`
	Readiness string         `json:"readiness"`
	Resources ResourcesState `json:"resources"`
	Booking   BookingState   `json:"booking"`
}

func DefaultState() State {
	return State{Readiness: "ok"}
}

type HTTPPatch struct {
	LatencyMS *int     `json:"latency_ms"`
	ErrorRate *float64 `json:"error_rate"`
	Status    *int     `json:"status"`
}

type ResourcesPatch struct {
	MemoryLeakMBPerMin *int `json:"memory_leak_mb_per_min"`
}

type BookingPatch struct {
	AllowOverbooking *bool `json:"allow_overbooking"`
}

// Patch mirrors State with pointers, so a field the request did not mention
// stays nil and is left untouched. That is the whole partial-update mechanism.
type Patch struct {
	HTTP      *HTTPPatch      `json:"http"`
	Readiness *string         `json:"readiness"`
	Resources *ResourcesPatch `json:"resources"`
	Booking   *BookingPatch   `json:"booking"`
}

func (s State) Apply(p Patch) State {
	if p.HTTP != nil {
		if p.HTTP.LatencyMS != nil {
			s.HTTP.LatencyMS = *p.HTTP.LatencyMS
		}
		if p.HTTP.ErrorRate != nil {
			s.HTTP.ErrorRate = *p.HTTP.ErrorRate
		}
		if p.HTTP.Status != nil {
			s.HTTP.Status = *p.HTTP.Status
		}
	}
	if p.Readiness != nil {
		s.Readiness = *p.Readiness
	}
	if p.Resources != nil && p.Resources.MemoryLeakMBPerMin != nil {
		s.Resources.MemoryLeakMBPerMin = *p.Resources.MemoryLeakMBPerMin
	}
	if p.Booking != nil && p.Booking.AllowOverbooking != nil {
		s.Booking.AllowOverbooking = *p.Booking.AllowOverbooking
	}
	return s
}

type Store interface {
	Get(ctx context.Context) (State, error)
	Apply(ctx context.Context, p Patch) (State, error)
	Reset(ctx context.Context) (State, error)
}

// MemoryStore keeps the state in the instance that was asked. With several
// replicas a POST therefore reaches exactly one of them, which is the lesson
// behind CHAOS_STORE=db in a later release.
type MemoryStore struct {
	mu    sync.RWMutex
	state State
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{state: DefaultState()}
}

func (m *MemoryStore) Get(ctx context.Context) (State, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state, nil
}

func (m *MemoryStore) Apply(ctx context.Context, p Patch) (State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = m.state.Apply(p)
	return m.state, nil
}

func (m *MemoryStore) Reset(ctx context.Context) (State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = DefaultState()
	return m.state, nil
}
