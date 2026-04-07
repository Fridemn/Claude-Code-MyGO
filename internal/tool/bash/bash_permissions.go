package bash


import (
	"fmt"
	"strings"
	"sync"
)

// PermissionMode represents the permission mode for command execution
type PermissionMode string

const (
	PermissionModeAcceptEdits     PermissionMode = "acceptEdits"     // Accept all edits, prompt for dangerous
	PermissionModeLimitTools      PermissionMode = "limitTools"       // Limit to safe tools only
	PermissionModeBypassPermissions PermissionMode = "bypassPermissions" // Skip all permission checks
	PermissionModeAsk            PermissionMode = "ask"            // Ask for permission on each command
)

// PermissionRule represents a permission rule for bash commands
type PermissionRule struct {
	Pattern  string         // Command pattern (e.g., "git *", "npm run:*")
	Type     PermissionRuleType
	Behavior PermissionBehavior
	Source   string // "userSettings", "env", etc.
}

// PermissionRuleType determines how the pattern is matched
type PermissionRuleType int

const (
	RuleTypeExact   PermissionRuleType = iota // Exact match
	RuleTypePrefix                           // Prefix match (e.g., "npm run:*")
	RuleTypeWildcard                         // Wildcard match
)

// PermissionBehavior determines what happens when a rule matches
type PermissionBehavior int

const (
	BehaviorAllow PermissionBehavior = iota // Allow without prompting
	BehaviorAsk                              // Ask user before executing
	BehaviorDeny                             // Deny execution
)

// PermissionResult represents the result of a permission check
type PermissionResult struct {
	Allowed  bool
	Reason   string
	Rule     *PermissionRule
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
	mu         sync.RWMutex
	mode       PermissionMode
	rules      []PermissionRule
	pending    map[string]*PermissionRequest
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
			Allowed:  true,
			Reason:   "bypassPermissions mode",
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
			Allowed:  true,
			Reason:   "limitTools mode: safe command",
		}
	}

	// Check against rules
	for _, rule := range p.rules {
		if matchRule(rule, command) {
			switch rule.Behavior {
			case BehaviorAllow:
				return PermissionResult{
					Allowed:  true,
					Reason:   fmt.Sprintf("matched rule: %s", rule.Pattern),
					Rule:     &rule,
				}
			case BehaviorDeny:
				return PermissionResult{
					Allowed:  false,
					Reason:   fmt.Sprintf("denied by rule: %s", rule.Pattern),
					Rule:     &rule,
				}
			case BehaviorAsk:
				return PermissionResult{
					Allowed:  false,
					Reason:   fmt.Sprintf("requires permission: %s", rule.Pattern),
					Rule:     &rule,
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
				Allowed:  true,
				Reason:   "safe command",
			}
		}
		return PermissionResult{
			Allowed:  false,
			Reason:   "command may modify files",
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
		Allowed:  true,
		Reason:   "default allow",
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

// CheckGlobalPermission checks permissions using the global checker
func CheckGlobalPermission(command, description string) PermissionResult {
	return globalPermissionChecker.Check(command, description)
}
