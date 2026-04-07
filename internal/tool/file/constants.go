package file

// Constants matching the TypeScript implementation
const (
	// Tool names matching TS
	FileReadToolName  = "Read"
	FileWriteToolName = "Write"
	FileEditToolName  = "Edit"

	// File unchanged stub message
	FileUnchangedStub = "File unchanged since last read. The content from the earlier Read tool_result in this conversation is still current — refer to that instead of re-reading."

	// Max lines to read by default
	MaxLinesToRead = 2000

	// Line format instruction
	LineFormatInstruction = "- Results are returned using cat -n format, with line numbers starting at 1"

	// Offset instructions
	OffsetInstructionDefault  = "- You can optionally specify a line offset and limit (especially handy for long files), but it's recommended to read the whole file by not providing these parameters"
	OffsetInstructionTargeted = "- When you already know which part of the file you need, only read that part. This can be important for larger files."

	// File unexpectedly modified error
	FileUnexpectedlyModifiedError = "File has been unexpectedly modified. Read it again before attempting to write it."
)

// Blocked device paths that would hang the process
var BlockedDevicePaths = map[string]bool{
	"/dev/zero":     true,
	"/dev/random":   true,
	"/dev/urandom":  true,
	"/dev/full":     true,
	"/dev/stdin":    true,
	"/dev/tty":      true,
	"/dev/console":  true,
	"/dev/stdout":   true,
	"/dev/stderr":   true,
	"/dev/fd/0":     true,
	"/dev/fd/1":     true,
	"/dev/fd/2":     true,
}

// IsBlockedDevicePath checks if a path is a blocked device path
func IsBlockedDevicePath(path string) bool {
	if BlockedDevicePaths[path] {
		return true
	}
	// Check for /proc/self/fd/0-2 and /proc/<pid>/fd/0-2
	if len(path) > 6 && path[:6] == "/proc/" {
		if path[len(path)-5:] == "/fd/0" ||
			path[len(path)-5:] == "/fd/1" ||
			path[len(path)-5:] == "/fd/2" {
			return true
		}
	}
	return false
}

// Common image extensions
var ImageExtensions = map[string]bool{
	"png":  true,
	"jpg":  true,
	"jpeg": true,
	"gif":  true,
	"webp": true,
}

// IsImageExtension checks if a file extension is an image type
func IsImageExtension(ext string) bool {
	return ImageExtensions[ext]
}