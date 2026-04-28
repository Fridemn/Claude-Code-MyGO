package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

const apiProfilesFileName = "api-profiles.json"

type APIProfile struct {
	Name         string `json:"name"`
	APIKey       string `json:"api_key"`
	BaseURL      string `json:"base_url"`
	Model        string `json:"model"`
	SummaryModel string `json:"summary_model,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
	UpdatedAt    string `json:"updated_at,omitempty"`
}

type APIProfilesStore struct {
	Active   string                `json:"active,omitempty"`
	Profiles map[string]APIProfile `json:"profiles,omitempty"`
}

func APIProfilesPath() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".claude-go", apiProfilesFileName)
	}
	return filepath.Join(".claude-go", apiProfilesFileName)
}

func LoadAPIProfiles() (*APIProfilesStore, error) {
	path := APIProfilesPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &APIProfilesStore{Profiles: map[string]APIProfile{}}, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return &APIProfilesStore{Profiles: map[string]APIProfile{}}, nil
	}
	var store APIProfilesStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("parse API profiles: %w", err)
	}
	if store.Profiles == nil {
		store.Profiles = map[string]APIProfile{}
	}
	return &store, nil
}

func SaveAPIProfiles(store *APIProfilesStore) error {
	if store == nil {
		store = &APIProfilesStore{}
	}
	if store.Profiles == nil {
		store.Profiles = map[string]APIProfile{}
	}
	path := APIProfilesPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func (s *APIProfilesStore) SortedProfiles() []APIProfile {
	if s == nil {
		return nil
	}
	out := make([]APIProfile, 0, len(s.Profiles))
	for _, profile := range s.Profiles {
		out = append(out, profile)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func (s *APIProfilesStore) Upsert(profile APIProfile) error {
	if s.Profiles == nil {
		s.Profiles = map[string]APIProfile{}
	}
	profile = SanitizeAPIProfile(profile)
	if profile.Name == "" {
		return fmt.Errorf("profile name is required")
	}
	if profile.APIKey == "" {
		return fmt.Errorf("api key is required")
	}
	if profile.BaseURL == "" {
		return fmt.Errorf("base URL is required")
	}
	if profile.Model == "" {
		return fmt.Errorf("model is required")
	}
	if err := validateProfileBaseURL(profile.BaseURL); err != nil {
		return err
	}
	now := time.Now().Format(time.RFC3339)
	if existing, ok := s.Profiles[profile.Name]; ok && existing.CreatedAt != "" {
		profile.CreatedAt = existing.CreatedAt
	} else if profile.CreatedAt == "" {
		profile.CreatedAt = now
	}
	profile.UpdatedAt = now
	s.Profiles[profile.Name] = profile
	return nil
}

func (s *APIProfilesStore) Remove(name string) bool {
	name = strings.TrimSpace(name)
	if s == nil || name == "" {
		return false
	}
	if _, ok := s.Profiles[name]; !ok {
		return false
	}
	delete(s.Profiles, name)
	if s.Active == name {
		s.Active = ""
	}
	return true
}

func ApplyAPIProfile(cfg Config, profile APIProfile) Config {
	profile = SanitizeAPIProfile(profile)
	if profile.APIKey != "" {
		cfg.APIKey = profile.APIKey
	}
	if profile.BaseURL != "" {
		cfg.BaseURL = profile.BaseURL
	}
	if profile.Model != "" {
		cfg.Model = profile.Model
	}
	cfg.SummaryModel = profile.SummaryModel
	return cfg
}

func ApplyActiveAPIProfile(cfg Config) (Config, string, error) {
	store, err := LoadAPIProfiles()
	if err != nil {
		return cfg, "", err
	}
	if store.Active == "" {
		return cfg, "", nil
	}
	profile, ok := store.Profiles[store.Active]
	if !ok {
		return cfg, "", nil
	}
	return ApplyAPIProfile(cfg, profile), store.Active, nil
}

func SanitizeAPIProfile(profile APIProfile) APIProfile {
	profile.Name = CleanConfigValue(profile.Name)
	profile.APIKey = CleanConfigValue(profile.APIKey)
	profile.BaseURL = CleanConfigValue(profile.BaseURL)
	profile.Model = CleanConfigValue(profile.Model)
	profile.SummaryModel = CleanConfigValue(profile.SummaryModel)
	return profile
}

func CleanConfigValue(value string) string {
	value = strings.Map(func(r rune) rune {
		switch r {
		case '\x00', '\ufeff':
			return -1
		case '\r', '\n', '\t':
			return -1
		default:
			if unicode.IsControl(r) {
				return -1
			}
			return r
		}
	}, value)
	return strings.TrimSpace(value)
}

func validateProfileBaseURL(raw string) error {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return fmt.Errorf("invalid base URL %q: %w", raw, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("invalid base URL %q: scheme must be http or https", raw)
	}
	if parsed.Host == "" {
		return fmt.Errorf("invalid base URL %q: host is required", raw)
	}
	return nil
}
