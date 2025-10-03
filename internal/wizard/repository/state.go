package repository

import (
	"sync"
	"time"
)

// Flow constants define the different conversational flows the wizard can be in.
const (
	FlowNone       = ""
	FlowCreateMenu = "create_menu"
	FlowCreateOTP  = "create_otp"
	FlowCreateQR   = "create_qr"
	FlowToken      = "token_login"
	FlowManage     = "manage_userbot"
	FlowAdmin      = "admin"
)

// FlowState represents the user's current position in a conversational flow.
type FlowState struct {
	Flow      string
	Step      int
	Data      map[string]string
	UpdatedAt time.Time
	ExpiresAt time.Time
}

// StateStore defines the interface for storing and retrieving user flow state.
type StateStore interface {
	Get(chatID int64) (FlowState, bool)
	Set(chatID int64, state FlowState)
	Reset(chatID int64)
}

// memoryStateStore is an in-memory implementation of the StateStore.
type memoryStateStore struct {
	mu  sync.RWMutex
	ttl time.Duration
	m   map[int64]FlowState
}

// NewMemoryStateStore creates a new in-memory state store.
func NewMemoryStateStore(ttl time.Duration) StateStore {
	return &memoryStateStore{ttl: ttl, m: make(map[int64]FlowState)}
}

// Get retrieves a user's state, checking for expiration.
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

// Set saves a user's state with a new TTL.
func (s *memoryStateStore) Set(chatID int64, state FlowState) {
	state.UpdatedAt = time.Now()
	state.ExpiresAt = state.UpdatedAt.Add(s.ttl)
	s.mu.Lock()
	s.m[chatID] = state
	s.mu.Unlock()
}

// Reset clears a user's state.
func (s *memoryStateStore) Reset(chatID int64) {
	s.mu.Lock()
	delete(s.m, chatID)
	s.mu.Unlock()
}