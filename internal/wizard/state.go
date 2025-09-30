package wizard

import (
	"sync"
	"time"
)

type FlowState struct {
	Flow      string
	Step      int
	Data      map[string]string
	UpdatedAt time.Time
	ExpiresAt time.Time
}

type StateStore interface {
	Get(chatID int64) (FlowState, bool)
	Set(chatID int64, state FlowState)
	Reset(chatID int64)
}

type memoryStateStore struct {
	mu  sync.RWMutex
	ttl time.Duration
	m   map[int64]FlowState
}

func NewMemoryStateStore(ttl time.Duration) StateStore {
	return &memoryStateStore{ttl: ttl, m: make(map[int64]FlowState)}
}

func (s *memoryStateStore) Get(chatID int64) (FlowState, bool) {
	s.mu.RLock()
	state, ok := s.m[chatID]
	s.mu.RUnlock()
	if !ok {
		return FlowState{}, false
	}
	if time.Now().After(state.ExpiresAt) {
		s.Reset(chatID)
		return FlowState{}, false
	}
	return state, true
}

func (s *memoryStateStore) Set(chatID int64, state FlowState) {
	state.UpdatedAt = time.Now()
	state.ExpiresAt = state.UpdatedAt.Add(s.ttl)
	s.mu.Lock()
	s.m[chatID] = state
	s.mu.Unlock()
}

func (s *memoryStateStore) Reset(chatID int64) {
	s.mu.Lock()
	delete(s.m, chatID)
	s.mu.Unlock()
}
