package stats

import (
	"context"
	"fmt"
	"os"
	"strings"

	"claude-go/internal/command"
)

// Effort levels matching TS src/utils/effort.ts
const (
	EffortLevelLow    = "low"
	EffortLevelMedium = "medium"
	EffortLevelHigh   = "high"
	EffortLevelMax    = "max"
)

var validEffortLevels = []string{EffortLevelLow, EffortLevelMedium, EffortLevelHigh, EffortLevelMax}

// getEffortLevelDescription returns user-facing description for effort levels
// TS src/utils/effort.ts:getEffortLevelDescription()
func getEffortLevelDescription(level string) string {
	switch level {
	case EffortLevelLow:
		return "Quick, straightforward implementation with minimal overhead"
	case EffortLevelMedium:
		return "Balanced approach with standard implementation and testing"
	case EffortLevelHigh:
		return "Comprehensive implementation with extensive testing and documentation"
	case EffortLevelMax:
		return "Maximum capability with deepest reasoning (Opus 4.6 only)"
	default:
		return "Balanced approach with standard implementation and testing"
	}
}

// isEffortLevel checks if value is a valid effort level
// TS src/utils/effort.ts:isEffortLevel()
func isEffortLevel(value string) bool {
	for _, level := range validEffortLevels {
		if level == value {
			return true
		}
	}
	return false
}

// getEffortEnvOverride checks for CLAUDE_CODE_EFFORT_LEVEL environment variable
// TS src/utils/effort.ts:getEffortEnvOverride()
func getEffortEnvOverride() string {
	envOverride := os.Getenv("CLAUDE_CODE_EFFORT_LEVEL")
	if envOverride == "" {
		return ""
	}
	lower := strings.ToLower(strings.TrimSpace(envOverride))
	if lower == "unset" || lower == "auto" {
		return "unset"
	}
	return lower
}

// toPersistableEffort determines if an effort level can be persisted to settings
// TS src/utils/effort.ts:toPersistableEffort()
// For personal users (not enterprise), 'max' is session-only
func toPersistableEffort(value string) string {
	if value == EffortLevelLow || value == EffortLevelMedium || value == EffortLevelHigh {
		return value
	}
	// 'max' is session-scoped for non-enterprise users
	return ""
}

// showCurrentEffort displays current effort level
// TS src/commands/effort/effort.tsx:showCurrentEffort()
func showCurrentEffort(appStateEffort string, model string) string {
	envOverride := getEffortEnvOverride()
	effectiveValue := appStateEffort
	if envOverride != "" {
		if envOverride == "unset" {
			effectiveValue = ""
		} else {
			effectiveValue = envOverride
		}
	}

	if effectiveValue == "" {
		// Default to 'high' when no explicit effort set (TS parity)
		level := "high"
		return fmt.Sprintf("Effort level: auto (currently %s)", level)
	}

	description := getEffortLevelDescription(effectiveValue)
	return fmt.Sprintf("Current effort level: %s (%s)", effectiveValue, description)
}

// executeEffort handles setting or unsetting effort level
// TS src/commands/effort/effort.tsx:executeEffort()
func executeEffort(args string, runtime command.Runtime) string {
	normalized := strings.ToLower(strings.TrimSpace(args))

	// Handle auto/unset
	if normalized == "auto" || normalized == "unset" {
		return unsetEffortLevel(runtime)
	}

	// Validate effort level
	if !isEffortLevel(normalized) {
		return fmt.Sprintf("Invalid argument: %s. Valid options are: low, medium, high, max, auto", args)
	}

	return setEffortValue(normalized, runtime)
}

// setEffortValue sets a specific effort level
// TS src/commands/effort/effort.tsx:setEffortValue()
func setEffortValue(effortValue string, runtime command.Runtime) string {
	persistable := toPersistableEffort(effortValue)

	// Check env override
	envOverride := getEffortEnvOverride()
	if envOverride != "" && envOverride != "unset" && envOverride != effortValue {
		envRaw := os.Getenv("CLAUDE_CODE_EFFORT_LEVEL")
		if persistable == "" {
			return fmt.Sprintf("Not applied: CLAUDE_CODE_EFFORT_LEVEL=%s overrides effort this session, and %s is session-only (nothing saved)", envRaw, effortValue)
		}
		return fmt.Sprintf("CLAUDE_CODE_EFFORT_LEVEL=%s overrides this session — clear it and %s takes over", envRaw, effortValue)
	}

	description := getEffortLevelDescription(effortValue)
	suffix := ""
	if persistable == "" {
		suffix = " (this session only)"
	}

	// Set effort in state
	if runtime.State != nil {
		runtime.State.SetSessionFlag("effortLevel", true)
		// Store the effort value in a custom session flag
		runtime.State.SetSessionFlag("effort_"+effortValue, true)
	}

	return fmt.Sprintf("Set effort level to %s%s: %s", effortValue, suffix, description)
}

// unsetEffortLevel clears effort level
// TS src/commands/effort/effort.tsx:unsetEffortLevel()
func unsetEffortLevel(runtime command.Runtime) string {
	envOverride := getEffortEnvOverride()
	if envOverride != "" && envOverride != "unset" {
		envRaw := os.Getenv("CLAUDE_CODE_EFFORT_LEVEL")
		return fmt.Sprintf("Cleared effort from settings, but CLAUDE_CODE_EFFORT_LEVEL=%s still controls this session", envRaw)
	}

	// Clear effort in state
	if runtime.State != nil {
		runtime.State.SetSessionFlag("effortLevel", false)
	}

	return "Effort level set to auto"
}

// getEffortFromState retrieves current effort level from state
func getEffortFromState(runtime command.Runtime) string {
	if runtime.State == nil {
		return ""
	}
	// Check session flags for effort
	for _, level := range validEffortLevels {
		if runtime.State.GetSessionFlag("effort_" + level) {
			return level
		}
	}
	return ""
}

func registerEffort(r *command.Registry) {
	r.Register(command.LegacyCommand{
		Type:        command.KindLocalJSX,
		Name:        "effort",
		Description: "set effort level for responses",
		Load:        loadEffortModel,
		Handler: func(ctx context.Context, runtime command.Runtime, args []string) (string, error) {
			// TS src/commands/effort/effort.tsx:call()
			argsStr := ""
			if len(args) > 0 {
				argsStr = strings.Join(args, " ")
			}
			argsStr = strings.TrimSpace(argsStr)

			// Handle help args
			commonHelpArgs := []string{"help", "-h", "--help"}
			for _, helpArg := range commonHelpArgs {
				if argsStr == helpArg {
					return `Usage: /effort [low|medium|high|max|auto]

Effort levels:
- low: Quick, straightforward implementation with minimal overhead
- medium: Balanced approach with standard implementation and testing
- high: Comprehensive implementation with extensive testing and documentation
- max: Maximum capability with deepest reasoning (Opus 4.6 only)
- auto: Use the default effort level for your model`, nil
				}
			}

			// Handle current/status or no args
			if argsStr == "" || argsStr == "current" || argsStr == "status" {
				appStateEffort := getEffortFromState(runtime)
				model := ""
				if runtime.State != nil {
					model = runtime.State.Snapshot().CurrentModel
				}
				return showCurrentEffort(appStateEffort, model), nil
			}

			return executeEffort(argsStr, runtime), nil
		},
	})
}