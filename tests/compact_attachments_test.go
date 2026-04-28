package tests

import (
	"testing"

	"claude-go/internal/services"
)

func TestPlanState(t *testing.T) {
	t.Parallel()

	// Test initial state
	if services.GetPlanContent() != "" {
		t.Error("Plan content should be empty initially")
	}

	// Set plan content
	services.SetPlanContent("# My Plan\n\n1. Step one\n2. Step two")
	if services.GetPlanContent() == "" {
		t.Error("Plan content should not be empty after setting")
	}

	// Set plan file path
	services.SetPlanFilePath("/path/to/plan.md")
	if services.GetPlanFilePath() != "/path/to/plan.md" {
		t.Errorf("Plan file path mismatch: got %s", services.GetPlanFilePath())
	}

	// Clear
	services.SetPlanContent("")
	services.SetPlanFilePath("")
}

func TestInvokedSkillsState(t *testing.T) {
	t.Parallel()

	// Clear first
	services.ClearInvokedSkills()

	// Test initial state
	skills := services.GetInvokedSkillsForAgent("")
	if len(skills) != 0 {
		t.Error("Skills should be empty initially")
	}

	// Add skill
	services.AddInvokedSkill("test-skill", "/path/to/skill.md", "Skill content here")
	skills = services.GetInvokedSkillsForAgent("")
	if len(skills) != 1 {
		t.Errorf("Expected 1 skill, got %d", len(skills))
	}

	if skills["test-skill"].SkillName != "test-skill" {
		t.Errorf("Skill name mismatch: got %s", skills["test-skill"].SkillName)
	}

	// Clear
	services.ClearInvokedSkills()
	skills = services.GetInvokedSkillsForAgent("")
	if len(skills) != 0 {
		t.Error("Skills should be empty after clear")
	}
}

func TestAsyncAgentsState(t *testing.T) {
	t.Parallel()

	// Add agent
	agent := &services.AsyncAgentState{
		AgentID:     "agent-1",
		Description: "Test agent",
		Status:      "running",
		Summary:     "Working on task",
	}
	services.AddAsyncAgent(agent)

	agents := services.GetAsyncAgents()
	if len(agents) != 1 {
		t.Errorf("Expected 1 agent, got %d", len(agents))
	}

	if agents["agent-1"].Status != "running" {
		t.Errorf("Agent status mismatch: got %s", agents["agent-1"].Status)
	}

	// Remove agent
	services.RemoveAsyncAgent("agent-1")
	agents = services.GetAsyncAgents()
	if len(agents) != 0 {
		t.Error("Agents should be empty after removal")
	}
}

func TestPlanModeState(t *testing.T) {
	t.Parallel()

	// Test initial state
	if services.IsInPlanMode() {
		t.Error("Should not be in plan mode initially")
	}

	// Set plan mode
	services.SetPlanMode(true)
	if !services.IsInPlanMode() {
		t.Error("Should be in plan mode after setting")
	}

	// Reset
	services.SetPlanMode(false)
	if services.IsInPlanMode() {
		t.Error("Should not be in plan mode after resetting")
	}
}

func TestCreatePlanAttachmentIfNeeded(t *testing.T) {
	t.Parallel()

	// No plan content - should return nil
	services.SetPlanContent("")
	attachment := services.CreatePlanAttachmentIfNeeded("")
	if attachment != nil {
		t.Error("Expected nil when no plan content")
	}

	// With plan content
	services.SetPlanContent("# Plan\n1. Step one")
	services.SetPlanFilePath("/path/to/plan.md")
	attachment = services.CreatePlanAttachmentIfNeeded("")
	if attachment == nil {
		t.Fatal("Expected non-nil attachment")
	}

	if attachment.Attachment.Type != services.AttachmentTypePlanFileReference {
		t.Errorf("Wrong attachment type: got %s", attachment.Attachment.Type)
	}

	if attachment.Attachment.PlanContent != "# Plan\n1. Step one" {
		t.Errorf("Plan content mismatch: got %s", attachment.Attachment.PlanContent)
	}

	// Cleanup
	services.SetPlanContent("")
	services.SetPlanFilePath("")
}

func TestCreateSkillAttachmentIfNeeded(t *testing.T) {
	t.Parallel()

	// Clear skills first
	services.ClearInvokedSkills()

	// No skills - should return nil
	attachment := services.CreateSkillAttachmentIfNeeded("")
	if attachment != nil {
		t.Error("Expected nil when no skills")
	}

	// Add skill
	services.AddInvokedSkill("test-skill", "/path/to/skill.md", "Test skill content")
	attachment = services.CreateSkillAttachmentIfNeeded("")
	if attachment == nil {
		t.Fatal("Expected non-nil attachment")
	}

	if attachment.Attachment.Type != services.AttachmentTypeInvokedSkills {
		t.Errorf("Wrong attachment type: got %s", attachment.Attachment.Type)
	}

	if len(attachment.Attachment.Skills) != 1 {
		t.Errorf("Expected 1 skill, got %d", len(attachment.Attachment.Skills))
	}

	// Cleanup
	services.ClearInvokedSkills()
}

func TestCreatePlanModeAttachmentIfNeeded(t *testing.T) {
	t.Parallel()

	// Not in plan mode - should return nil
	services.SetPlanMode(false)
	attachment := services.CreatePlanModeAttachmentIfNeeded("")
	if attachment != nil {
		t.Error("Expected nil when not in plan mode")
	}

	// In plan mode
	services.SetPlanMode(true)
	attachment = services.CreatePlanModeAttachmentIfNeeded("")
	if attachment == nil {
		t.Fatal("Expected non-nil attachment")
	}

	if attachment.Attachment.Type != services.AttachmentTypePlanMode {
		t.Errorf("Wrong attachment type: got %s", attachment.Attachment.Type)
	}

	if attachment.Attachment.ReminderType != "full" {
		t.Errorf("Reminder type should be 'full', got %s", attachment.Attachment.ReminderType)
	}

	// Cleanup
	services.SetPlanMode(false)
}

func TestCreateAsyncAgentAttachmentsIfNeeded(t *testing.T) {
	t.Parallel()

	// No agents - should return empty
	attachments := services.CreateAsyncAgentAttachmentsIfNeeded("")
	if len(attachments) != 0 {
		t.Errorf("Expected 0 attachments, got %d", len(attachments))
	}

	// Add running agent
	agent := &services.AsyncAgentState{
		AgentID:     "agent-1",
		Description: "Test agent",
		Status:      "running",
		Summary:     "Working",
	}
	services.AddAsyncAgent(agent)

	attachments = services.CreateAsyncAgentAttachmentsIfNeeded("other-agent")
	if len(attachments) != 1 {
		t.Errorf("Expected 1 attachment, got %d", len(attachments))
	}

	if attachments[0].Attachment.Type != services.AttachmentTypeTaskStatus {
		t.Errorf("Wrong attachment type: got %s", attachments[0].Attachment.Type)
	}

	// Agent should be skipped when it's the current agent
	attachments = services.CreateAsyncAgentAttachmentsIfNeeded("agent-1")
	if len(attachments) != 0 {
		t.Errorf("Expected 0 attachments when current agent matches, got %d", len(attachments))
	}

	// Add retrieved agent - should be skipped
	agent2 := &services.AsyncAgentState{
		AgentID:   "agent-2",
		Status:    "completed",
		Retrieved: true,
	}
	services.AddAsyncAgent(agent2)

	// Add pending agent - should be skipped
	agent3 := &services.AsyncAgentState{
		AgentID: "agent-3",
		Status:  "pending",
	}
	services.AddAsyncAgent(agent3)

	attachments = services.CreateAsyncAgentAttachmentsIfNeeded("other-agent")
	if len(attachments) != 1 {
		t.Errorf("Expected 1 attachment (only running non-retrieved), got %d", len(attachments))
	}

	// Cleanup
	services.RemoveAsyncAgent("agent-1")
	services.RemoveAsyncAgent("agent-2")
	services.RemoveAsyncAgent("agent-3")
}

func TestTruncateToTokens(t *testing.T) {
	t.Parallel()

	// Short content - no truncation
	shortContent := "short content"
	result := services.TruncateContentToTokens(shortContent, 1000)
	if result != shortContent {
		t.Error("Short content should not be truncated")
	}

	// Long content - should truncate
	longContent := ""
	for i := 0; i < 10000; i++ {
		longContent += "x"
	}
	result = services.TruncateContentToTokens(longContent, 100)
	if result == longContent {
		t.Error("Long content should be truncated")
	}

	// Check truncation marker is present
	if !containsSubstring(result, services.SkillTruncationMarker) {
		t.Error("Truncated content should contain truncation marker")
	}
}

func TestShouldExcludeFromPostCompactRestore(t *testing.T) {
	t.Parallel()

	// Plan file should be excluded
	services.SetPlanFilePath("/path/to/plan.md")
	if !services.ShouldExcludeFromPostCompactRestore("/path/to/plan.md") {
		t.Error("Plan file should be excluded")
	}

	// claude.md files should be excluded
	if !services.ShouldExcludeFromPostCompactRestore("/path/to/CLAUDE.md") {
		t.Error("CLAUDE.md should be excluded")
	}

	// Memory files should be excluded
	if !services.ShouldExcludeFromPostCompactRestore("/home/user/.claude/memory.md") {
		t.Error(".claude memory file should be excluded")
	}

	// Regular files should not be excluded
	if services.ShouldExcludeFromPostCompactRestore("/path/to/regular.txt") {
		t.Error("Regular file should not be excluded")
	}

	// Cleanup
	services.SetPlanFilePath("")
}

func TestCreatePostCompactFileAttachments(t *testing.T) {
	t.Parallel()

	// Create messages with unchanged stub
	messages := []services.CompactMessage{
		{
			Type: services.MessageTypeAssistant,
			ToolCalls: []services.ToolCallContent{
				{
					ID:        "tool-1",
					Name:      services.FileReadToolName,
					Arguments: `{"file_path": "/path/to/file.txt"}`,
				},
			},
		},
		{
			Type: services.MessageTypeUser,
			ToolResults: []services.ToolResultContent{
				{
					ToolUseID: "tool-1",
					Content:   services.FileUnchangedStub,
				},
			},
		},
	}

	// Mock file reader
	readFileContent := func(path string) (string, error) {
		return "file content for " + path, nil
	}

	attachments := services.CreatePostCompactFileAttachments(messages, readFileContent)

	// Should have attachment for the unchanged file
	if len(attachments) != 1 {
		t.Errorf("Expected 1 attachment, got %d", len(attachments))
	}

	if attachments[0].Attachment.Filename != "/path/to/file.txt" {
		t.Errorf("Filename mismatch: got %s", attachments[0].Attachment.Filename)
	}
}

func TestCreateAllPostCompactAttachments(t *testing.T) {
	t.Parallel()

	// Setup state
	services.SetPlanContent("# Plan")
	services.SetPlanFilePath("/plan.md")
	services.SetPlanMode(true)
	services.AddInvokedSkill("skill1", "/skill.md", "skill content")

	// Create messages
	messages := []services.CompactMessage{}

	// Mock file reader
	readFileContent := func(path string) (string, error) {
		return "content", nil
	}

	attachments := services.CreateAllPostCompactAttachments(messages, "", readFileContent)

	// Should have plan, skill, and plan mode attachments
	if len(attachments) < 2 {
		t.Errorf("Expected at least 2 attachments, got %d", len(attachments))
	}

	// Cleanup
	services.SetPlanContent("")
	services.SetPlanFilePath("")
	services.SetPlanMode(false)
	services.ClearInvokedSkills()
}

func TestAttachmentTokenBudget(t *testing.T) {
	t.Parallel()

	// Clear skills
	services.ClearInvokedSkills()

	// Add skill with large content
	largeContent := ""
	for i := 0; i < 100000; i++ {
		largeContent += "x"
	}
	services.AddInvokedSkill("large-skill", "/skill.md", largeContent)

	attachment := services.CreateSkillAttachmentIfNeeded("")
	if attachment == nil {
		t.Fatal("Expected non-nil attachment")
	}

	// Content should be truncated
	if attachment.Attachment.Skills[0].Content == largeContent {
		t.Error("Large content should be truncated")
	}

	// Check truncation marker
	if !containsSubstring(attachment.Attachment.Skills[0].Content, services.SkillTruncationMarker) {
		t.Error("Truncated content should have marker")
	}

	// Cleanup
	services.ClearInvokedSkills()
}

// Helper function

