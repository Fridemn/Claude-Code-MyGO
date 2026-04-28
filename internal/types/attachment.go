package types

import (
	"encoding/json"
	"fmt"
	"time"
)

// Attachment types
const (
	AttachmentTypeImage               = "image"
	AttachmentTypeDocument            = "document"
	AttachmentTypeHookBlockingError   = "hook_blocking_error"
	AttachmentTypeHookCancelled       = "hook_cancelled"
	AttachmentTypeHookError           = "hook_error_during_execution"
	AttachmentTypeHookNonBlockingError = "hook_non_blocking_error"
	AttachmentTypeHookSuccess         = "hook_success"
	AttachmentTypeHookSystemMessage   = "hook_system_message"
	AttachmentTypeHookAdditionalContext = "hook_additional_context"
	AttachmentTypeHookStoppedContinuation = "hook_stopped_continuation"
)

// Attachment represents an attachment in a message.
type Attachment struct {
	Type string `json:"type"`

	// For image attachments
	Path     string `json:"path,omitempty"`
	MediaType string `json:"media_type,omitempty"`
	Data     string `json:"data,omitempty"` // base64 data

	// For document attachments
	FileName string `json:"file_name,omitempty"`
	Content  string `json:"content,omitempty"`

	// For hook attachments
	HookName   string `json:"hook_name,omitempty"`
	HookEvent  string `json:"hook_event,omitempty"`
	ToolUseID  string `json:"tool_use_id,omitempty"`
	HookOutput string `json:"hook_output,omitempty"`
	HookError  string `json:"hook_error,omitempty"`
}

// AttachmentMessage represents an attachment message.
type AttachmentMessage struct {
	UUID       string     `json:"uuid"`
	Type       string     `json:"type"` // always "attachment"
	Timestamp  time.Time  `json:"timestamp"`
	Attachment Attachment `json:"attachment"`
}

// NewImageAttachment creates an image attachment.
func NewImageAttachment(path, mediaType string) Attachment {
	return Attachment{
		Type:      AttachmentTypeImage,
		Path:      path,
		MediaType: mediaType,
	}
}

// NewImageAttachmentWithData creates an image attachment with base64 data.
func NewImageAttachmentWithData(mediaType, base64Data string) Attachment {
	return Attachment{
		Type:      AttachmentTypeImage,
		MediaType: mediaType,
		Data:      base64Data,
	}
}

// NewDocumentAttachment creates a document attachment.
func NewDocumentAttachment(fileName, content string) Attachment {
	return Attachment{
		Type:     AttachmentTypeDocument,
		FileName: fileName,
		Content:  content,
	}
}

// NewHookSuccessAttachment creates a hook success attachment.
func NewHookSuccessAttachment(hookName, hookEvent, toolUseID, output string) Attachment {
	return Attachment{
		Type:      AttachmentTypeHookSuccess,
		HookName:  hookName,
		HookEvent: hookEvent,
		ToolUseID: toolUseID,
		HookOutput: output,
	}
}

// NewHookErrorAttachment creates a hook error attachment.
func NewHookErrorAttachment(hookName, hookEvent, toolUseID, errMsg string, isBlocking bool) Attachment {
	attType := AttachmentTypeHookNonBlockingError
	if isBlocking {
		attType = AttachmentTypeHookBlockingError
	}
	return Attachment{
		Type:      attType,
		HookName:  hookName,
		HookEvent: hookEvent,
		ToolUseID: toolUseID,
		HookError: errMsg,
	}
}

// NewHookCancelledAttachment creates a hook cancelled attachment.
func NewHookCancelledAttachment(hookName, hookEvent, toolUseID string) Attachment {
	return Attachment{
		Type:      AttachmentTypeHookCancelled,
		HookName:  hookName,
		HookEvent: hookEvent,
		ToolUseID: toolUseID,
	}
}

// HookResultToAttachment converts a HookResult to an Attachment.
func HookResultToAttachment(result HookResult) Attachment {
	if result.Error != "" {
		return NewHookErrorAttachment(result.HookName, result.HookEvent, result.ToolUseID, result.Error, result.IsBlocking)
	}
	if result.Decision == "block" {
		return NewHookErrorAttachment(result.HookName, result.HookEvent, result.ToolUseID, result.Error, true)
	}
	return NewHookSuccessAttachment(result.HookName, result.HookEvent, result.ToolUseID, result.Output)
}

// HookResultsToAttachmentMessage creates an AttachmentMessage from hook results.
func HookResultsToAttachmentMessage(hookEvent string, results []HookResult) *AttachmentMessage {
	if len(results) == 0 {
		return nil
	}

	// Create a summary attachment
	attachments := make([]Attachment, 0, len(results))
	for _, result := range results {
		attachments = append(attachments, HookResultToAttachment(result))
	}

	// Create content from all results
	var content string
	for i, result := range results {
		if i > 0 {
			content += "\n---\n"
		}
		content += fmt.Sprintf("[%s] %s", result.HookName, result.HookEvent)
		if result.Error != "" {
			content += fmt.Sprintf("\nError: %s", result.Error)
		} else if result.Output != "" {
			content += fmt.Sprintf("\n%s", result.Output)
		}
	}

	// Return first attachment as the message
	if len(attachments) > 0 {
		msg := attachments[0].ToMessage()
		msg.Attachment.Content = content
		return &msg
	}

	return nil
}

// IsHookAttachment returns true if this is a hook attachment.
func (a *Attachment) IsHookAttachment() bool {
	switch a.Type {
	case AttachmentTypeHookBlockingError,
		AttachmentTypeHookCancelled,
		AttachmentTypeHookError,
		AttachmentTypeHookNonBlockingError,
		AttachmentTypeHookSuccess,
		AttachmentTypeHookSystemMessage,
		AttachmentTypeHookAdditionalContext,
		AttachmentTypeHookStoppedContinuation:
		return true
	default:
		return false
	}
}

// IsPreToolUseHook returns true if this is a pre-tool-use hook attachment.
func (a *Attachment) IsPreToolUseHook() bool {
	return a.IsHookAttachment() && a.HookEvent == "PreToolUse"
}

// IsPostToolUseHook returns true if this is a post-tool-use hook attachment.
func (a *Attachment) IsPostToolUseHook() bool {
	return a.IsHookAttachment() && a.HookEvent == "PostToolUse"
}

// ToMessage creates an AttachmentMessage from the attachment.
func (a *Attachment) ToMessage() AttachmentMessage {
	return AttachmentMessage{
		UUID:       GenerateUUID(),
		Type:       MessageTypeAttachment,
		Timestamp:  time.Now(),
		Attachment: *a,
	}
}

// SystemMessage represents a system message.
type SystemMessage struct {
	UUID      string    `json:"uuid"`
	Type      string    `json:"type"` // always "system"
	Subtype   string    `json:"subtype"`
	Content   string    `json:"content"`
	Level     string    `json:"level,omitempty"` // "info", "warning", "error"
	Timestamp time.Time `json:"timestamp"`
}

// System message subtypes
const (
	SystemSubtypeAPIError         = "api_error"
	SystemSubtypeLocalCommand     = "local_command"
	SystemSubtypePermissionRetry  = "permission_retry"
	SystemSubtypeCompactBoundary  = "compact_boundary"
	SystemSubtypeMicrocompactBoundary = "microcompact_boundary"
	SystemSubtypeAwaySummary      = "away_summary"
	SystemSubtypeStopHookSummary  = "stop_hook_summary"
	SystemSubtypeBridgeStatus     = "bridge_status"
	SystemSubtypeAgentsKilled     = "agents_killed"
	SystemSubtypeTurnDuration     = "turn_duration"
	SystemSubtypeMemorySaved      = "memory_saved"
	SystemSubtypeInformational    = "informational"
	SystemSubtypeScheduledTaskFire = "scheduled_task_fire"
)

// NewSystemMessage creates a system message.
func NewSystemMessage(subtype, content string) SystemMessage {
	return SystemMessage{
		UUID:      GenerateUUID(),
		Type:      MessageTypeSystem,
		Subtype:   subtype,
		Content:   content,
		Timestamp: time.Now(),
	}
}

// NewSystemMessageWithLevel creates a system message with a level.
func NewSystemMessageWithLevel(subtype, content, level string) SystemMessage {
	return SystemMessage{
		UUID:      GenerateUUID(),
		Type:      MessageTypeSystem,
		Subtype:   subtype,
		Content:   content,
		Level:     level,
		Timestamp: time.Now(),
	}
}

// ToMessage converts SystemMessage to Message.
func (sm *SystemMessage) ToMessage() Message {
	return Message{
		UUID:      sm.UUID,
		Type:      MessageTypeSystem,
		Role:      MessageTypeSystem,
		Content:   sm.Content,
		Timestamp: sm.Timestamp,
	}
}

// ProgressMessageData represents data in a progress message.
type ProgressMessageData struct {
	Type        string `json:"type"`
	ToolUseID   string `json:"tool_use_id,omitempty"`
	HookEvent   string `json:"hook_event,omitempty"`
	HookName    string `json:"hook_name,omitempty"`
	Message     string `json:"message,omitempty"`
	Percentage  int    `json:"percentage,omitempty"`
}

// ProgressMessage represents a progress update message.
type ProgressMessage struct {
	UUID             string              `json:"uuid"`
	Type             string              `json:"type"` // always "progress"
	ToolUseID        string              `json:"tool_use_id"`
	ParentToolUseID  string              `json:"parent_tool_use_id,omitempty"`
	Data             ProgressMessageData `json:"data"`
	Timestamp        time.Time           `json:"timestamp"`
}

// NewProgressMessage creates a progress message.
func NewProgressMessage(toolUseID, parentToolUseID string, data ProgressMessageData) ProgressMessage {
	return ProgressMessage{
		UUID:            GenerateUUID(),
		Type:            MessageTypeProgress,
		ToolUseID:       toolUseID,
		ParentToolUseID: parentToolUseID,
		Data:            data,
		Timestamp:       time.Now(),
	}
}

// ToMessage converts ProgressMessage to Message.
func (pm *ProgressMessage) ToMessage() Message {
	dataJSON, _ := json.Marshal(pm.Data)
	return Message{
		UUID:        pm.UUID,
		Type:        MessageTypeProgress,
		ToolCallID:  pm.ToolUseID,
		Timestamp:   pm.Timestamp,
		ToolUseResult: json.RawMessage(dataJSON),
	}
}

// ToolUseSummaryMessage represents a summary of tool usage.
type ToolUseSummaryMessage struct {
	UUID      string    `json:"uuid"`
	Type      string    `json:"type"` // "tool_use_summary"
	Summary   string    `json:"summary"`
	ToolNames []string  `json:"tool_names"`
	Timestamp time.Time `json:"timestamp"`
}

// NewToolUseSummaryMessage creates a tool use summary message.
func NewToolUseSummaryMessage(summary string, toolNames []string) ToolUseSummaryMessage {
	return ToolUseSummaryMessage{
		UUID:      GenerateUUID(),
		Type:      "tool_use_summary",
		Summary:   summary,
		ToolNames: toolNames,
		Timestamp: time.Now(),
	}
}