package types

// MemoryType represents the type of memory file.
// Files are loaded in the following order (reverse priority - later files have higher priority):
// 1. Managed - Global instructions for all users (e.g., /etc/claude-code/CLAUDE.md)
// 2. User - Private global instructions for all projects (~/.claude/CLAUDE.md)
// 3. Project - Instructions checked into the codebase (CLAUDE.md, .claude/CLAUDE.md, .claude/rules/*.md)
// 4. Local - Private project-specific instructions (CLAUDE.local.md)
// 5. AutoMem - User's auto-memory, persists across conversations
// 6. TeamMem - Shared team memory, synced across the organization
type MemoryType string

const (
	MemoryTypeManaged  MemoryType = "Managed"
	MemoryTypeUser     MemoryType = "User"
	MemoryTypeProject  MemoryType = "Project"
	MemoryTypeLocal    MemoryType = "Local"
	MemoryTypeAutoMem  MemoryType = "AutoMem"
	MemoryTypeTeamMem  MemoryType = "TeamMem"
)

// MemoryFileInfo represents information about a loaded memory file.
type MemoryFileInfo struct {
	// Path is the absolute path to the memory file
	Path string `json:"path"`
	// Type is the memory type classification
	Type MemoryType `json:"type"`
	// Content is the processed content of the file
	Content string `json:"content"`
	// Parent is the path of the file that included this one (for @include)
	Parent string `json:"parent,omitempty"`
	// Globs are glob patterns for conditional rules (from frontmatter paths)
	Globs []string `json:"globs,omitempty"`
	// ContentDiffersFromDisk indicates if content was transformed (stripped comments, etc.)
	ContentDiffersFromDisk bool `json:"contentDiffersFromDisk,omitempty"`
	// RawContent holds the unmodified disk bytes when contentDiffersFromDisk is true
	RawContent string `json:"rawContent,omitempty"`
}

// MemoryHeader represents the header information extracted from a memory file.
// Used for memory scanning and retrieval without reading the full content.
type MemoryHeader struct {
	// Filename is the relative path within the memory directory
	Filename string `json:"filename"`
	// FilePath is the absolute path to the memory file
	FilePath string `json:"filePath"`
	// MtimeMs is the modification time in milliseconds
	MtimeMs int64 `json:"mtimeMs"`
	// Description is extracted from frontmatter
	Description string `json:"description,omitempty"`
	// Type is the memory type from frontmatter
	Type MemoryType `json:"type,omitempty"`
}

// RelevantMemory represents a memory file selected as relevant to a query.
type RelevantMemory struct {
	Path    string `json:"path"`
	MtimeMs int64  `json:"mtimeMs"`
}

// MemoryEntrypointTruncation represents truncation info for MEMORY.md files.
type MemoryEntrypointTruncation struct {
	Content         string `json:"content"`
	LineCount       int    `json:"lineCount"`
	ByteCount       int    `json:"byteCount"`
	WasLineTruncated  bool `json:"wasLineTruncated"`
	WasByteTruncated bool `json:"wasByteTruncated"`
}

// IsInstructionsMemoryType returns true if the type is an instruction memory type
// (not AutoMem or TeamMem which are separate memory systems).
func (t MemoryType) IsInstructionsMemoryType() bool {
	switch t {
	case MemoryTypeManaged, MemoryTypeUser, MemoryTypeProject, MemoryTypeLocal:
		return true
	default:
		return false
	}
}

// String returns the string representation of the memory type.
func (t MemoryType) String() string {
	return string(t)
}

// ParseMemoryType parses a string into a MemoryType.
func ParseMemoryType(s string) MemoryType {
	switch s {
	case "user":
		return MemoryTypeUser
	case "feedback":
		return MemoryTypeUser // feedback is a subtype of user
	case "project":
		return MemoryTypeProject
	case "reference":
		return MemoryTypeProject // reference is a subtype of project
	case "managed":
		return MemoryTypeManaged
	case "local":
		return MemoryTypeLocal
	case "automem", "auto":
		return MemoryTypeAutoMem
	case "teammem", "team":
		return MemoryTypeTeamMem
	default:
		return MemoryType(s)
	}
}