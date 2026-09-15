// Package agentsignal records opt-in working/idle hints from hooks and cmux.
package agentsignal

import (
	"sync"
	"time"
)

// Event is one hook or cmux status report.
type Event struct {
	PID   int
	Tool  string
	State string
	At    time.Time
}

// Store holds recent agent state signals.
type Store struct {
	mu     sync.RWMutex
	events []Event
}

// New returns an empty Store.
func New() *Store {
	return &Store{}
}

// Record appends a signal and drops events older than 2 minutes.
func (s *Store) Record(pid int, tool, state string) {
	s.mu.Lock()
	now := time.Now()
	s.events = append(s.events, Event{PID: pid, Tool: tool, State: state, At: now})
	cut := now.Add(-2 * time.Minute)
	i := 0
	for _, e := range s.events {
		if e.At.After(cut) {
			s.events[i] = e
			i++
		}
	}
	s.events = s.events[:i]
	s.mu.Unlock()
}

// Working reports whether pid has a working signal within d (and no newer idle).
func (s *Store) Working(pid int, within time.Duration) bool {
	return s.latest(pid, within) == "working"
}

// WorkingByTool matches tool name (claude, codex, cursor) when pid is 0.
func (s *Store) WorkingByTool(tool string, within time.Duration) bool {
	return s.latestTool(tool, within) == "working"
}

func (s *Store) latest(pid int, within time.Duration) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cut := time.Now().Add(-within)
	var state string
	var at time.Time
	for _, e := range s.events {
		if e.PID != pid || !e.At.After(cut) {
			continue
		}
		if e.At.After(at) {
			at = e.At
			state = e.State
		}
	}
	return state
}

func (s *Store) latestTool(tool string, within time.Duration) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cut := time.Now().Add(-within)
	var state string
	var at time.Time
	for _, e := range s.events {
		if e.Tool != tool || !e.At.After(cut) {
			continue
		}
		if e.At.After(at) {
			at = e.At
			state = e.State
		}
	}
	return state
}
