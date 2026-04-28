package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// SettingsJson represents the structure of settings.json files
// Matches src/utils/settings/types.ts SettingsSchema
type SettingsJson struct {
	Schema                *string                    `json:"$schema,omitempty"`
	Permissions           *PermissionsSettings       `json:"permissions,omitempty"`
	Env                   map[string]string          `json:"env,omitempty"`
	Model                 *string                    `json:"model,omitempty"`
	RespectGitignore      *bool                      `json:"respectGitignore,omitempty"`
	CleanupPeriodDays     *int                       `json:"cleanupPeriodDays,omitempty"`
	IncludeCoAuthoredBy   *bool                      `json:"includeCoAuthoredBy,omitempty"`
	IncludeGitInstructions *bool                     `json:"includeGitInstructions,omitempty"`
	AdditionalFields      map[string]json.RawMessage `json:"-"` // Preserve unknown fields
}

// PermissionsSettings represents the permissions section
// Matches src/utils/settings/types.ts PermissionsSchema
type PermissionsSettings struct {
	Allow                 []string `json:"allow,omitempty"`
	Deny                  []string `json:"deny,omitempty"`
	Ask                   []string `json:"ask,omitempty"`
	DefaultMode           *string  `json:"defaultMode,omitempty"`
	DisableBypassMode     *string  `json:"disableBypassPermissionsMode,omitempty"`
	AdditionalDirectories []string `json:"additionalDirectories,omitempty"`
}

// SettingSource represents where settings come from
type SettingSource string

const (
	SourceUserSettings    SettingSource = "userSettings"
	SourceProjectSettings SettingSource = "projectSettings"
	SourceLocalSettings   SettingSource = "localSettings"
	SourcePolicySettings  SettingSource = "policySettings"
	SourceFlagSettings    SettingSource = "flagSettings"
)

// SettingsManager handles loading and saving settings files
type SettingsManager struct {
	mu            sync.RWMutex
	workingDir    string
	localSettings *SettingsJson
	userSettings  *SettingsJson
}

// Global settings manager instance
var globalSettingsManager *SettingsManager
var settingsOnce sync.Once

// GetSettingsManager returns the global settings manager
func GetSettingsManager() *SettingsManager {
	settingsOnce.Do(func() {
		globalSettingsManager = createSettingsManager()
	})
	return globalSettingsManager
}

// createSettingsManager creates a new settings manager
func createSettingsManager() *SettingsManager {
	workingDir, err := os.Getwd()
	if err != nil {
		workingDir = "."
	}
	sm := &SettingsManager{
		workingDir: workingDir,
	}
	sm.loadAllSettings()
	return sm
}

// GetRelativeSettingsFilePath returns the relative path for a settings source
// Matches src/utils/settings/settings.ts getRelativeSettingsFilePathForSource
func GetRelativeSettingsFilePath(source SettingSource) string {
	switch source {
	case SourceProjectSettings:
		return filepath.Join(".claude-go", "settings.json")
	case SourceLocalSettings:
		return filepath.Join(".claude-go", "settings.local.json")
	default:
		return ""
	}
}

// GetSettingsFilePath returns the absolute path for a settings source
func GetSettingsFilePath(source SettingSource, workingDir string) string {
	relative := GetRelativeSettingsFilePath(source)
	if relative == "" {
		return ""
	}
	return filepath.Join(workingDir, relative)
}

// GetUserSettingsFilePath returns the path to user-level settings
// Uses Go CLI specific directory
func GetUserSettingsFilePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".claude-go", "settings.json")
}

// loadAllSettings loads settings from all sources
func (sm *SettingsManager) loadAllSettings() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Load user settings (global)
	userPath := GetUserSettingsFilePath()
	if userPath != "" {
		sm.userSettings = loadSettingsFile(userPath)
	}

	// Load local settings (project-specific, gitignored)
	localPath := GetSettingsFilePath(SourceLocalSettings, sm.workingDir)
	if localPath != "" {
		sm.localSettings = loadSettingsFile(localPath)
	}
}

// loadSettingsFile loads a settings file from disk
func loadSettingsFile(path string) *SettingsJson {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &SettingsJson{}
		}
		return &SettingsJson{}
	}

	if len(strings.TrimSpace(string(data))) == 0 {
		return &SettingsJson{}
	}

	var settings SettingsJson
	// First pass: parse known fields
	if err := json.Unmarshal(data, &settings); err != nil {
		return &SettingsJson{}
	}

	// Second pass: preserve unknown fields for backward compatibility
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err == nil {
		settings.AdditionalFields = make(map[string]json.RawMessage)
		knownFields := []string{
			"$schema", "permissions", "env", "model",
			"respectGitignore", "cleanupPeriodDays",
			"includeCoAuthoredBy", "includeGitInstructions",
		}
		for key, value := range raw {
			isKnown := false
			for _, known := range knownFields {
				if key == known {
					isKnown = true
					break
				}
			}
			if !isKnown {
				settings.AdditionalFields[key] = value
			}
		}
	}

	return &settings
}

// saveSettingsFile saves a settings file to disk
func saveSettingsFile(path string, settings *SettingsJson) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create settings directory: %w", err)
	}

	// Build JSON with both known and unknown fields
	raw := make(map[string]json.RawMessage)

	// Add known fields if non-nil
	if settings.Schema != nil {
		raw["$schema"] = json.RawMessage(fmt.Sprintf(`"%s"`, *settings.Schema))
	}
	if settings.Permissions != nil {
		permJSON, _ := json.Marshal(settings.Permissions)
		raw["permissions"] = permJSON
	}
	if len(settings.Env) > 0 {
		envJSON, _ := json.Marshal(settings.Env)
		raw["env"] = envJSON
	}
	if settings.Model != nil {
		raw["model"] = json.RawMessage(fmt.Sprintf(`"%s"`, *settings.Model))
	}
	if settings.RespectGitignore != nil {
		raw["respectGitignore"] = json.RawMessage(fmt.Sprintf(`%v`, *settings.RespectGitignore))
	}
	if settings.CleanupPeriodDays != nil {
		raw["cleanupPeriodDays"] = json.RawMessage(fmt.Sprintf(`%d`, *settings.CleanupPeriodDays))
	}
	if settings.IncludeCoAuthoredBy != nil {
		raw["includeCoAuthoredBy"] = json.RawMessage(fmt.Sprintf(`%v`, *settings.IncludeCoAuthoredBy))
	}
	if settings.IncludeGitInstructions != nil {
		raw["includeGitInstructions"] = json.RawMessage(fmt.Sprintf(`%v`, *settings.IncludeGitInstructions))
	}

	// Add unknown/preserved fields
	for key, value := range settings.AdditionalFields {
		raw[key] = value
	}

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	// Write to file
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// GetSettingsForSource returns settings for a specific source
func (sm *SettingsManager) GetSettingsForSource(source SettingSource) *SettingsJson {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	switch source {
	case SourceUserSettings:
		if sm.userSettings != nil {
			return sm.userSettings
		}
		return &SettingsJson{}
	case SourceLocalSettings:
		if sm.localSettings != nil {
			return sm.localSettings
		}
		return &SettingsJson{}
	default:
		return &SettingsJson{}
	}
}

// GetMergedSettings returns merged settings from all sources
// Priority: userSettings < projectSettings < localSettings
func (sm *SettingsManager) GetMergedSettings() *SettingsJson {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	merged := &SettingsJson{}

	// Merge user settings first (lowest priority)
	if sm.userSettings != nil {
		mergeSettings(merged, sm.userSettings)
	}

	// Merge local settings (highest priority for user-editable)
	if sm.localSettings != nil {
		mergeSettings(merged, sm.localSettings)
	}

	return merged
}

// mergeSettings merges source into target (target wins on conflicts for arrays)
func mergeSettings(target, source *SettingsJson) {
	if source.Permissions != nil {
		if target.Permissions == nil {
			target.Permissions = &PermissionsSettings{}
		}
		// Merge permission arrays (concatenate and dedupe)
		target.Permissions.Allow = mergeStringArrays(target.Permissions.Allow, source.Permissions.Allow)
		target.Permissions.Deny = mergeStringArrays(target.Permissions.Deny, source.Permissions.Deny)
		target.Permissions.Ask = mergeStringArrays(target.Permissions.Ask, source.Permissions.Ask)
		if source.Permissions.DefaultMode != nil && target.Permissions.DefaultMode == nil {
			target.Permissions.DefaultMode = source.Permissions.DefaultMode
		}
		target.Permissions.AdditionalDirectories = mergeStringArrays(
			target.Permissions.AdditionalDirectories,
			source.Permissions.AdditionalDirectories,
		)
	}

	// Merge env
	for k, v := range source.Env {
		if target.Env == nil {
			target.Env = make(map[string]string)
		}
		if _, exists := target.Env[k]; !exists {
			target.Env[k] = v
		}
	}

	// Merge other fields (source wins if target is nil)
	if source.Model != nil && target.Model == nil {
		target.Model = source.Model
	}
}

// mergeStringArrays merges two string arrays, deduplicating
func mergeStringArrays(a, b []string) []string {
	seen := make(map[string]bool)
	result := []string{}
	for _, s := range a {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	for _, s := range b {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// UpdateSettingsForSource updates settings for a specific source
// Matches src/utils/settings/settings.ts updateSettingsForSource
func (sm *SettingsManager) UpdateSettingsForSource(source SettingSource, updates *SettingsJson) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var settings *SettingsJson
	var path string

	switch source {
	case SourceLocalSettings:
		path = GetSettingsFilePath(SourceLocalSettings, sm.workingDir)
		settings = sm.localSettings
		if settings == nil {
			settings = &SettingsJson{}
		}
	case SourceUserSettings:
		path = GetUserSettingsFilePath()
		settings = sm.userSettings
		if settings == nil {
			settings = &SettingsJson{}
		}
	default:
		return fmt.Errorf("unsupported settings source: %s", source)
	}

	// Merge updates into existing settings
	mergeSettings(settings, updates)

	// Save to disk
	if err := saveSettingsFile(path, settings); err != nil {
		return err
	}

	// Update in-memory cache
	switch source {
	case SourceLocalSettings:
		sm.localSettings = settings
	case SourceUserSettings:
		sm.userSettings = settings
	}

	// Add to .gitignore if local settings
	if source == SourceLocalSettings {
		gitignorePath := filepath.Join(sm.workingDir, ".gitignore")
		addToGitignore(gitignorePath, GetRelativeSettingsFilePath(SourceLocalSettings))
	}

	return nil
}

// addToGitignore adds a pattern to .gitignore if not already present
func addToGitignore(gitignorePath, pattern string) {
	data, err := os.ReadFile(gitignorePath)
	if err != nil && !os.IsNotExist(err) {
		return
	}

	content := ""
	if data != nil {
		content = string(data)
	}

	// Check if pattern already exists
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == pattern {
			return // Already in gitignore
		}
	}

	// Append pattern
	var newData []byte
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		newData = []byte(content + "\n" + pattern + "\n")
	} else {
		newData = []byte(content + pattern + "\n")
	}

	os.WriteFile(gitignorePath, newData, 0644)
}

// AddPermissionRule adds a permission rule to settings
// Matches src/utils/permissions/PermissionUpdate.ts persistPermissionUpdate
func (sm *SettingsManager) AddPermissionRule(rule string, behavior string, source SettingSource) error {
	updates := &SettingsJson{
		Permissions: &PermissionsSettings{},
	}

	switch behavior {
	case "allow":
		updates.Permissions.Allow = []string{rule}
	case "deny":
		updates.Permissions.Deny = []string{rule}
	case "ask":
		updates.Permissions.Ask = []string{rule}
	default:
		return fmt.Errorf("invalid behavior: %s", behavior)
	}

	return sm.UpdateSettingsForSource(source, updates)
}

// RemovePermissionRule removes a permission rule from settings
func (sm *SettingsManager) RemovePermissionRule(rule string, behavior string, source SettingSource) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var settings *SettingsJson
	var path string

	switch source {
	case SourceLocalSettings:
		path = GetSettingsFilePath(SourceLocalSettings, sm.workingDir)
		settings = sm.localSettings
	case SourceUserSettings:
		path = GetUserSettingsFilePath()
		settings = sm.userSettings
	default:
		return fmt.Errorf("unsupported settings source: %s", source)
	}

	if settings == nil || settings.Permissions == nil {
		return nil
	}

	// Remove the rule
	switch behavior {
	case "allow":
		settings.Permissions.Allow = removeFromSlice(settings.Permissions.Allow, rule)
	case "deny":
		settings.Permissions.Deny = removeFromSlice(settings.Permissions.Deny, rule)
	case "ask":
		settings.Permissions.Ask = removeFromSlice(settings.Permissions.Ask, rule)
	}

	// Save to disk
	if err := saveSettingsFile(path, settings); err != nil {
		return err
	}

	return nil
}

// removeFromSlice removes a string from a slice
func removeFromSlice(slice []string, item string) []string {
	result := []string{}
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}

// GetPermissionRules returns all permission rules of a specific behavior
func (sm *SettingsManager) GetPermissionRules(behavior string, source SettingSource) []string {
	settings := sm.GetSettingsForSource(source)
	if settings == nil || settings.Permissions == nil {
		return []string{}
	}

	switch behavior {
	case "allow":
		return settings.Permissions.Allow
	case "deny":
		return settings.Permissions.Deny
	case "ask":
		return settings.Permissions.Ask
	default:
		return []string{}
	}
}

// GetAllPermissionRules returns all permission rules merged from all sources
func (sm *SettingsManager) GetAllPermissionRules(behavior string) []string {
	merged := sm.GetMergedSettings()
	if merged == nil || merged.Permissions == nil {
		return []string{}
	}

	switch behavior {
	case "allow":
		return merged.Permissions.Allow
	case "deny":
		return merged.Permissions.Deny
	case "ask":
		return merged.Permissions.Ask
	default:
		return []string{}
	}
}

// InitSettingsDirectory creates the .claude-go directory if needed
func InitSettingsDirectory(workingDir string) error {
	claudeDir := filepath.Join(workingDir, ".claude-go")
	return os.MkdirAll(claudeDir, 0755)
}

// ResetSettingsManager resets the global settings manager (for testing)
func ResetSettingsManager() {
	settingsOnce = sync.Once{}
	globalSettingsManager = nil
}

// SetWorkingDir sets the working directory for the settings manager
func SetWorkingDir(workingDir string) {
	sm := GetSettingsManager()
	sm.mu.Lock()
	sm.workingDir = workingDir
	sm.mu.Unlock()
	sm.loadAllSettings()
}