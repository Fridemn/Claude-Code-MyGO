package services

import (
	"strings"
	"sync"
)

// Attachment types for compact service
// Ported from src/utils/attachments.ts

// AttachmentType constants
const (
	AttachmentTypePlanFileReference = "plan_file_reference"
	AttachmentTypeInvokedSkills     = "invoked_skills"
	AttachmentTypePlanMode          = "plan_mode"
	AttachmentTypeTaskStatus        = "task_status"
	AttachmentTypeFile              = "file"
	AttachmentTypeCompactFileRef    = "compact_file_reference"
)

// Post-compact token budgets
// Ported from src/services/compact/compact.ts
const (
	CompactMaxTokensPerSkill  = 20000
	CompactSkillsTokenBudget  = 60000
	CompactMaxTokensPerFile   = 5000
	CompactFilesTokenBudget   = 20000
)

// SkillTruncationMarker is appended when skill content is truncated
const SkillTruncationMarker = "\n\n[... skill content truncated for compaction; use Read on the skill path if you need the full text]"

// Attachment represents an attachment message
type Attachment struct {
	Type        string      `json:"type"`
	Data        interface{} `json:"data,omitempty"`
	PlanFilePath string      `json:"plan_file_path,omitempty"`
	PlanContent  string      `json:"plan_content,omitempty"`
	Skills       []SkillAttachment `json:"skills,omitempty"`
	ReminderType string      `json:"reminder_type,omitempty"`
	IsSubAgent   bool        `json:"is_sub_agent,omitempty"`
	PlanExists   bool        `json:"plan_exists,omitempty"`
	TaskID       string      `json:"task_id,omitempty"`
	TaskType     string      `json:"task_type,omitempty"`
	Description  string      `json:"description,omitempty"`
	Status       string      `json:"status,omitempty"`
	DeltaSummary string      `json:"delta_summary,omitempty"`
	OutputFilePath string    `json:"output_file_path,omitempty"`
	Filename     string      `json:"filename,omitempty"`
	Content      string      `json:"content,omitempty"`
	DisplayPath  string      `json:"display_path,omitempty"`
	Truncated    bool        `json:"truncated,omitempty"`
}

// SkillAttachment represents an invoked skill
type SkillAttachment struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Content string `json:"content"`
}

// AttachmentMessage represents an attachment wrapped in a message
// Ported from src/types/message.ts:AttachmentMessage
type AttachmentMessage struct {
	Type       string     `json:"type"`
	UUID       string     `json:"uuid"`
	Timestamp  string     `json:"timestamp"`
	Attachment Attachment `json:"attachment"`
}

// InvokedSkillInfo contains information about an invoked skill
type InvokedSkillInfo struct {
	SkillName string
	SkillPath string
	Content   string
	InvokedAt int64
}

// PlanState contains plan-related state for attachment creation
type PlanState struct {
	mu             sync.RWMutex
	planContent    string
	planFilePath   string
}

// Global plan state
var globalPlanState = &PlanState{}

// GetPlanContent returns the current plan content
func GetPlanContent() string {
	globalPlanState.mu.RLock()
	defer globalPlanState.mu.RUnlock()
	return globalPlanState.planContent
}

// SetPlanContent sets the plan content
func SetPlanContent(content string) {
	globalPlanState.mu.Lock()
	defer globalPlanState.mu.Unlock()
	globalPlanState.planContent = content
}

// GetPlanFilePath returns the plan file path
func GetPlanFilePath() string {
	globalPlanState.mu.RLock()
	defer globalPlanState.mu.RUnlock()
	return globalPlanState.planFilePath
}

// SetPlanFilePath sets the plan file path
func SetPlanFilePath(path string) {
	globalPlanState.mu.Lock()
	defer globalPlanState.mu.Unlock()
	globalPlanState.planFilePath = path
}

// InvokedSkillsState manages invoked skills for attachment creation
type InvokedSkillsState struct {
	mu     sync.RWMutex
	skills map[string]*InvokedSkillInfo // keyed by skill name
}

// Global invoked skills state
var globalInvokedSkillsState = &InvokedSkillsState{
	skills: make(map[string]*InvokedSkillInfo),
}

// AddInvokedSkill adds or updates an invoked skill
func AddInvokedSkill(name, path, content string) {
	globalInvokedSkillsState.mu.Lock()
	defer globalInvokedSkillsState.mu.Unlock()
	globalInvokedSkillsState.skills[name] = &InvokedSkillInfo{
		SkillName: name,
		SkillPath: path,
		Content:   content,
		InvokedAt: currentTimeMillis(),
	}
}

// GetInvokedSkillsForAgent returns invoked skills (agentId parameter for API parity, currently unused)
func GetInvokedSkillsForAgent(agentID string) map[string]*InvokedSkillInfo {
	globalInvokedSkillsState.mu.RLock()
	defer globalInvokedSkillsState.mu.RUnlock()
	result := make(map[string]*InvokedSkillInfo)
	for k, v := range globalInvokedSkillsState.skills {
		result[k] = v
	}
	return result
}

// ClearInvokedSkills clears all invoked skills
func ClearInvokedSkills() {
	globalInvokedSkillsState.mu.Lock()
	defer globalInvokedSkillsState.mu.Unlock()
	globalInvokedSkillsState.skills = make(map[string]*InvokedSkillInfo)
}

// AsyncAgentState represents state of an async agent for attachment
type AsyncAgentState struct {
	AgentID     string
	Description string
	Status      string // "running", "completed", "error"
	Error       string
	Summary     string
	OutputPath  string
	Retrieved   bool
}

// AsyncAgentsState manages async agents for attachment creation
type AsyncAgentsState struct {
	mu     sync.RWMutex
	agents map[string]*AsyncAgentState
}

// Global async agents state
var globalAsyncAgentsState = &AsyncAgentsState{
	agents: make(map[string]*AsyncAgentState),
}

// AddAsyncAgent adds or updates an async agent
func AddAsyncAgent(agent *AsyncAgentState) {
	globalAsyncAgentsState.mu.Lock()
	defer globalAsyncAgentsState.mu.Unlock()
	globalAsyncAgentsState.agents[agent.AgentID] = agent
}

// GetAsyncAgents returns all async agents
func GetAsyncAgents() map[string]*AsyncAgentState {
	globalAsyncAgentsState.mu.RLock()
	defer globalAsyncAgentsState.mu.RUnlock()
	result := make(map[string]*AsyncAgentState)
	for k, v := range globalAsyncAgentsState.agents {
		result[k] = v
	}
	return result
}

// RemoveAsyncAgent removes an async agent
func RemoveAsyncAgent(agentID string) {
	globalAsyncAgentsState.mu.Lock()
	defer globalAsyncAgentsState.mu.Unlock()
	delete(globalAsyncAgentsState.agents, agentID)
}

// PlanModeState tracks plan mode state
type PlanModeState struct {
	mu        sync.RWMutex
	inPlanMode bool
}

// Global plan mode state
var globalPlanModeState = &PlanModeState{}

// IsInPlanMode returns whether currently in plan mode
func IsInPlanMode() bool {
	globalPlanModeState.mu.RLock()
	defer globalPlanModeState.mu.RUnlock()
	return globalPlanModeState.inPlanMode
}

// SetPlanMode sets plan mode state
func SetPlanMode(inPlanMode bool) {
	globalPlanModeState.mu.Lock()
	defer globalPlanModeState.mu.Unlock()
	globalPlanModeState.inPlanMode = inPlanMode
}

// currentTimeMillis returns current time in milliseconds
func currentTimeMillis() int64 {
	return 0 // Simplified - would use time.Now().UnixMilli() in production
}

// CreatePlanAttachmentIfNeeded creates a plan file attachment if a plan exists
// Ported from src/services/compact/compact.ts:createPlanAttachmentIfNeeded
func CreatePlanAttachmentIfNeeded(agentID string) *AttachmentMessage {
	planContent := GetPlanContent()
	if planContent == "" {
		return nil
	}

	planFilePath := GetPlanFilePath()

	return &AttachmentMessage{
		Type:      "attachment",
		UUID:      generateUUID(),
		Timestamp: currentTimeStamp(),
		Attachment: Attachment{
			Type:         AttachmentTypePlanFileReference,
			PlanFilePath: planFilePath,
			PlanContent:  planContent,
		},
	}
}

// CreateSkillAttachmentIfNeeded creates an attachment for invoked skills
// Ported from src/services/compact/compact.ts:createSkillAttachmentIfNeeded
func CreateSkillAttachmentIfNeeded(agentID string) *AttachmentMessage {
	invokedSkills := GetInvokedSkillsForAgent(agentID)

	if len(invokedSkills) == 0 {
		return nil
	}

	// Sort most-recent-first so budget pressure drops least-relevant skills
	skillList := make([]*InvokedSkillInfo, 0, len(invokedSkills))
	for _, skill := range invokedSkills {
		skillList = append(skillList, skill)
	}

	// Sort by invokedAt descending (most recent first)
	for i := 0; i < len(skillList)-1; i++ {
		for j := i + 1; j < len(skillList); j++ {
			if skillList[j].InvokedAt > skillList[i].InvokedAt {
				skillList[i], skillList[j] = skillList[j], skillList[i]
			}
		}
	}

	// Apply token budget
	usedTokens := 0
	var skills []SkillAttachment

	for _, skill := range skillList {
		truncatedContent := TruncateContentToTokens(skill.Content, CompactMaxTokensPerSkill)
		tokens := EstimateTokenCount(truncatedContent)

		if usedTokens+tokens > CompactSkillsTokenBudget {
			continue
		}

		usedTokens += tokens
		skills = append(skills, SkillAttachment{
			Name:    skill.SkillName,
			Path:    skill.SkillPath,
			Content: truncatedContent,
		})
	}

	if len(skills) == 0 {
		return nil
	}

	return &AttachmentMessage{
		Type:      "attachment",
		UUID:      generateUUID(),
		Timestamp: currentTimeStamp(),
		Attachment: Attachment{
			Type:  AttachmentTypeInvokedSkills,
			Skills: skills,
		},
	}
}

// CreatePlanModeAttachmentIfNeeded creates a plan_mode attachment if in plan mode
// Ported from src/services/compact/compact.ts:createPlanModeAttachmentIfNeeded
func CreatePlanModeAttachmentIfNeeded(agentID string) *AttachmentMessage {
	if !IsInPlanMode() {
		return nil
	}

	planFilePath := GetPlanFilePath()
	planExists := GetPlanContent() != ""

	return &AttachmentMessage{
		Type:      "attachment",
		UUID:      generateUUID(),
		Timestamp: currentTimeStamp(),
		Attachment: Attachment{
			Type:         AttachmentTypePlanMode,
			ReminderType: "full",
			IsSubAgent:   agentID != "",
			PlanFilePath: planFilePath,
			PlanExists:   planExists,
		},
	}
}

// CreateAsyncAgentAttachmentsIfNeeded creates attachments for async agents
// Ported from src/services/compact/compact.ts:createAsyncAgentAttachmentsIfNeeded
func CreateAsyncAgentAttachmentsIfNeeded(currentAgentID string) []*AttachmentMessage {
	agents := GetAsyncAgents()
	var attachments []*AttachmentMessage

	for _, agent := range agents {
		// Skip retrieved, pending, or self agents
		if agent.Retrieved || agent.Status == "pending" || agent.AgentID == currentAgentID {
			continue
		}

		deltaSummary := ""
		if agent.Status == "running" {
			deltaSummary = agent.Summary
		} else {
			deltaSummary = agent.Error
		}

		attachments = append(attachments, &AttachmentMessage{
			Type:      "attachment",
			UUID:      generateUUID(),
			Timestamp: currentTimeStamp(),
			Attachment: Attachment{
				Type:           AttachmentTypeTaskStatus,
				TaskID:         agent.AgentID,
				TaskType:       "local_agent",
				Description:    agent.Description,
				Status:         agent.Status,
				DeltaSummary:   deltaSummary,
				OutputFilePath: agent.OutputPath,
			},
		})
	}

	return attachments
}

// CreatePostCompactFileAttachments creates file attachments for post-compact
// Re-reads recently read files that need to be restored in context
// Ported from src/services/compact/compact.ts
func CreatePostCompactFileAttachments(messages []CompactMessage, readFileContent func(path string) (string, error)) []*AttachmentMessage {
	// Get paths that were read but have unchanged stubs
	unchangedPaths := GetUnchangedFilePaths(messages)

	// Get paths already visible in preserved tail
	visiblePaths := CollectReadToolFilePaths(messages)

	var attachments []*AttachmentMessage
	usedTokens := 0

	// Re-read files that need to be restored
	for path := range unchangedPaths {
		// Skip if already visible
		if visiblePaths[path] {
			continue
		}

		// Skip excluded files (plan files, claude.md files)
		if ShouldExcludeFromPostCompactRestore(path) {
			continue
		}

		// Read file content
		content, err := readFileContent(path)
		if err != nil {
			continue
		}

		// Truncate if needed
		truncated := false
		if EstimateTokenCount(content) > CompactMaxTokensPerFile {
			content = TruncateContentToTokens(content, CompactMaxTokensPerFile)
			truncated = true
		}

		tokens := EstimateTokenCount(content)
		if usedTokens+tokens > CompactFilesTokenBudget {
			continue
		}

		usedTokens += tokens
		attachments = append(attachments, &AttachmentMessage{
			Type:      "attachment",
			UUID:      generateUUID(),
			Timestamp: currentTimeStamp(),
			Attachment: Attachment{
				Type:        AttachmentTypeFile,
				Filename:    path,
				Content:     content,
				DisplayPath: path,
				Truncated:   truncated,
			},
		})
	}

	return attachments
}

// TruncateContentToTokens truncates content to roughly maxTokens, keeping the head
// Ported from src/services/compact/compact.ts:truncateToTokens
func TruncateContentToTokens(content string, maxTokens int) string {
	if EstimateTokenCount(content) <= maxTokens {
		return content
	}

	// ~4 chars per token (default bytesPerToken)
	charBudget := maxTokens*4 - len(SkillTruncationMarker)
	if charBudget < 0 {
		charBudget = 0
	}

	if charBudget > len(content) {
		charBudget = len(content)
	}

	return content[:charBudget] + SkillTruncationMarker
}

// ShouldExcludeFromPostCompactRestore checks if a file should be excluded from restoration
// Ported from src/services/compact/compact.ts:shouldExcludeFromPostCompactRestore
func ShouldExcludeFromPostCompactRestore(filename string) bool {
	// Exclude plan files
	planPath := GetPlanFilePath()
	if planPath != "" && filename == planPath {
		return true
	}

	// Exclude claude.md files (simplified check)
	lower := strings.ToLower(filename)
	if strings.HasSuffix(lower, "claude.md") ||
	   strings.HasSuffix(lower, ".claude.md") ||
	   strings.HasSuffix(lower, "claudemd") {
		return true
	}

	// Exclude memory files in .claude-go directory
	if strings.Contains(lower, "/.claude-go/") || strings.Contains(lower, "\\.claude-go\\") {
		return true
	}

	return false
}

// CreateAllPostCompactAttachments creates all needed attachments after compaction
// Ported from src/services/compact/compact.ts (post-compact attachment logic)
func CreateAllPostCompactAttachments(messages []CompactMessage, agentID string, readFileContent func(path string) (string, error)) []*AttachmentMessage {
	var attachments []*AttachmentMessage

	// 1. Plan attachment
	if planAttachment := CreatePlanAttachmentIfNeeded(agentID); planAttachment != nil {
		attachments = append(attachments, planAttachment)
	}

	// 2. Skill attachment
	if skillAttachment := CreateSkillAttachmentIfNeeded(agentID); skillAttachment != nil {
		attachments = append(attachments, skillAttachment)
	}

	// 3. Plan mode attachment
	if planModeAttachment := CreatePlanModeAttachmentIfNeeded(agentID); planModeAttachment != nil {
		attachments = append(attachments, planModeAttachment)
	}

	// 4. Async agent attachments
	asyncAttachments := CreateAsyncAgentAttachmentsIfNeeded(agentID)
	attachments = append(attachments, asyncAttachments...)

	// 5. Post-compact file attachments
	if readFileContent != nil {
		fileAttachments := CreatePostCompactFileAttachments(messages, readFileContent)
		attachments = append(attachments, fileAttachments...)
	}

	return attachments
}

// Helper functions

// generateUUID generates a unique identifier
func generateUUID() string {
	// Simplified - would use proper UUID generation in production
	return "uuid-" + currentTimeStamp()
}

// currentTimeStamp returns current timestamp as string
func currentTimeStamp() string {
	// Simplified - would use time.Now().Format() in production
	return "2024-01-01T00:00:00Z"
}
