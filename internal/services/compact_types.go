package services

// Message types for compact service.
// Ported from src/types/message.ts (relevant portions)

// MessageType constants
const (
	MessageTypeUser      = "user"
	MessageTypeAssistant = "assistant"
	MessageTypeSystem    = "system"
	MessageTypeTool      = "tool"
)

// CompactMessage represents a message in the compact context.
// This is a simplified version focusing on compact-relevant fields.
type CompactMessage struct {
	UUID        string              `json:"uuid,omitempty"`
	Type        string              `json:"type,omitempty"`
	Role        string              `json:"role"`
	Content     string              `json:"content"`
	Images      []string            `json:"images,omitempty"`
	ToolCalls   []ToolCallContent   `json:"tool_calls,omitempty"`
	ToolResults []ToolResultContent `json:"tool_results,omitempty"`
	Timestamp   string              `json:"timestamp,omitempty"`
	IsMeta      bool                `json:"is_meta,omitempty"`

	// Compact-specific metadata
	IsCompactSummary          bool   `json:"is_compact_summary,omitempty"`
	IsVisibleInTranscriptOnly bool   `json:"is_visible_in_transcript_only,omitempty"`
	MessageID                 string `json:"message_id,omitempty"` // For assistant messages

	// For tool results
	ToolUseID string `json:"tool_use_id,omitempty"`
}

// ToolCallContent represents a tool call in a message
type ToolCallContent struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolResultContent represents the result of a tool call
type ToolResultContent struct {
	ToolUseID string   `json:"tool_use_id"`
	Content   string   `json:"content"`
	IsError   bool     `json:"is_error,omitempty"`
	Images    []string `json:"images,omitempty"`
	Documents []string `json:"documents,omitempty"`
}

// PartialCompactDirection indicates the direction for partial compaction
type PartialCompactDirection string

const (
	DirectionFrom  PartialCompactDirection = "from"
	DirectionUpTo  PartialCompactDirection = "up_to"
)

// CompactMetadata contains metadata for compact boundary messages
type CompactMetadata struct {
	Trigger                   string            `json:"trigger"`
	PreCompactTokenCount      int               `json:"pre_compact_token_count"`
	LastPreCompactUuid        string            `json:"last_pre_compact_uuid,omitempty"`
	UserFeedback              string            `json:"user_feedback,omitempty"`
	MessagesSummarized        int               `json:"messages_summarized,omitempty"`
	PreservedSegment          *PreservedSegment `json:"preserved_segment,omitempty"`
	PreCompactDiscoveredTools []string          `json:"pre_compact_discovered_tools,omitempty"`
}

// PreservedSegment tracks preserved messages during partial compact
type PreservedSegment struct {
	HeadUUID   string `json:"head_uuid"`
	AnchorUUID string `json:"anchor_uuid"`
	TailUUID   string `json:"tail_uuid"`
}

// RecompactionInfo tracks recompaction state for telemetry
// Ported from src/services/compact/compact.ts:RecompactionInfo
type RecompactionInfo struct {
	IsRecompactionInChain     bool
	TurnsSincePreviousCompact int
	PreviousCompactTurnID     string
	AutoCompactThreshold      int
	QuerySource               string
}
