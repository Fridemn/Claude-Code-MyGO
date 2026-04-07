// Package memory provides memory file management for the Claude Code CLI.
// It handles loading, parsing, and retrieval of memory files (CLAUDE.md, etc.)
// following the same architecture as the TypeScript implementation.
package memory

import (
	"claude-code-go/internal/types"
)

// Constants for memory file processing.
const (
	// EntrypointName is the name of the memory directory entrypoint file.
	EntrypointName = "MEMORY.md"

	// MaxEntrypointLines is the maximum number of lines for MEMORY.md.
	MaxEntrypointLines = 200

	// MaxEntrypointBytes is the maximum size for MEMORY.md (~25KB).
	MaxEntrypointBytes = 25000

	// MaxMemoryFiles is the maximum number of memory files to scan.
	MaxMemoryFiles = 200

	// FrontmatterMaxLines is the maximum lines to read for frontmatter parsing.
	FrontmatterMaxLines = 30

	// MaxMemoryCharacterCount is the recommended max character count for a memory file.
	MaxMemoryCharacterCount = 40000

	// MaxIncludeDepth is the maximum depth for @include directives.
	MaxIncludeDepth = 5

	// MemoryInstructionPrompt is the prompt prefix for memory content.
	MemoryInstructionPrompt = "Codebase and user instructions are shown below. Be sure to adhere to these instructions. IMPORTANT: These instructions OVERRIDE any default behavior and MUST MUST them exactly as written."
)

// TextFileExtensions are allowed for @include directives.
// This prevents binary files from being loaded into memory.
var TextFileExtensions = map[string]bool{
	// Markdown and text
	".md": true, ".txt": true, ".text": true,
	// Data formats
	".json": true, ".yaml": true, ".yml": true, ".toml": true, ".xml": true, ".csv": true,
	// Web
	".html": true, ".htm": true, ".css": true, ".scss": true, ".sass": true, ".less": true,
	// JavaScript/TypeScript
	".js": true, ".ts": true, ".tsx": true, ".jsx": true, ".mjs": true, ".cjs": true, ".mts": true, ".cts": true,
	// Python
	".py": true, ".pyi": true, ".pyw": true,
	// Ruby
	".rb": true, ".erb": true, ".rake": true,
	// Go
	".go": true,
	// Rust
	".rs": true,
	// Java/Kotlin/Scala
	".java": true, ".kt": true, ".kts": true, ".scala": true,
	// C/C++
	".c": true, ".cpp": true, ".cc": true, ".cxx": true, ".h": true, ".hpp": true, ".hxx": true,
	// C#
	".cs": true,
	// Swift
	".swift": true,
	// Shell
	".sh": true, ".bash": true, ".zsh": true, ".fish": true, ".ps1": true, ".bat": true, ".cmd": true,
	// Config
	".env": true, ".ini": true, ".cfg": true, ".conf": true, ".config": true, ".properties": true,
	// Database
	".sql": true, ".graphql": true, ".gql": true,
	// Protocol
	".proto": true,
	// Frontend frameworks
	".vue": true, ".svelte": true, ".astro": true,
	// Templating
	".ejs": true, ".hbs": true, ".pug": true, ".jade": true,
	// Other languages
	".php": true, ".pl": true, ".pm": true, ".lua": true, ".r": true, ".R": true,
	".dart": true, ".ex": true, ".exs": true, ".erl": true, ".hrl": true,
	".clj": true, ".cljs": true, ".cljc": true, ".edn": true,
	".hs": true, ".lhs": true, ".elm": true, ".ml": true, ".mli": true,
	".f": true, ".f90": true, ".f95": true, ".for": true,
	// Build files
	".cmake": true, ".make": true, ".makefile": true, ".gradle": true, ".sbt": true,
	// Documentation
	".rst": true, ".adoc": true, ".asciidoc": true, ".org": true, ".tex": true, ".latex": true,
	// Lock files
	".lock": true,
	// Misc
	".log": true, ".diff": true, ".patch": true,
}

// MemoryTypeValues returns all memory type values for iteration.
func MemoryTypeValues() []types.MemoryType {
	return []types.MemoryType{
		types.MemoryTypeManaged,
		types.MemoryTypeUser,
		types.MemoryTypeProject,
		types.MemoryTypeLocal,
		types.MemoryTypeAutoMem,
		types.MemoryTypeTeamMem,
	}
}

// IsTextFileExtension checks if a file extension is allowed for @include.
func IsTextFileExtension(ext string) bool {
	return TextFileExtensions[ext]
}