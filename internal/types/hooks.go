package types

// HookEvent represents the type of hook event.
// Ported from src/entrypoints/sdk/coreSchemas.ts:HOOK_EVENTS
type HookEvent string

const (
	HookEventPreToolUse         HookEvent = "PreToolUse"
	HookEventPostToolUse        HookEvent = "PostToolUse"
	HookEventPostToolUseFailure HookEvent = "PostToolUseFailure"
	HookEventNotification       HookEvent = "Notification"
	HookEventUserPromptSubmit   HookEvent = "UserPromptSubmit"
	HookEventSessionStart       HookEvent = "SessionStart"
	HookEventSessionEnd         HookEvent = "SessionEnd"
	HookEventStop               HookEvent = "Stop"
	HookEventStopFailure        HookEvent = "StopFailure"
	HookEventSubagentStart      HookEvent = "SubagentStart"
	HookEventSubagentStop       HookEvent = "SubagentStop"
	HookEventPreCompact         HookEvent = "PreCompact"
	HookEventPostCompact        HookEvent = "PostCompact"
	HookEventPermissionRequest  HookEvent = "PermissionRequest"
	HookEventPermissionDenied   HookEvent = "PermissionDenied"
	HookEventSetup              HookEvent = "Setup"
	HookEventTeammateIdle       HookEvent = "TeammateIdle"
	HookEventTaskCreated        HookEvent = "TaskCreated"
	HookEventTaskCompleted      HookEvent = "TaskCompleted"
	HookEventElicitation        HookEvent = "Elicitation"
	HookEventElicitationResult  HookEvent = "ElicitationResult"
	HookEventConfigChange       HookEvent = "ConfigChange"
	HookEventWorktreeCreate     HookEvent = "WorktreeCreate"
	HookEventWorktreeRemove     HookEvent = "WorktreeRemove"
	HookEventInstructionsLoaded HookEvent = "InstructionsLoaded"
	HookEventCwdChanged         HookEvent = "CwdChanged"
	HookEventFileChanged        HookEvent = "FileChanged"
)

// AllHookEvents returns all hook events.
func AllHookEvents() []HookEvent {
	return []HookEvent{
		HookEventPreToolUse,
		HookEventPostToolUse,
		HookEventPostToolUseFailure,
		HookEventNotification,
		HookEventUserPromptSubmit,
		HookEventSessionStart,
		HookEventSessionEnd,
		HookEventStop,
		HookEventStopFailure,
		HookEventSubagentStart,
		HookEventSubagentStop,
		HookEventPreCompact,
		HookEventPostCompact,
		HookEventPermissionRequest,
		HookEventPermissionDenied,
		HookEventSetup,
		HookEventTeammateIdle,
		HookEventTaskCreated,
		HookEventTaskCompleted,
		HookEventElicitation,
		HookEventElicitationResult,
		HookEventConfigChange,
		HookEventWorktreeCreate,
		HookEventWorktreeRemove,
		HookEventInstructionsLoaded,
		HookEventCwdChanged,
		HookEventFileChanged,
	}
}

// IsHookEvent checks if a string is a valid hook event.
func IsHookEvent(s string) bool {
	for _, e := range AllHookEvents() {
		if string(e) == s {
			return true
		}
	}
	return false
}

// BaseHookInput contains common fields for all hook inputs.
type BaseHookInput struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	CWD            string `json:"cwd"`
	PermissionMode string `json:"permission_mode,omitempty"`
	AgentID        string `json:"agent_id,omitempty"`
	AgentType      string `json:"agent_type,omitempty"`
}

// PreToolUseHookInput is the input for PreToolUse hooks.
type PreToolUseHookInput struct {
	BaseHookInput
	HookEventName string      `json:"hook_event_name"`
	ToolName      string      `json:"tool_name"`
	ToolInput     interface{} `json:"tool_input"`
	ToolUseID     string      `json:"tool_use_id"`
}

// PostToolUseHookInput is the input for PostToolUse hooks.
type PostToolUseHookInput struct {
	BaseHookInput
	HookEventName string      `json:"hook_event_name"`
	ToolName      string      `json:"tool_name"`
	ToolInput     interface{} `json:"tool_input"`
	ToolResponse  interface{} `json:"tool_response"`
	ToolUseID     string      `json:"tool_use_id"`
}

// PostToolUseFailureHookInput is the input for PostToolUseFailure hooks.
type PostToolUseFailureHookInput struct {
	BaseHookInput
	HookEventName string      `json:"hook_event_name"`
	ToolName      string      `json:"tool_name"`
	ToolInput     interface{} `json:"tool_input"`
	Error         string      `json:"error"`
	ToolUseID     string      `json:"tool_use_id"`
}

// NotificationHookInput is the input for Notification hooks.
type NotificationHookInput struct {
	BaseHookInput
	HookEventName    string `json:"hook_event_name"`
	NotificationType string `json:"notification_type"`
	Title            string `json:"title,omitempty"`
	Message          string `json:"message,omitempty"`
}

// UserPromptSubmitHookInput is the input for UserPromptSubmit hooks.
type UserPromptSubmitHookInput struct {
	BaseHookInput
	HookEventName string `json:"hook_event_name"`
	Prompt        string `json:"prompt"`
}

// SessionStartHookInput is the input for SessionStart hooks.
type SessionStartHookInput struct {
	BaseHookInput
	HookEventName string `json:"hook_event_name"`
	Source        string `json:"source,omitempty"`
	Trigger       string `json:"trigger,omitempty"`
}

// SessionEndHookInput is the input for SessionEnd hooks.
type SessionEndHookInput struct {
	BaseHookInput
	HookEventName string `json:"hook_event_name"`
	Reason        string `json:"reason"`
}

// StopHookInput is the input for Stop hooks.
type StopHookInput struct {
	BaseHookInput
	HookEventName string `json:"hook_event_name"`
	Reason        string `json:"reason,omitempty"`
}

// StopFailureHookInput is the input for StopFailure hooks.
type StopFailureHookInput struct {
	BaseHookInput
	HookEventName string `json:"hook_event_name"`
	Error         string `json:"error"`
}

// SubagentStartHookInput is the input for SubagentStart hooks.
type SubagentStartHookInput struct {
	BaseHookInput
	HookEventName string `json:"hook_event_name"`
	AgentType     string `json:"agent_type"`
	TaskID        string `json:"task_id,omitempty"`
	Description   string `json:"description,omitempty"`
}

// SubagentStopHookInput is the input for SubagentStop hooks.
type SubagentStopHookInput struct {
	BaseHookInput
	HookEventName string `json:"hook_event_name"`
	AgentType     string `json:"agent_type"`
	TaskID        string `json:"task_id,omitempty"`
	Result        string `json:"result,omitempty"`
}

// PreCompactHookInput is the input for PreCompact hooks.
type PreCompactHookInput struct {
	BaseHookInput
	HookEventName     string `json:"hook_event_name"`
	Trigger           string `json:"trigger"`
	CustomInstructions string `json:"custom_instructions,omitempty"`
}

// PostCompactHookInput is the input for PostCompact hooks.
type PostCompactHookInput struct {
	BaseHookInput
	HookEventName     string `json:"hook_event_name"`
	Trigger           string `json:"trigger"`
	WasCompacted      bool   `json:"was_compacted"`
	TokensBefore      int    `json:"tokens_before,omitempty"`
	TokensAfter       int    `json:"tokens_after,omitempty"`
}

// PermissionRequestHookInput is the input for PermissionRequest hooks.
type PermissionRequestHookInput struct {
	BaseHookInput
	HookEventName string      `json:"hook_event_name"`
	ToolName      string      `json:"tool_name"`
	ToolInput     interface{} `json:"tool_input"`
	ToolUseID     string      `json:"tool_use_id"`
	PermissionKey string      `json:"permission_key,omitempty"`
}

// PermissionDeniedHookInput is the input for PermissionDenied hooks.
type PermissionDeniedHookInput struct {
	BaseHookInput
	HookEventName string      `json:"hook_event_name"`
	ToolName      string      `json:"tool_name"`
	ToolInput     interface{} `json:"tool_input"`
	ToolUseID     string      `json:"tool_use_id"`
	Reason        string      `json:"reason,omitempty"`
}

// SetupHookInput is the input for Setup hooks.
type SetupHookInput struct {
	BaseHookInput
	HookEventName string `json:"hook_event_name"`
	Trigger       string `json:"trigger"`
}

// TaskCreatedHookInput is the input for TaskCreated hooks.
type TaskCreatedHookInput struct {
	BaseHookInput
	HookEventName string `json:"hook_event_name"`
	TaskID        string `json:"task_id"`
	TaskType      string `json:"task_type,omitempty"`
}

// TaskCompletedHookInput is the input for TaskCompleted hooks.
type TaskCompletedHookInput struct {
	BaseHookInput
	HookEventName string `json:"hook_event_name"`
	TaskID        string `json:"task_id"`
	TaskType      string `json:"task_type,omitempty"`
	Status        string `json:"status"`
}

// ConfigChangeHookInput is the input for ConfigChange hooks.
type ConfigChangeHookInput struct {
	BaseHookInput
	HookEventName string `json:"hook_event_name"`
	Source        string `json:"source"`
	Changes       map[string]interface{} `json:"changes,omitempty"`
}

// CwdChangedHookInput is the input for CwdChanged hooks.
type CwdChangedHookInput struct {
	BaseHookInput
	HookEventName string `json:"hook_event_name"`
	OldCWD        string `json:"old_cwd"`
	NewCWD        string `json:"new_cwd"`
}

// FileChangedHookInput is the input for FileChanged hooks.
type FileChangedHookInput struct {
	BaseHookInput
	HookEventName string `json:"hook_event_name"`
	FilePath      string `json:"file_path"`
	ChangeType    string `json:"change_type,omitempty"` // "created", "modified", "deleted"
}

// InstructionsLoadedHookInput is the input for InstructionsLoaded hooks.
type InstructionsLoadedHookInput struct {
	BaseHookInput
	HookEventName string `json:"hook_event_name"`
	LoadReason    string `json:"load_reason"`
	Source        string `json:"source,omitempty"`
}

// HookInput is a union type for all hook inputs.
type HookInput interface {
	GetHookEventName() string
	GetSessionID() string
}

// Implement HookInput interface for all input types
func (h PreToolUseHookInput) GetHookEventName() string { return h.HookEventName }
func (h PreToolUseHookInput) GetSessionID() string     { return h.SessionID }

func (h PostToolUseHookInput) GetHookEventName() string { return h.HookEventName }
func (h PostToolUseHookInput) GetSessionID() string     { return h.SessionID }

func (h PostToolUseFailureHookInput) GetHookEventName() string { return h.HookEventName }
func (h PostToolUseFailureHookInput) GetSessionID() string     { return h.SessionID }

func (h NotificationHookInput) GetHookEventName() string { return h.HookEventName }
func (h NotificationHookInput) GetSessionID() string     { return h.SessionID }

func (h UserPromptSubmitHookInput) GetHookEventName() string { return h.HookEventName }
func (h UserPromptSubmitHookInput) GetSessionID() string     { return h.SessionID }

func (h SessionStartHookInput) GetHookEventName() string { return h.HookEventName }
func (h SessionStartHookInput) GetSessionID() string     { return h.SessionID }

func (h SessionEndHookInput) GetHookEventName() string { return h.HookEventName }
func (h SessionEndHookInput) GetSessionID() string     { return h.SessionID }

func (h StopHookInput) GetHookEventName() string { return h.HookEventName }
func (h StopHookInput) GetSessionID() string     { return h.SessionID }

func (h StopFailureHookInput) GetHookEventName() string { return h.HookEventName }
func (h StopFailureHookInput) GetSessionID() string     { return h.SessionID }

func (h SubagentStartHookInput) GetHookEventName() string { return h.HookEventName }
func (h SubagentStartHookInput) GetSessionID() string     { return h.SessionID }

func (h SubagentStopHookInput) GetHookEventName() string { return h.HookEventName }
func (h SubagentStopHookInput) GetSessionID() string     { return h.SessionID }

func (h PreCompactHookInput) GetHookEventName() string { return h.HookEventName }
func (h PreCompactHookInput) GetSessionID() string     { return h.SessionID }

func (h PostCompactHookInput) GetHookEventName() string { return h.HookEventName }
func (h PostCompactHookInput) GetSessionID() string     { return h.SessionID }

func (h PermissionRequestHookInput) GetHookEventName() string { return h.HookEventName }
func (h PermissionRequestHookInput) GetSessionID() string     { return h.SessionID }

func (h PermissionDeniedHookInput) GetHookEventName() string { return h.HookEventName }
func (h PermissionDeniedHookInput) GetSessionID() string     { return h.SessionID }

func (h SetupHookInput) GetHookEventName() string { return h.HookEventName }
func (h SetupHookInput) GetSessionID() string     { return h.SessionID }

func (h TaskCreatedHookInput) GetHookEventName() string { return h.HookEventName }
func (h TaskCreatedHookInput) GetSessionID() string     { return h.SessionID }

func (h TaskCompletedHookInput) GetHookEventName() string { return h.HookEventName }
func (h TaskCompletedHookInput) GetSessionID() string     { return h.SessionID }

func (h ConfigChangeHookInput) GetHookEventName() string { return h.HookEventName }
func (h ConfigChangeHookInput) GetSessionID() string     { return h.SessionID }

func (h CwdChangedHookInput) GetHookEventName() string { return h.HookEventName }
func (h CwdChangedHookInput) GetSessionID() string     { return h.SessionID }

func (h FileChangedHookInput) GetHookEventName() string { return h.HookEventName }
func (h FileChangedHookInput) GetSessionID() string     { return h.SessionID }

func (h InstructionsLoadedHookInput) GetHookEventName() string { return h.HookEventName }
func (h InstructionsLoadedHookInput) GetSessionID() string     { return h.SessionID }

// SyncHookOutput represents the synchronous output from a hook.
type SyncHookOutput struct {
	Continue         bool                   `json:"continue,omitempty"`
	SuppressOutput   bool                   `json:"suppressOutput,omitempty"`
	StopReason       string                 `json:"stopReason,omitempty"`
	Decision         string                 `json:"decision,omitempty"` // "approve" or "block"
	Reason           string                 `json:"reason,omitempty"`
	SystemMessage    string                 `json:"systemMessage,omitempty"`
	AdditionalContext string                `json:"additionalContext,omitempty"`
	UpdatedInput     map[string]interface{} `json:"updatedInput,omitempty"`
	PermissionDecision string               `json:"permissionDecision,omitempty"`
}

// AsyncHookOutput represents the asynchronous output from a hook.
type AsyncHookOutput struct {
	Async bool `json:"async"`
}

// PromptRequest represents a prompt elicitation request.
type PromptRequest struct {
	Prompt   string          `json:"prompt"`
	Message  string          `json:"message"`
	Options  []PromptOption  `json:"options"`
}

// PromptOption represents an option in a prompt request.
type PromptOption struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// PromptResponse represents a response to a prompt request.
type PromptResponse struct {
	PromptResponse string `json:"prompt_response"`
	Selected       string `json:"selected"`
}

// HookJSONOutput is the union of sync and async hook outputs.
type HookJSONOutput struct {
	Sync  *SyncHookOutput
	Async *AsyncHookOutput
}

// HookMatcher defines how to match hooks to events.
type HookMatcher struct {
	Matcher   string `json:"matcher,omitempty"`
	Hooks     []HookDefinition `json:"hooks"`
}

// HookDefinition defines a single hook.
type HookDefinition struct {
	Type        string `json:"type"` // "command" or "prompt"
	Command     string `json:"command,omitempty"`
	TimeoutMs   int    `json:"timeout_ms,omitempty"`
	Blocking    bool   `json:"blocking,omitempty"`
	Description string `json:"description,omitempty"`
}

// HookResult represents the result of a hook execution.
type HookResult struct {
	HookName    string `json:"hook_name"`
	HookEvent   string `json:"hook_event"`
	ToolUseID   string `json:"tool_use_id,omitempty"`
	Output      string `json:"output,omitempty"`
	Error       string `json:"error,omitempty"`
	IsBlocking  bool   `json:"is_blocking,omitempty"`
	Continue    bool   `json:"continue"`
	Decision    string `json:"decision,omitempty"`
	DurationMs  int    `json:"duration_ms,omitempty"`
}

// HookBlockingError represents an error that blocks tool execution.
type HookBlockingError struct {
	Message     string `json:"message"`
	StopReason  string `json:"stop_reason,omitempty"`
	HookName    string `json:"hook_name,omitempty"`
}

// AggregatedHookResult represents the aggregated result of multiple hooks.
type AggregatedHookResult struct {
	HookEventName string        `json:"hook_event_name"`
	Results       []HookResult  `json:"results"`
	Blocked       bool          `json:"blocked"`
	StopReason    string        `json:"stop_reason,omitempty"`
	SystemMessage string        `json:"system_message,omitempty"`
	Attachments   []Attachment  `json:"attachments,omitempty"`
}
