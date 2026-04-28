package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"claude-go/internal/types"
)

// HooksConfig represents the hooks configuration from settings.
type HooksConfig struct {
	Hooks map[string][]HookMatcherConfig `json:"hooks"`
}

// HookMatcherConfig represents a hook matcher from configuration.
type HookMatcherConfig struct {
	Matcher   string           `json:"matcher,omitempty"`
	IfExists  string           `json:"if,omitempty"`
	Hooks     []HookDefinition `json:"hooks"`
	PluginRoot string          `json:"pluginRoot,omitempty"`
	PluginID   string          `json:"pluginId,omitempty"`
	PluginName string          `json:"pluginName,omitempty"`
	SkillRoot  string          `json:"skillRoot,omitempty"`
	SkillName  string          `json:"skillName,omitempty"`
}

// HookDefinition represents a single hook from configuration.
type HookDefinition struct {
	Type          string            `json:"type"` // "command", "prompt", "http", "agent"
	Command       string            `json:"command,omitempty"`
	Prompt        string            `json:"prompt,omitempty"`
	URL           string            `json:"url,omitempty"`
	Shell         string            `json:"shell,omitempty"`
	Timeout       int               `json:"timeout,omitempty"`       // seconds
	TimeoutMs     int               `json:"timeout_ms,omitempty"`    // milliseconds
	Blocking      bool              `json:"blocking,omitempty"`
	Async         bool              `json:"async,omitempty"`
	Once          bool              `json:"once,omitempty"`
	StatusMessage string            `json:"statusMessage,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	Model         string            `json:"model,omitempty"`
}

// HooksSettingsManager manages loading hooks from settings files.
type HooksSettingsManager struct {
	configPath string
	config     *HooksConfig
}

// NewHooksSettingsManager creates a new hooks settings manager.
func NewHooksSettingsManager(configPath string) *HooksSettingsManager {
	return &HooksSettingsManager{
		configPath: configPath,
	}
}

// Load loads hooks configuration from the settings file.
func (m *HooksSettingsManager) Load() error {
	if m.configPath == "" {
		m.config = &HooksConfig{Hooks: make(map[string][]HookMatcherConfig)}
		return nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			m.config = &HooksConfig{Hooks: make(map[string][]HookMatcherConfig)}
			return nil
		}
		return fmt.Errorf("read hooks config: %w", err)
	}

	var config HooksConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("parse hooks config: %w", err)
	}

	if config.Hooks == nil {
		config.Hooks = make(map[string][]HookMatcherConfig)
	}

	m.config = &config
	return nil
}

// GetHooksForEvent returns hooks for a specific event.
func (m *HooksSettingsManager) GetHooksForEvent(event types.HookEvent) []HookMatcherConfig {
	if m.config == nil {
		return nil
	}
	return m.config.Hooks[string(event)]
}

// GetAllHooks returns all configured hooks.
func (m *HooksSettingsManager) GetAllHooks() map[string][]HookMatcherConfig {
	if m.config == nil {
		return nil
	}
	return m.config.Hooks
}

// AddHook adds a hook for an event.
func (m *HooksSettingsManager) AddHook(event types.HookEvent, matcher HookMatcherConfig) error {
	if m.config == nil {
		m.config = &HooksConfig{Hooks: make(map[string][]HookMatcherConfig)}
	}
	if m.config.Hooks == nil {
		m.config.Hooks = make(map[string][]HookMatcherConfig)
	}

	m.config.Hooks[string(event)] = append(m.config.Hooks[string(event)], matcher)
	return m.Save()
}

// RemoveHook removes a hook for an event by index.
func (m *HooksSettingsManager) RemoveHook(event types.HookEvent, index int) error {
	if m.config == nil || m.config.Hooks == nil {
		return nil
	}

	hooks := m.config.Hooks[string(event)]
	if index < 0 || index >= len(hooks) {
		return fmt.Errorf("invalid hook index %d", index)
	}

	m.config.Hooks[string(event)] = append(hooks[:index], hooks[index+1:]...)
	return m.Save()
}

// Save saves the hooks configuration to the settings file.
func (m *HooksSettingsManager) Save() error {
	if m.configPath == "" {
		return nil
	}

	// Ensure directory exists
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create hooks config directory: %w", err)
	}

	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal hooks config: %w", err)
	}

	return os.WriteFile(m.configPath, data, 0644)
}

// ConvertToHook converts a HookMatcherConfig to internal Hook type.
func ConvertToHook(matcher HookMatcherConfig, index int) Hook {
	var hooks []HookDefinition
	if len(matcher.Hooks) > 0 {
		hooks = matcher.Hooks
	} else {
		// Legacy format - single hook
		hooks = []HookDefinition{}
	}

	resultHooks := make([]Hook, 0, len(hooks))
	for i, def := range hooks {
		hook := Hook{
			Event:       fmt.Sprintf("hook_%d", i),
			Matcher:     matcher.Matcher,
			Command:     def.Command,
			Description: def.StatusMessage,
			Enabled:     true,
			Blocking:    def.Blocking,
			Shell:       def.Shell,
		}

		// Handle timeout (convert seconds to ms if needed)
		if def.TimeoutMs > 0 {
			hook.TimeoutMs = def.TimeoutMs
		} else if def.Timeout > 0 {
			hook.TimeoutMs = def.Timeout * 1000
		} else {
			hook.TimeoutMs = DefaultHookTimeoutMs
		}

		// Set source
		if matcher.PluginRoot != "" {
			hook.Source = "plugin:" + matcher.PluginName
		} else if matcher.SkillRoot != "" {
			hook.Source = "skill:" + matcher.SkillName
		} else {
			hook.Source = "user"
		}

		hook.Status = "loaded"
		resultHooks = append(resultHooks, hook)
	}

	if len(resultHooks) == 0 {
		return Hook{
			Matcher: matcher.Matcher,
			Source:  "user",
			Status:  "loaded",
			Enabled: true,
		}
	}

	return resultHooks[0]
}

// MatchesPattern checks if a query matches a pattern (glob-style).
func MatchesPattern(pattern, query string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	if query == "" {
		return false
	}

	// Simple glob matching
	if strings.Contains(pattern, "*") {
		// Convert glob to simple matching
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			prefix := parts[0]
			suffix := parts[1]
			return strings.HasPrefix(query, prefix) && strings.HasSuffix(query, suffix)
		}
		return strings.Contains(query, strings.ReplaceAll(pattern, "*", ""))
	}

	return pattern == query
}

// ParseHookEvent parses a string to a HookEvent.
func ParseHookEvent(s string) (types.HookEvent, bool) {
	event := types.HookEvent(s)
	for _, e := range types.AllHookEvents() {
		if e == event {
			return event, true
		}
	}
	return "", false
}

// HookType constants
const (
	HookTypeCommand = "command"
	HookTypePrompt  = "prompt"
	HookTypeHTTP    = "http"
	HookTypeAgent   = "agent"
)

// ShellType constants
const (
	ShellTypeBash       = "bash"
	ShellTypePowerShell = "powershell"
	ShellTypeSh         = "sh"
)

// DefaultHookShell returns the default shell for the current platform.
func DefaultHookShell() string {
	return ShellTypeBash
}