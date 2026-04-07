package services

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"claude-code-go/internal/types"
	"claude-code-go/internal/utils"
)

// ExtractMemoriesService extracts durable memories from the current session transcript
// and writes them to the auto-memory directory (~/.claude/projects/<path>/memory/).
type ExtractMemoriesService struct {
	memoryDir string
	enabled   bool

	// State tracking
	lastMemoryMessageUuid string
	inProgress             bool
	turnsSinceExtraction   int
	mu                     sync.Mutex
}

// CreateExtractMemoriesService creates a new memory extraction service.
func CreateExtractMemoriesService(memoryDir string) *ExtractMemoriesService {
	return &ExtractMemoriesService{
		memoryDir: memoryDir,
		enabled:   true,
	}
}

// Execute runs memory extraction at the end of a query loop.
// Called fire-and-forget from stop hooks.
func (s *ExtractMemoriesService) Execute(ctx context.Context, messages []types.Message, appendSystemMessage func(msg types.Message)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.enabled {
		return nil
	}

	// Only run for the main agent, not subagents
	// In the full implementation, we'd check the agent ID

	// Check if already in progress
	if s.inProgress {
		return nil
	}

	// Check turn throttle
	s.turnsSinceExtraction++
	if s.turnsSinceExtraction < 1 {
		return nil
	}

	s.inProgress = true
	defer func() {
		s.inProgress = false
	}()

	// Check if main agent already wrote memories
	if s.hasMemoryWritesSince(messages, s.lastMemoryMessageUuid) {
		// Skip extraction, advance cursor
		if len(messages) > 0 {
			s.lastMemoryMessageUuid = messages[len(messages)-1].UUID
		}
		return nil
	}

	// Extract memories
	writtenPaths, err := s.runExtraction(ctx, messages)
	if err != nil {
		return err
	}

	// Advance cursor
	if len(messages) > 0 {
		s.lastMemoryMessageUuid = messages[len(messages)-1].UUID
	}
	s.turnsSinceExtraction = 0

	// Filter out MEMORY.md updates
	var memoryPaths []string
	for _, p := range writtenPaths {
		if !strings.HasSuffix(p, "MEMORY.md") {
			memoryPaths = append(memoryPaths, p)
		}
	}

	// Create system notification if memories were saved
	if len(memoryPaths) > 0 && appendSystemMessage != nil {
		msg := createMemorySavedMessage(memoryPaths)
		appendSystemMessage(msg)
	}

	return nil
}

// hasMemoryWritesSince checks if any assistant message after the cursor contains
// a Write/Edit tool_use block targeting an auto-memory path.
func (s *ExtractMemoriesService) hasMemoryWritesSince(messages []types.Message, sinceUuid string) bool {
	found := sinceUuid == ""
	for _, msg := range messages {
		if !found {
			if msg.UUID == sinceUuid {
				found = true
			}
			continue
		}

		// Check for Write/Edit tool calls targeting memory files
		if msg.Role == "assistant" && msg.HasToolCalls() {
			for _, tc := range msg.ToolCalls {
				toolName := tc.Name
				if toolName == "Write" || toolName == "Edit" {
					// Parse arguments to find file path
					args := tc.Arguments
					if strings.Contains(args, s.memoryDir) {
						return true
					}
				}
			}
		}
	}
	return false
}

// runExtraction performs the actual memory extraction.
func (s *ExtractMemoriesService) runExtraction(ctx context.Context, messages []types.Message) ([]string, error) {
	// In the full implementation, this would:
	// 1. Scan existing memory files for manifest
	// 2. Build extraction prompt
	// 3. Run a forked agent with restricted tool permissions
	// 4. Return written paths

	// For now, return empty (no memories extracted)
	return nil, nil
}

// createMemorySavedMessage creates a system message about saved memories.
func createMemorySavedMessage(paths []string) types.Message {
	// Use the paths to create a message about saved memories
	_ = paths // Paths used in full implementation
	return types.Message{
		Role:    "system",
		Content: "Memories saved successfully",
	}
}

// Drain awaits all in-flight extractions with a soft timeout.
func (s *ExtractMemoriesService) Drain(timeoutMs int) {
	deadline := time.Duration(timeoutMs) * time.Millisecond
	start := time.Now()
	for time.Since(start) < deadline {
		s.mu.Lock()
		inProgress := s.inProgress
		s.mu.Unlock()

		if !inProgress {
			return
		}

		// Sleep briefly before checking again
		time.Sleep(10 * time.Millisecond)
	}
}

// Memory extraction prompts

const extractMemoriesSystemPrompt = `You are extracting durable memories from a conversation transcript.
Read the conversation and identify any information that should be remembered for future conversations.

Guidelines:
- Only save information that will be useful in FUTURE conversations
- DO NOT save code patterns, file paths, or architecture details (these can be derived from the codebase)
- DO save user preferences, feedback, project context, and pointers to external systems
- Each memory should be a single topic in its own file
- Update existing memories rather than creating duplicates`

// BuildExtractPrompt builds the prompt for memory extraction.
func BuildExtractPrompt(newMessageCount int, existingMemories string) string {
	var sb strings.Builder

	sb.WriteString("Extract memories from the last ")
	sb.WriteString(intToString(newMessageCount))
	sb.WriteString(" messages.\n\n")

	if existingMemories != "" {
		sb.WriteString("Existing memories in the directory:\n")
		sb.WriteString(existingMemories)
		sb.WriteString("\n\n")
		sb.WriteString("Check these before creating new memories to avoid duplicates.\n\n")
	}

	sb.WriteString("Save memories that will be useful in future conversations.\n")
	sb.WriteString("Focus on:\n")
	sb.WriteString("- User preferences and feedback\n")
	sb.WriteString("- Project context (deadlines, decisions, stakeholders)\n")
	sb.WriteString("- Pointers to external systems (dashboards, issue trackers)\n")
	sb.WriteString("- Anything the user explicitly asked to remember\n")

	return sb.String()
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	var result []byte
	for n > 0 {
		result = append([]byte{byte('0' + n%10)}, result...)
		n /= 10
	}
	return string(result)
}

// GetAutoMemPath returns the path to the auto-memory directory.
func GetAutoMemPath(configHomeDir, originalCwd string) string {
	projectHash := getProjectHash(originalCwd)
	return filepath.Join(configHomeDir, "projects", projectHash, "memory")
}

func getProjectHash(cwd string) string {
	// Use djb2-based sanitization matching TS implementation
	return utils.SanitizePath(cwd)
}