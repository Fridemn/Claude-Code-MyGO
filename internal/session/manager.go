package session

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"claude-code-go/internal/utils"
)

type Manager struct {
	dir string
}

func CreateManager(dir string) (*Manager, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Manager{dir: dir}, nil
}

func (m *Manager) Create(_ context.Context) (*Session, error) {
	id, err := generateID()
	if err != nil {
		return nil, err
	}
	return &Session{ID: id}, nil
}

func (m *Manager) CreateWithID(id string) *Session {
	return &Session{ID: id}
}

func (m *Manager) Load(id string) (*Session, error) {
	path := filepath.Join(m.dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (m *Manager) Save(s *Session) error {
	path := filepath.Join(m.dir, s.ID+".json")
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func generateID() (string, error) {
	return utils.GenerateID("", 8)
}
