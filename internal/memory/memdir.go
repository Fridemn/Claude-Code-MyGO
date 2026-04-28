package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"claude-go/internal/types"
)

// DirExistsGuidance is shared guidance text appended to each memory directory prompt.
const DirExistsGuidance = "This directory already exists — write to it directly with the Write tool (do not run mkdir or check for its existence)."

// MemDirManager manages the memory directory (MEMORY.md and topic files).
type MemDirManager struct {
	memoryDir string
}

// MemDirManager creates a new memory directory manager.
func CreateMemDirManager(memoryDir string) *MemDirManager {
	return &MemDirManager{memoryDir: memoryDir}
}

// EnsureDirExists creates the memory directory if it doesn't exist.
func (m *MemDirManager) EnsureDirExists() error {
	return os.MkdirAll(m.memoryDir, 0755)
}

// BuildMemoryLines builds the typed-memory behavioral instructions.
// Returns lines of text describing how to use the memory system.
func BuildMemoryLines(displayName, memoryDir string, extraGuidelines []string, skipIndex bool) []string {
	var lines []string

	lines = append(lines, fmt.Sprintf("# %s", displayName))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("You have a persistent, file-based memory system at `%s`. %s", memoryDir, DirExistsGuidance))
	lines = append(lines, "")
	lines = append(lines, "You should build up this memory system over time so that future conversations can have a complete picture of who the user is, how they'd like to collaborate with you, what behaviors to avoid or repeat, and the context behind the work the user gives you.")
	lines = append(lines, "")
	lines = append(lines, "If the user explicitly asks you to remember something, save it immediately as whichever type fits best. If they ask you to forget something, find and remove the relevant entry.")
	lines = append(lines, "")

	// Types section
	lines = append(lines, getTypesSection()...)
	lines = append(lines, "")

	// What not to save section
	lines = append(lines, getWhatNotToSaveSection()...)
	lines = append(lines, "")

	// How to save section
	lines = append(lines, getHowToSaveSection(skipIndex)...)
	lines = append(lines, "")

	// When to access section
	lines = append(lines, getWhenToAccessSection()...)
	lines = append(lines, "")

	// Trusting recall section
	lines = append(lines, getTrustingRecallSection()...)
	lines = append(lines, "")

	// Extra guidelines
	if len(extraGuidelines) > 0 {
		lines = append(lines, extraGuidelines...)
		lines = append(lines, "")
	}

	return lines
}

// BuildMemoryPrompt builds the typed-memory prompt with MEMORY.md content included.
func BuildMemoryPrompt(displayName, memoryDir string, extraGuidelines []string) string {
	entrypoint := filepath.Join(memoryDir, EntrypointName)

	// Read existing memory entrypoint
	entrypointContent := ""
	if content, err := os.ReadFile(entrypoint); err == nil {
		entrypointContent = string(content)
	}

	lines := BuildMemoryLines(displayName, memoryDir, extraGuidelines, false)

	if strings.TrimSpace(entrypointContent) != "" {
		t := TruncateEntrypointContent(entrypointContent)
		lines = append(lines, fmt.Sprintf("## %s", EntrypointName))
		lines = append(lines, "")
		lines = append(lines, t.Content)
	} else {
		lines = append(lines, fmt.Sprintf("## %s", EntrypointName))
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("Your %s is currently empty. When you save new memories, they will appear here.", EntrypointName))
	}

	return strings.Join(lines, "\n")
}

// LoadMemoryPrompt loads the unified memory prompt for inclusion in the system prompt.
// Returns nil when auto memory is disabled.
func (m *MemDirManager) LoadMemoryPrompt(ctx context.Context) (string, error) {
	// Ensure directory exists
	if err := m.EnsureDirExists(); err != nil {
		// Log but continue - the model's Write will surface the real error
	}

	return BuildMemoryPrompt("auto memory", m.memoryDir, nil), nil
}

// Memory types and sections

func getTypesSection() []string {
	return []string{
		"## Types of memories",
		"",
		"Your memory system organizes memories into four types:",
		"",
		"<types>",
		"<type>",
		"    <name>user</name>",
		"    <description>Contain information about the user's role, goals, responsibilities, and knowledge. Great user memories help you tailor your future behavior to the user's preferences and perspective.</description>",
		"    <when_to_save>When you learn any details about the user's role, preferences, responsibilities, or knowledge</when_to_save>",
		"    <how_to_use>When your work should be informed by the user's profile or perspective.</how_to_use>",
		"</type>",
		"<type>",
		"    <name>feedback</name>",
		"    <description>Guidance the user has given you about how to approach work — both what to avoid and what to keep doing.</description>",
		"    <when_to_save>Any time the user corrects your approach OR confirms a non-obvious approach worked.</when_to_save>",
		"    <how_to_use>Let these memories guide your behavior so the user does not need to offer the same guidance twice.</how_to_use>",
		"</type>",
		"<type>",
		"    <name>project</name>",
		"    <description>Information that you learn about ongoing work, goals, initiatives, bugs, or incidents within the project that is not otherwise derivable from the code or git history.</description>",
		"    <when_to_save>When you learn who is doing what, why, or by when.</when_to_save>",
		"    <how_to_use>Use these memories to more fully understand the details and nuance behind the user's request.</how_to_use>",
		"</type>",
		"<type>",
		"    <name>reference</name>",
		"    <description>Stores pointers to where information can be found in external systems.</description>",
		"    <when_to_save>When you learn about resources in external systems and their purpose.</when_to_save>",
		"    <how_to_use>When the user references an external system or information that may be in an external system.</how_to_use>",
		"</type>",
		"</types>",
	}
}

func getWhatNotToSaveSection() []string {
	return []string{
		"## What NOT to save in memory",
		"",
		"- Code patterns, conventions, architecture, file paths, or project structure — these can be derived by reading the current project state.",
		"- Git history, recent changes, or who-changed-what — `git log` / `git blame` are authoritative.",
		"- Debugging solutions or fix recipes — the fix is in the code; the commit message has the context.",
		"- Anything already documented in CLAUDE.md files.",
		"- Ephemeral task details: in-progress work, temporary state, current conversation context.",
	}
}

func getHowToSaveSection(skipIndex bool) []string {
	if skipIndex {
		return []string{
			"## How to save memories",
			"",
			"Write each memory to its own file (e.g., `user_role.md`, `feedback_testing.md`) using this frontmatter format:",
			"",
			"```markdown",
			"---",
			"name: {{memory name}}",
			"description: {{one-line description — used to decide relevance in future conversations}}",
			"type: {{user, feedback, project, reference}}",
			"---",
			"",
			"{{memory content}}",
			"```",
			"",
			"- Keep the name, description, and type fields in memory files up-to-date with the content",
			"- Organize memory semantically by topic, not chronologically",
			"- Update or remove memories that turn out to be wrong or outdated",
			"- Do not write duplicate memories. First check if there is an existing memory you can update before writing a new one.",
		}
	}

	return []string{
		"## How to save memories",
		"",
		"Saving a memory is a two-step process:",
		"",
		"**Step 1** — write the memory to its own file (e.g., `user_role.md`, `feedback_testing.md`) using this frontmatter format:",
		"",
		"```markdown",
		"---",
		"name: {{memory name}}",
		"description: {{one-line description — used to decide relevance in future conversations}}",
		"type: {{user, feedback, project, reference}}",
		"---",
		"",
		"{{memory content}}",
		"```",
		"",
		fmt.Sprintf("**Step 2** — add a pointer to that file in `%s`. `%s` is an index, not a memory — each entry should be one line, under ~150 characters: `- [Title](file.md) — one-line hook`. It has no frontmatter. Never write memory content directly into `%s`.", EntrypointName, EntrypointName, EntrypointName),
		"",
		fmt.Sprintf("- `%s` is always loaded into your conversation context — lines after %d will be truncated, so keep the index concise", EntrypointName, MaxEntrypointLines),
		"- Keep the name, description, and type fields in memory files up-to-date with the content",
		"- Organize memory semantically by topic, not chronologically",
		"- Update or remove memories that turn out to be wrong or outdated",
		"- Do not write duplicate memories. First check if there is an existing memory you can update before writing a new one.",
	}
}

func getWhenToAccessSection() []string {
	return []string{
		"## When to access memories",
		"",
		"When memories seem relevant, or the user references prior-conversation work.",
		"You MUST access memory when the user explicitly asks you to check, recall, or remember.",
		"If the user says to *ignore* or *not use* memory: proceed as if MEMORY.md were empty. Do not apply remembered facts, cite, compare against, or mention memory content.",
		"Memory records can become stale over time. Use memory as context for what was true at a given point in time. Before answering the user or building assumptions based solely on information in memory records, verify that the memory is still correct and up-to-date by reading the current state of the files or resources.",
	}
}

func getTrustingRecallSection() []string {
	return []string{
		"## Before recommending from memory",
		"",
		"A memory that names a specific function, file, or flag is a claim that it existed *when the memory was written*. It may have been renamed, removed, or never merged. Before recommending it:",
		"",
		"- If the memory names a file path: check the file exists.",
		"- If the memory names a function or flag: grep for it.",
		"- If the user is about to act on your recommendation (not just asking about history), verify first.",
		"",
		"\"The memory says X exists\" is not the same as \"X exists now.\"",
	}
}

// GetAutoMemPath returns the path to the auto-memory directory.
func GetAutoMemPath(configHomeDir, originalCwd string) string {
	projectHash := getProjectHash(originalCwd)
	return filepath.Join(configHomeDir, "projects", projectHash, "memory")
}

// IsAutoMemPath checks if a path is within the auto-memory directory.
func IsAutoMemPath(path, autoMemDir string) bool {
	return strings.HasPrefix(path, autoMemDir)
}

// ReadMemoriesForSurfacing reads memory files for surfacing to the model.
func ReadMemoriesForSurfacing(ctx context.Context, memoryDir string, memories []types.RelevantMemory) (map[string]string, error) {
	result := make(map[string]string)

	for _, mem := range memories {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		content, err := os.ReadFile(mem.Path)
		if err != nil {
			continue
		}

		// Generate header with freshness info
		header := generateMemoryHeader(mem.Path, mem.MtimeMs)
		result[mem.Path] = header + "\n\n" + string(content)
	}

	return result, nil
}

func generateMemoryHeader(path string, mtimeMs int64) string {
	// TODO: Format with freshness info
	return fmt.Sprintf("Memory file: %s", path)
}
