package bash

import (
	"fmt"
	"strings"
	"sync"

	"claude-go/internal/settings"
	"claude-go/internal/tool/interaction"
)

// PermissionMode represents the permission mode for command execution
type PermissionMode string

const (
	PermissionModeAcceptEdits       PermissionMode = "acceptEdits"       // Accept all edits, prompt for dangerous
	PermissionModeLimitTools        PermissionMode = "limitTools"        // Limit to safe tools only
	PermissionModeBypassPermissions PermissionMode = "bypassPermissions" // Skip all permission checks
	PermissionModeAsk               PermissionMode = "ask"               // Ask for permission on each command
)

// PermissionRule represents a permission rule for bash commands
type PermissionRule struct {
	Pattern  string // Command pattern (e.g., "git *", "npm run:*")
	Type     PermissionRuleType
	Behavior PermissionBehavior
	Source   string // "userSettings", "env", etc.
}

// PermissionRuleType determines how the pattern is matched
type PermissionRuleType int

const (
	RuleTypeExact    PermissionRuleType = iota // Exact match
	RuleTypePrefix                             // Prefix match (e.g., "npm run:*")
	RuleTypeWildcard                           // Wildcard match
)

// PermissionBehavior determines what happens when a rule matches
type PermissionBehavior int

const (
	BehaviorAllow PermissionBehavior = iota // Allow without prompting
	BehaviorAsk                             // Ask user before executing
	BehaviorDeny                            // Deny execution
)

// PermissionResult represents the result of a permission check
type PermissionResult struct {
	Allowed     bool
	Reason      string
	Rule        *PermissionRule
	Suggestions []string
}

// PermissionRequest represents a pending permission request
type PermissionRequest struct {
	Command     string
	Description string
	Reasons     []string
	Denied      bool
}

// PermissionChecker handles permission checking for bash commands
type PermissionChecker struct {
	mu      sync.RWMutex
	mode    PermissionMode
	rules   []PermissionRule
	pending map[string]*PermissionRequest
}

// CreatePermissionChecker creates a new permission checker
func CreatePermissionChecker(mode PermissionMode) *PermissionChecker {
	return &PermissionChecker{
		mode:    mode,
		rules:   []PermissionRule{},
		pending: make(map[string]*PermissionRequest),
	}
}

// SetMode sets the permission mode
func (p *PermissionChecker) SetMode(mode PermissionMode) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.mode = mode
}

// GetMode gets the current permission mode
func (p *PermissionChecker) GetMode() PermissionMode {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.mode
}

// RulesSnapshot returns a copy of current permission rules.
func (p *PermissionChecker) RulesSnapshot() []PermissionRule {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]PermissionRule, len(p.rules))
	copy(out, p.rules)
	return out
}

// AddRule adds a permission rule
func (p *PermissionChecker) AddRule(rule PermissionRule) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rules = append(p.rules, rule)
}

// AddRules adds multiple permission rules
func (p *PermissionChecker) AddRules(rules []PermissionRule) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rules = append(p.rules, rules...)
}

// ClearRules removes all permission rules
func (p *PermissionChecker) ClearRules() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rules = nil
}

// Check checks if a command is allowed to execute
func (p *PermissionChecker) Check(command string, description string) PermissionResult {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Bypass permissions mode allows everything
	if p.mode == PermissionModeBypassPermissions {
		return PermissionResult{
			Allowed: true,
			Reason:  "bypassPermissions mode",
		}
	}

	// Limit tools mode restricts to known safe commands
	if p.mode == PermissionModeLimitTools {
		if !isSafeCommand(command) {
			return PermissionResult{
				Allowed: false,
				Reason:  "limitTools mode: only safe commands allowed",
			}
		}
		return PermissionResult{
			Allowed: true,
			Reason:  "limitTools mode: safe command",
		}
	}

	// Check against rules
	for _, rule := range p.rules {
		if matchRule(rule, command) {
			switch rule.Behavior {
			case BehaviorAllow:
				return PermissionResult{
					Allowed: true,
					Reason:  fmt.Sprintf("matched rule: %s", rule.Pattern),
					Rule:    &rule,
				}
			case BehaviorDeny:
				return PermissionResult{
					Allowed: false,
					Reason:  fmt.Sprintf("denied by rule: %s", rule.Pattern),
					Rule:    &rule,
				}
			case BehaviorAsk:
				return PermissionResult{
					Allowed: false,
					Reason:  fmt.Sprintf("requires permission: %s", rule.Pattern),
					Rule:    &rule,
					Suggestions: []string{
						fmt.Sprintf("Allow this command: %s", rule.Pattern),
						"Add to allowed patterns",
					},
				}
			}
		}
	}

	// Read-only/safe commands are auto-allowed in interactive ask mode.
	// This matches the practical CLI expectation that exploratory commands like
	// ls/cat/find/pwd shouldn't stall basic code analysis flows.
	if isSafeCommand(command) {
		return PermissionResult{
			Allowed: true,
			Reason:  "safe command",
		}
	}

	// Default behavior based on mode
	switch p.mode {
	case PermissionModeAcceptEdits:
		// In acceptEdits mode, allow safe commands, prompt for others
		if isSafeCommand(command) {
			return PermissionResult{
				Allowed: true,
				Reason:  "safe command",
			}
		}
		return PermissionResult{
			Allowed: false,
			Reason:  "command may modify files",
			Suggestions: []string{
				"Review the command before allowing",
				"Use acceptEdits mode to allow without prompting",
			},
		}
	case PermissionModeAsk:
		return PermissionResult{
			Allowed: false,
			Reason:  "permission required",
			Suggestions: []string{
				"Approve the command to continue",
				"Add a permission rule to allow without prompting",
			},
		}
	}

	return PermissionResult{
		Allowed: true,
		Reason:  "default allow",
	}
}

// matchRule checks if a command matches a rule
func matchRule(rule PermissionRule, command string) bool {
	switch rule.Type {
	case RuleTypeExact:
		return extractCommandBase(command) == rule.Pattern
	case RuleTypePrefix:
		cmd := extractCommandBase(command)
		return strings.HasPrefix(cmd, rule.Pattern) || cmd == rule.Pattern
	case RuleTypeWildcard:
		return matchWildcard(rule.Pattern, command)
	}
	return false
}

// matchWildcard matches a pattern with wildcards (* and ?)
func matchWildcard(pattern, text string) bool {
	// Simple wildcard matching
	pattern = strings.ReplaceAll(pattern, "*", ".*")
	pattern = strings.ReplaceAll(pattern, "?", ".")
	// This is a simplified implementation
	return strings.Contains(text, strings.Trim(pattern, ".*"))
}

// extractCommandBase extracts the base command (first word)
func extractCommandBase(command string) string {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return ""
	}
	// Skip env var assignments
	for len(parts) > 0 && strings.Contains(parts[0], "=") {
		parts = parts[1:]
	}
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

// isSafeCommand checks if a command is considered safe
func isSafeCommand(command string) bool {
	base := extractCommandBase(command)

	// Read-only commands are safe
	safeCommands := map[string]bool{
		"ls": true, "cat": true, "head": true, "tail": true, "pwd": true,
		"whoami": true, "date": true, "echo": true, "printf": true,
		"grep": true, "rg": true, "find": true, "stat": true,
		"wc": true, "sort": true, "uniq": true, "cut": true,
		"which": true, "whereis": true, "type": true,
	}

	if safeCommands[base] {
		return true
	}

	// Commands with safe arguments
	if base == "git" {
		safeGitSubcommands := map[string]bool{
			"status": true, "log": true, "show": true, "diff": true,
			"branch": true, "tag": true, "remote": true, "stash": true,
			"describe": true, "rev-parse": true, "ls-files": true,
			"ls-tree": true, "show-ref": true,
		}
		parts := strings.Fields(command)
		if len(parts) >= 2 {
			return safeGitSubcommands[parts[1]]
		}
	}

	return false
}

// ParsePermissionRule parses a permission rule string
func ParsePermissionRule(pattern string) PermissionRule {
	rule := PermissionRule{
		Pattern: pattern,
		Type:    RuleTypeWildcard,
	}

	// Check for exact match
	if !strings.Contains(pattern, "*") && !strings.Contains(pattern, "?") {
		rule.Type = RuleTypeExact
		return rule
	}

	// Check for prefix match (ends with :*)
	if strings.HasSuffix(pattern, ":*") {
		rule.Pattern = strings.TrimSuffix(pattern, ":*")
		rule.Type = RuleTypePrefix
		return rule
	}

	// Wildcard match
	rule.Type = RuleTypeWildcard
	return rule
}

// RuleFromPattern creates a PermissionRule from a pattern string with behavior
func RuleFromPattern(pattern string, behavior PermissionBehavior) PermissionRule {
	rule := ParsePermissionRule(pattern)
	rule.Behavior = behavior
	rule.Source = "userSettings"
	return rule
}

// Global permission checker instance
var globalPermissionChecker = CreatePermissionChecker(PermissionModeAsk)

// GetPermissionChecker returns the global permission checker
func GetPermissionChecker() *PermissionChecker {
	return globalPermissionChecker
}

// SetGlobalPermissionMode sets the global permission mode
func SetGlobalPermissionMode(mode PermissionMode) {
	globalPermissionChecker.SetMode(mode)
}

// LoadPersistedPermissionRules loads permission rules from settings.local.json
// This should be called at startup to restore previously saved rules
func LoadPersistedPermissionRules() {
	sm := settings.GetSettingsManager()

	// Load allow rules from all settings sources
	allowRules := sm.GetAllPermissionRules("allow")
	for _, rule := range allowRules {
		parsedRule := parsePermissionRuleString(rule, BehaviorAllow)
		if parsedRule != nil {
			globalPermissionChecker.AddRule(*parsedRule)
		}
	}

	// Load deny rules
	denyRules := sm.GetAllPermissionRules("deny")
	for _, rule := range denyRules {
		parsedRule := parsePermissionRuleString(rule, BehaviorDeny)
		if parsedRule != nil {
			globalPermissionChecker.AddRule(*parsedRule)
		}
	}

	// Load ask rules
	askRules := sm.GetAllPermissionRules("ask")
	for _, rule := range askRules {
		parsedRule := parsePermissionRuleString(rule, BehaviorAsk)
		if parsedRule != nil {
			globalPermissionChecker.AddRule(*parsedRule)
		}
	}

	// Set default mode from settings if specified
	merged := sm.GetMergedSettings()
	if merged != nil && merged.Permissions != nil && merged.Permissions.DefaultMode != nil {
		mode := stringToPermissionMode(*merged.Permissions.DefaultMode)
		if mode != "" {
			globalPermissionChecker.SetMode(mode)
		}
	}
}

// parsePermissionRuleString parses a permission rule string like "Bash(git status)"
// Returns nil if the rule is not for Bash or cannot be parsed
func parsePermissionRuleString(rule string, behavior PermissionBehavior) *PermissionRule {
	// Check if it's a Bash rule
	if !strings.HasPrefix(rule, "Bash") {
		return nil // Not a bash rule, skip
	}

	// Extract command pattern from Bash(command) format
	var pattern string
	if strings.HasPrefix(rule, "Bash(") && strings.HasSuffix(rule, ")") {
		// Extract content between parentheses
		content := rule[5 : len(rule)-1]
		pattern = content
	} else if rule == "Bash" {
		// "Bash" without parentheses means allow all bash commands
		pattern = "*"
	} else {
		return nil
	}

	// Create the permission rule
	parsedRule := ParsePermissionRule(pattern)
	parsedRule.Behavior = behavior
	parsedRule.Source = "localSettings"

	return &parsedRule
}

// stringToPermissionMode converts a mode string to PermissionMode
func stringToPermissionMode(mode string) PermissionMode {
	switch strings.ToLower(mode) {
	case "acceptedits":
		return PermissionModeAcceptEdits
	case "limittools":
		return PermissionModeLimitTools
	case "bypasspermissions":
		return PermissionModeBypassPermissions
	case "ask", "default":
		return PermissionModeAsk
	default:
		return ""
	}
}

// PermissionPromptOverride carries extra context for interactive approval UI.
// It is used by security validators that require explicit user approval even
// when a command might otherwise be auto-allowed by generic permission mode.
type PermissionPromptOverride struct {
	Reason      string
	Suggestions []string
}

// CheckGlobalPermission checks permissions using the global checker
func CheckGlobalPermission(command, description string) PermissionResult {
	return CheckGlobalPermissionWithPromptOverride(command, description, nil)
}

// CheckGlobalPermissionWithPromptOverride checks permissions using the global
// checker and optionally forces an interactive prompt with a custom reason.
// When user chooses "always allow", the rule is persisted to settings.local.json.
func CheckGlobalPermissionWithPromptOverride(command, description string, override *PermissionPromptOverride) PermissionResult {
	result := globalPermissionChecker.Check(command, description)
	if !shouldPromptForInteractivePermissionWithOverride(result, override) {
		return result
	}

	reason := strings.TrimSpace(result.Reason)
	suggestions := append([]string{}, result.Suggestions...)
	if override != nil {
		overrideReason := strings.TrimSpace(override.Reason)
		if overrideReason != "" {
			reason = overrideReason
		}
		if len(override.Suggestions) > 0 {
			suggestions = append(append([]string{}, override.Suggestions...), suggestions...)
		}
	}

	decision, err := interaction.RequestPermissionApproval("Bash", command, reason, suggestions)
	if err != nil {
		// If a security override required an explicit approval prompt and no
		// interactive handler is available, fail closed instead of silently
		// allowing execution in permissive modes.
		if override != nil && strings.TrimSpace(override.Reason) != "" {
			return PermissionResult{
				Allowed: false,
				Reason:  strings.TrimSpace(override.Reason),
			}
		}
		return result
	}

	switch decision {
	case interaction.PermissionDecisionAllowOnce:
		return PermissionResult{
			Allowed: true,
			Reason:  "approved by user (once)",
		}
	case interaction.PermissionDecisionAlwaysAllow:
		// Add rule to in-memory checker for immediate effect
		globalPermissionChecker.AddRule(PermissionRule{
			Pattern:  command,
			Type:     RuleTypeWildcard,
			Behavior: BehaviorAllow,
			Source:   "localSettings",
		})

		// Persist the rule to settings.local.json (matches TS PermissionUpdate.ts)
		// Format: "Bash(command:*)" or just the command pattern
		ruleStr := formatPermissionRule(command)
		sm := settings.GetSettingsManager()
		if err := sm.AddPermissionRule(ruleStr, "allow", settings.SourceLocalSettings); err != nil {
			// Log error but don't fail - rule is still in memory for this session
			fmt.Printf("Warning: failed to persist permission rule: %v\n", err)
		}

		return PermissionResult{
			Allowed: true,
			Reason:  "approved by user (always allow - persisted to settings.local.json)",
		}
	default:
		return PermissionResult{
			Allowed: false,
			Reason:  "permission denied by user",
		}
	}
}

// formatPermissionRule formats a command into a permission rule string
// Matches TS permission rule format: "Bash(command:pattern)" or "Bash"
func formatPermissionRule(command string) string {
	// For specific commands, use Bash(command:pattern) format
	if command != "" && command != "*" {
		return fmt.Sprintf("Bash(%s)", command)
	}
	// For general Bash permission, just use "Bash"
	return "Bash"
}

func shouldPromptForInteractivePermissionWithOverride(result PermissionResult, override *PermissionPromptOverride) bool {
	if override != nil && strings.TrimSpace(override.Reason) != "" {
		if result.Rule != nil && result.Rule.Behavior == BehaviorDeny {
			return false
		}
		reason := strings.ToLower(strings.TrimSpace(result.Reason))
		if strings.Contains(reason, "denied by rule") || strings.Contains(reason, "limittools mode") {
			return false
		}
		return true
	}
	return shouldPromptForInteractivePermission(result)
}

func shouldPromptForInteractivePermission(result PermissionResult) bool {
	if result.Allowed {
		return false
	}
	if result.Rule != nil && result.Rule.Behavior == BehaviorDeny {
		return false
	}

	reason := strings.ToLower(strings.TrimSpace(result.Reason))
	if reason == "" {
		return true
	}
	if strings.Contains(reason, "denied by rule") || strings.Contains(reason, "limittools mode") {
		return false
	}
	return strings.Contains(reason, "permission") ||
		strings.Contains(reason, "requires") ||
		strings.Contains(reason, "command may modify")
}
