package types

// ============================================================================
// Permission Modes
// ============================================================================

// PermissionMode represents the current permission mode.
type PermissionMode string

const (
	PermissionModeDefault         PermissionMode = "default"
	PermissionModeAuto            PermissionMode = "auto"
	PermissionModeDontAsk         PermissionMode = "dontAsk"
	PermissionModeBypass          PermissionMode = "bypassPermissions"
	PermissionModeAcceptEdits     PermissionMode = "acceptEdits"
	PermissionModePlan            PermissionMode = "plan"
	PermissionModeBubble          PermissionMode = "bubble"
)

// ExternalPermissionModes are modes that users can set directly.
var ExternalPermissionModes = []PermissionMode{
	PermissionModeDefault,
	PermissionModeDontAsk,
	PermissionModeBypass,
	PermissionModeAcceptEdits,
	PermissionModePlan,
}

// InternalPermissionModes includes all modes (external + internal-only).
var InternalPermissionModes = []PermissionMode{
	PermissionModeDefault,
	PermissionModeAuto,
	PermissionModeDontAsk,
	PermissionModeBypass,
	PermissionModeAcceptEdits,
	PermissionModePlan,
	PermissionModeBubble,
}

// AllPermissionModes returns all permission modes.
func AllPermissionModes() []PermissionMode {
	return InternalPermissionModes
}

// IsValidPermissionMode checks if a string is a valid permission mode.
func IsValidPermissionMode(s string) bool {
	for _, m := range AllPermissionModes() {
		if string(m) == s {
			return true
		}
	}
	return false
}

// IsExternalPermissionMode checks if a mode is user-addressable.
func IsExternalPermissionMode(mode PermissionMode) bool {
	for _, m := range ExternalPermissionModes {
		if m == mode {
			return true
		}
	}
	return false
}

// ============================================================================
// Permission Behaviors
// ============================================================================

// PermissionBehavior represents the action to take for a permission check.
type PermissionBehavior string

const (
	PermissionBehaviorAllow PermissionBehavior = "allow"
	PermissionBehaviorDeny  PermissionBehavior = "deny"
	PermissionBehaviorAsk   PermissionBehavior = "ask"
)

// ============================================================================
// Permission Rules
// ============================================================================

// PermissionRuleSource indicates where a permission rule originated.
type PermissionRuleSource string

const (
	PermissionSourceUserSettings    PermissionRuleSource = "userSettings"
	PermissionSourceProjectSettings PermissionRuleSource = "projectSettings"
	PermissionSourceLocalSettings   PermissionRuleSource = "localSettings"
	PermissionSourceFlagSettings    PermissionRuleSource = "flagSettings"
	PermissionSourcePolicySettings  PermissionRuleSource = "policySettings"
	PermissionSourceCLIArg          PermissionRuleSource = "cliArg"
	PermissionSourceCommand         PermissionRuleSource = "command"
	PermissionSourceSession         PermissionRuleSource = "session"
)

// PermissionRuleValue specifies which tool and optional content pattern.
type PermissionRuleValue struct {
	ToolName   string `json:"toolName"`
	RuleContent string `json:"ruleContent,omitempty"`
}

// PermissionRule represents a permission rule with its source and behavior.
type PermissionRule struct {
	Source       PermissionRuleSource `json:"source"`
	RuleBehavior PermissionBehavior  `json:"ruleBehavior"`
	RuleValue    PermissionRuleValue  `json:"ruleValue"`
}

// ============================================================================
// Permission Updates
// ============================================================================

// PermissionUpdateDestination specifies where to persist permission changes.
type PermissionUpdateDestination string

const (
	DestUserSettings    PermissionUpdateDestination = "userSettings"
	DestProjectSettings PermissionUpdateDestination = "projectSettings"
	DestLocalSettings   PermissionUpdateDestination = "localSettings"
	DestSession         PermissionUpdateDestination = "session"
	DestCLIArg          PermissionUpdateDestination = "cliArg"
)

// PermissionUpdate represents a permission configuration change.
type PermissionUpdate struct {
	Type        string                    `json:"type"` // addRules, replaceRules, removeRules, setMode, addDirectories, removeDirectories
	Destination PermissionUpdateDestination `json:"destination"`
	Rules       []PermissionRuleValue     `json:"rules,omitempty"`
	Behavior    PermissionBehavior        `json:"behavior,omitempty"`
	Mode        PermissionMode            `json:"mode,omitempty"`
	Directories []string                  `json:"directories,omitempty"`
}

// ============================================================================
// Permission Decisions & Results
// ============================================================================

// PermissionDecision represents the outcome of a permission check.
type PermissionDecision string

const (
	DecisionAllow PermissionDecision = "allow"
	DecisionDeny  PermissionDecision = "deny"
	DecisionAsk   PermissionDecision = "ask"
)

// PermissionDecisionReason explains why a permission decision was made.
type PermissionDecisionReason struct {
	Type    string            `json:"type"` // rule, mode, hook, classifier, asyncAgent, safetyCheck, other, etc.
	Rule    *PermissionRule   `json:"rule,omitempty"`
	Mode    PermissionMode    `json:"mode,omitempty"`
	HookName string           `json:"hookName,omitempty"`
	Reason  string            `json:"reason,omitempty"`
}

// PermissionResult represents the result of a permission check.
type PermissionResult struct {
	Behavior       PermissionBehavior       `json:"behavior"`
	Message        string                   `json:"message,omitempty"`
	UpdatedInput   map[string]interface{}   `json:"updatedInput,omitempty"`
	UserModified   bool                     `json:"userModified,omitempty"`
	DecisionReason *PermissionDecisionReason `json:"decisionReason,omitempty"`
	Suggestions    []PermissionUpdate       `json:"suggestions,omitempty"`
	BlockedPath    string                   `json:"blockedPath,omitempty"`
	IsBuiltin      bool                     `json:"isBuiltin,omitempty"`
	Rule           *PermissionRule          `json:"rule,omitempty"`
	Metadata       *PermissionMetadata      `json:"metadata,omitempty"`
}

// PermissionMetadata contains additional context for permission decisions.
type PermissionMetadata struct {
	CommandName        string `json:"commandName,omitempty"`
	CommandDescription string `json:"commandDescription,omitempty"`
}

// ============================================================================
// Permission Configuration
// ============================================================================

// ToolPermissionRulesBySource maps rule sources to rule strings.
type ToolPermissionRulesBySource map[PermissionRuleSource][]string

// ToolPermissionContext contains context needed for permission checking.
type ToolPermissionContext struct {
	Mode                         PermissionMode              `json:"mode"`
	AlwaysAllowRules             ToolPermissionRulesBySource `json:"alwaysAllowRules,omitempty"`
	AlwaysDenyRules              ToolPermissionRulesBySource `json:"alwaysDenyRules,omitempty"`
	AlwaysAskRules               ToolPermissionRulesBySource `json:"alwaysAskRules,omitempty"`
	AdditionalWorkingDirectories map[string]AdditionalWorkingDirectory `json:"additionalWorkingDirectories,omitempty"`
	IsBypassPermissionsModeAvailable bool                    `json:"isBypassPermissionsModeAvailable,omitempty"`
	ShouldAvoidPermissionPrompts    bool                    `json:"shouldAvoidPermissionPrompts,omitempty"`
	PrePlanMode                    PermissionMode           `json:"prePlanMode,omitempty"`
}

// PermissionConfig holds the permission configuration.
type PermissionConfig struct {
	Mode           PermissionMode          `json:"mode"`
	Rules          []PermissionRule        `json:"rules"`
	AllowList      []PermissionRuleValue   `json:"allowList,omitempty"`
	DenyList       []PermissionRuleValue   `json:"denyList,omitempty"`
	AdditionalDirs []AdditionalWorkingDirectory `json:"additionalDirs,omitempty"`
}

// AdditionalWorkingDirectory represents an additional directory in permission scope.
type AdditionalWorkingDirectory struct {
	Path   string               `json:"path"`
	Source PermissionRuleSource `json:"source"`
}

// ============================================================================
// Permission Check Context
// ============================================================================

// PermissionCheckContext contains information needed for permission checks.
type PermissionCheckContext struct {
	SessionID        string          `json:"sessionId"`
	CWD              string          `json:"cwd"`
	PermissionMode   PermissionMode  `json:"permissionMode"`
	ToolName         string          `json:"toolName"`
	ToolInput        interface{}     `json:"toolInput"`
	ToolUseID        string          `json:"toolUseID"`
	IsSubagent       bool            `json:"isSubagent,omitempty"`
	AgentID          string          `json:"agentId,omitempty"`
}

// ============================================================================
// Helper Functions
// ============================================================================

// NewAllowResult creates a permission result that allows the action.
func NewAllowResult(reason string) PermissionResult {
	return PermissionResult{
		Behavior: PermissionBehaviorAllow,
		Message:  reason,
	}
}

// NewDenyResult creates a permission result that denies the action.
func NewDenyResult(reason string) PermissionResult {
	return PermissionResult{
		Behavior: PermissionBehaviorDeny,
		Message:  reason,
	}
}

// NewAskResult creates a permission result that asks for confirmation.
func NewAskResult(reason string) PermissionResult {
	return PermissionResult{
		Behavior: PermissionBehaviorAsk,
		Message:  reason,
	}
}

// IsAllowed returns true if the permission result allows the action.
func (r PermissionResult) IsAllowed() bool {
	return r.Behavior == PermissionBehaviorAllow
}

// IsDenied returns true if the permission result denies the action.
func (r PermissionResult) IsDenied() bool {
	return r.Behavior == PermissionBehaviorDeny
}

// RequiresConfirmation returns true if the permission result requires user confirmation.
func (r PermissionResult) RequiresConfirmation() bool {
	return r.Behavior == PermissionBehaviorAsk
}

// MatchToolRule checks if a rule matches a tool name and optional content.
func MatchToolRule(rule PermissionRuleValue, toolName string, content string) bool {
	if rule.ToolName == "*" {
		return true
	}
	if rule.ToolName != toolName {
		return false
	}
	if rule.RuleContent == "" {
		return true
	}
	// Simple glob matching for rule content
	return MatchGlob(rule.RuleContent, content)
}

// MatchGlob performs simple glob pattern matching.
func MatchGlob(pattern, text string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	if text == "" {
		return false
	}
	// Simple implementation - can be enhanced for more complex patterns
	if pattern == text {
		return true
	}
	// Check for prefix/suffix patterns
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		return len(text) >= len(pattern)-1 && text[:len(pattern)-1] == pattern[:len(pattern)-1]
	}
	if len(pattern) > 0 && pattern[0] == '*' {
		return len(text) >= len(pattern)-1 && text[len(text)-(len(pattern)-1):] == pattern[1:]
	}
	return false
}

// FormatRuleValue formats a permission rule value as a string.
func FormatRuleValue(rule PermissionRuleValue) string {
	if rule.RuleContent != "" {
		return rule.ToolName + "(" + rule.RuleContent + ")"
	}
	return rule.ToolName
}

// ParseRuleValue parses a permission rule string like "Bash(git *)" into a PermissionRuleValue.
func ParseRuleValue(s string) PermissionRuleValue {
	// Find opening parenthesis
	openIdx := -1
	for i, c := range s {
		if c == '(' {
			openIdx = i
			break
		}
	}

	if openIdx == -1 {
		return PermissionRuleValue{ToolName: s}
	}

	// Find closing parenthesis
	closeIdx := -1
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ')' {
			closeIdx = i
			break
		}
	}

	if closeIdx <= openIdx {
		return PermissionRuleValue{ToolName: s[:openIdx]}
	}

	return PermissionRuleValue{
		ToolName:   s[:openIdx],
		RuleContent: s[openIdx+1 : closeIdx],
	}
}

// IsReadOnlyTool returns true if the tool is read-only.
func IsReadOnlyTool(toolName string) bool {
	switch toolName {
	case "Read", "Glob", "Grep", "WebFetch", "Agent", "TaskList", "TaskGet":
		return true
	default:
		return false
	}
}

// IsDangerousTool returns true if the tool could be dangerous.
func IsDangerousTool(toolName string) bool {
	switch toolName {
	case "Bash", "Write", "Edit", "Delete":
		return true
	default:
		return false
	}
}
