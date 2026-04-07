package config

import (
	"context"
	"fmt"
	"strings"

	"claude-code-go/internal/tool"
)

// ConfigTool implements the Config tool for getting and setting Claude Code settings
type ConfigTool struct{}

// Tool name constant
const ToolName = "Config"

// Description
const Description = "Get or set Claude Code configuration settings."

// SettingSource defines where a setting is stored
type SettingSource string

const (
	SourceGlobal   SettingSource = "global"
	SourceSettings SettingSource = "settings"
)

// SettingType defines the type of a setting
type SettingType string

const (
	TypeBoolean SettingType = "boolean"
	TypeString  SettingType = "string"
)

// SettingConfig defines a configurable setting
type SettingConfig struct {
	Source      SettingSource
	Type        SettingType
	Description string
	Path        []string
	Options     []string
}

// Supported settings registry
// This is a simplified version - the full TS version has more settings
// with feature flags and dynamic options
var supportedSettings = map[string]SettingConfig{
	"theme": {
		Source:      SourceGlobal,
		Type:        TypeString,
		Description: "Color theme for the UI",
		Options:     []string{"light", "dark", "auto"},
	},
	"editorMode": {
		Source:      SourceGlobal,
		Type:        TypeString,
		Description: "Key binding mode",
		Options:     []string{"default", "vim", "emacs"},
	},
	"verbose": {
		Source:      SourceGlobal,
		Type:        TypeBoolean,
		Description: "Show detailed debug output",
	},
	"autoCompactEnabled": {
		Source:      SourceGlobal,
		Type:        TypeBoolean,
		Description: "Auto-compact when context is full",
	},
	"autoMemoryEnabled": {
		Source:      SourceSettings,
		Type:        TypeBoolean,
		Description: "Enable auto-memory",
	},
	"showTurnDuration": {
		Source:      SourceGlobal,
		Type:        TypeBoolean,
		Description: "Show turn duration message after responses",
	},
	"terminalProgressBarEnabled": {
		Source:      SourceGlobal,
		Type:        TypeBoolean,
		Description: "Show OSC 9;4 progress indicator in supported terminals",
	},
	"todoFeatureEnabled": {
		Source:      SourceGlobal,
		Type:        TypeBoolean,
		Description: "Enable todo/task tracking",
	},
	"model": {
		Source:      SourceSettings,
		Type:        TypeString,
		Description: "Override the default model",
		Options:     []string{"sonnet", "opus", "haiku", "best"},
	},
	"permissions.defaultMode": {
		Source:      SourceSettings,
		Type:        TypeString,
		Description: "Default permission mode for tool usage",
		Options:     []string{"default", "plan", "acceptEdits", "dontAsk", "auto"},
	},
	"language": {
		Source:      SourceSettings,
		Type:        TypeString,
		Description: "Preferred language for Claude responses (e.g., \"japanese\", \"spanish\")",
	},
}

// isSettingSupported checks if a setting key is supported
func isSettingSupported(key string) bool {
	_, ok := supportedSettings[key]
	return ok
}

// getSettingConfig returns the config for a setting
func getSettingConfig(key string) (SettingConfig, bool) {
	cfg, ok := supportedSettings[key]
	return cfg, ok
}

// getSettingOptions returns the valid options for a setting
func getSettingOptions(key string) []string {
	cfg, ok := supportedSettings[key]
	if !ok {
		return nil
	}
	if len(cfg.Options) > 0 {
		return cfg.Options
	}
	return nil
}

// getSettingPath returns the path for a setting
func getSettingPath(key string) []string {
	cfg, ok := supportedSettings[key]
	if !ok {
		return nil
	}
	if len(cfg.Path) > 0 {
		return cfg.Path
	}
	// Default: split by dot
	return strings.Split(key, ".")
}

// Output represents the tool output
type Output struct {
	Success       bool   `json:"success"`
	Operation     string `json:"operation,omitempty"` // "get" or "set"
	Setting       string `json:"setting,omitempty"`
	Value         any    `json:"value,omitempty"`
	PreviousValue any    `json:"previousValue,omitempty"`
	NewValue      any    `json:"newValue,omitempty"`
	Error         string `json:"error,omitempty"`
}

// Name returns the tool name
func (ConfigTool) Name() string {
	return ToolName
}

// Description returns the tool description
func (ConfigTool) Description() string {
	return Description
}

// IsReadOnly returns true if the operation is a read (value not provided)
func (ConfigTool) IsReadOnly(in tool.Input) bool {
	_, hasValue := in["value"]
	return !hasValue
}

// ParametersSchema returns the JSON schema for the input
func (ConfigTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"setting": tool.SchemaString("The setting key (e.g., \"theme\", \"model\", \"permissions.defaultMode\")"),
		"value": map[string]any{
			"type":        "string",
			"description": "The new value. Omit to get current value.",
		},
	}, "setting")
}

// Call executes the tool
func (t ConfigTool) Call(ctx context.Context, in tool.Input, rt tool.Runtime) (tool.Result, error) {
	setting, _ := in["setting"].(string)
	if setting == "" {
		return tool.Result{Error: "setting parameter is required"}, nil
	}

	// Check if setting is supported
	if !isSettingSupported(setting) {
		return tool.Result{
			Content: Output{
				Success: false,
				Error:   fmt.Sprintf("Unknown setting: %q", setting),
			},
		}, nil
	}

	cfg, _ := getSettingConfig(setting)

	// Check for value parameter
	_, hasValue := in["value"]

	// GET operation
	if !hasValue {
		var currentValue any
		if cfg.Source == SourceGlobal && rt.Store != nil {
			// Try to get from store state
			state := rt.Store.Snapshot()
			switch setting {
			case "model":
				currentValue = state.CurrentModel
			case "verbose":
				currentValue = state.SessionFlags["verbose"]
			default:
				currentValue = nil
			}
		}

		return tool.Result{
			Content: Output{
				Success:   true,
				Operation: "get",
				Setting:   setting,
				Value:     currentValue,
			},
		}, nil
	}

	// SET operation
	value := in["value"]
	var finalValue any = value

	// Coerce and validate boolean values
	if cfg.Type == TypeBoolean {
		switch v := value.(type) {
		case bool:
			finalValue = v
		case string:
			lower := strings.ToLower(strings.TrimSpace(v))
			if lower == "true" {
				finalValue = true
			} else if lower == "false" {
				finalValue = false
			} else {
				return tool.Result{
					Content: Output{
						Success:   false,
						Operation: "set",
						Setting:   setting,
						Error:     fmt.Sprintf("%s requires true or false.", setting),
					},
				}, nil
			}
		default:
			return tool.Result{
				Content: Output{
					Success:   false,
					Operation: "set",
					Setting:   setting,
					Error:     fmt.Sprintf("%s requires true or false.", setting),
				},
			}, nil
		}
	}

	// Check options
	options := getSettingOptions(setting)
	if len(options) > 0 {
		strValue := fmt.Sprintf("%v", finalValue)
		valid := false
		for _, opt := range options {
			if opt == strValue {
				valid = true
				break
			}
		}
		if !valid {
			return tool.Result{
				Content: Output{
					Success:   false,
					Operation: "set",
					Setting:   setting,
					Error:     fmt.Sprintf("Invalid value %q. Options: %s", value, strings.Join(options, ", ")),
				},
			}, nil
		}
	}

	// Get previous value
	var previousValue any
	if rt.Store != nil {
		state := rt.Store.Snapshot()
		switch setting {
		case "model":
			previousValue = state.CurrentModel
		case "verbose":
			previousValue = state.SessionFlags["verbose"]
		}
	}

	// Write to storage
	if cfg.Source == SourceGlobal && rt.Store != nil {
		// For now, we handle a few known settings
		switch setting {
		case "model":
			if strVal, ok := finalValue.(string); ok {
				rt.Store.SetCurrentModel(strVal)
			}
		case "verbose":
			if boolVal, ok := finalValue.(bool); ok {
				state := rt.Store.Snapshot()
				state.SessionFlags["verbose"] = boolVal
			}
		}
	}

	return tool.Result{
		Content: Output{
			Success:       true,
			Operation:     "set",
			Setting:       setting,
			PreviousValue: previousValue,
			NewValue:      finalValue,
		},
	}, nil
}

// RegisterConfigTools registers config tools with the registry
func RegisterConfigTools(r *tool.Registry) {
	r.Register(ConfigTool{})
}