package state

import (
	"sync"
	"time"

	"claude-go/internal/types"
)

type Store struct {
	mu    sync.RWMutex
	state AppState
}

func CreateStore(initial AppState) *Store {
	return &Store{state: initial}
}

func (s *Store) Snapshot() AppState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	copy := s.state
	copy.Messages = append([]types.Message(nil), s.state.Messages...)
	return copy
}

func (s *Store) SetLoading(isLoading bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.IsLoading = isLoading
	s.state.UpdatedAt = time.Now()
}

func (s *Store) SetThinking(isThinking bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.IsThinking = isThinking
	s.state.UpdatedAt = time.Now()
}

func (s *Store) AppendMessage(message types.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Messages = append(s.state.Messages, message)
	s.state.UpdatedAt = time.Now()
}
