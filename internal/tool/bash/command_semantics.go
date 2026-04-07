package bash


import "strings"

// CommandSemantic interprets command exit codes with context-aware rules
type CommandSemantic struct {
	IsError bool
	Message string
}

// CommandSemanticFunc is a function type for command semantic interpretation
type CommandSemanticFunc func(exitCode int, stdout, stderr string) CommandSemantic

// defaultSemantic treats only 0 as success
func defaultSemantic(exitCode int, stdout, stderr string) CommandSemantic {
	if exitCode != 0 {
		return CommandSemantic{
			IsError: true,
			Message: "Command failed with exit code " + Itoa(exitCode),
		}
	}
	return CommandSemantic{IsError: false}
}

// commandSemantics maps base commands to their semantic interpreters
var commandSemantics = map[string]CommandSemanticFunc{
	// grep: 0=matches found, 1=no matches, 2+=error
	"grep": func(exitCode int, stdout, stderr string) CommandSemantic {
		if exitCode >= 2 {
			return CommandSemantic{IsError: true}
		}
		if exitCode == 1 {
			return CommandSemantic{IsError: false, Message: "No matches found"}
		}
		return CommandSemantic{IsError: false}
	},

	// ripgrep has same semantics as grep
	"rg": func(exitCode int, stdout, stderr string) CommandSemantic {
		if exitCode >= 2 {
			return CommandSemantic{IsError: true}
		}
		if exitCode == 1 {
			return CommandSemantic{IsError: false, Message: "No matches found"}
		}
		return CommandSemantic{IsError: false}
	},

	// find: 0=success, 1=partial success, 2+=error
	"find": func(exitCode int, stdout, stderr string) CommandSemantic {
		if exitCode >= 2 {
			return CommandSemantic{IsError: true}
		}
		if exitCode == 1 {
			return CommandSemantic{IsError: false, Message: "Some directories were inaccessible"}
		}
		return CommandSemantic{IsError: false}
	},

	// diff: 0=no differences, 1=differences found, 2+=error
	"diff": func(exitCode int, stdout, stderr string) CommandSemantic {
		if exitCode >= 2 {
			return CommandSemantic{IsError: true}
		}
		if exitCode == 1 {
			return CommandSemantic{IsError: false, Message: "Files differ"}
		}
		return CommandSemantic{IsError: false}
	},

	// test/[: 0=condition true, 1=condition false, 2+=error
	"test": func(exitCode int, stdout, stderr string) CommandSemantic {
		if exitCode >= 2 {
			return CommandSemantic{IsError: true}
		}
		if exitCode == 1 {
			return CommandSemantic{IsError: false, Message: "Condition is false"}
		}
		return CommandSemantic{IsError: false}
	},

	// [ is an alias for test
	"[": func(exitCode int, stdout, stderr string) CommandSemantic {
		if exitCode >= 2 {
			return CommandSemantic{IsError: true}
		}
		if exitCode == 1 {
			return CommandSemantic{IsError: false, Message: "Condition is false"}
		}
		return CommandSemantic{IsError: false}
	},

	// npm: 0=success, 1=error
	"npm": func(exitCode int, stdout, stderr string) CommandSemantic {
		if exitCode != 0 {
			return CommandSemantic{IsError: true, Message: "npm command failed"}
		}
		return CommandSemantic{IsError: false}
	},

	// git: 0=success, 1=error (most subcommands)
	"git": func(exitCode int, stdout, stderr string) CommandSemantic {
		if exitCode != 0 {
			return CommandSemantic{IsError: true, Message: "git command failed"}
		}
		return CommandSemantic{IsError: false}
	},

	// cargo: 0=success, 101=build error, 1=other errors
	"cargo": func(exitCode int, stdout, stderr string) CommandSemantic {
		if exitCode == 101 {
			return CommandSemantic{IsError: true, Message: "Cargo build failed"}
		}
		if exitCode != 0 {
			return CommandSemantic{IsError: true, Message: "Cargo command failed"}
		}
		return CommandSemantic{IsError: false}
	},

	// go: 0=success, 1=error
	"go": func(exitCode int, stdout, stderr string) CommandSemantic {
		if exitCode != 0 {
			return CommandSemantic{IsError: true, Message: "Go command failed"}
		}
		return CommandSemantic{IsError: false}
	},

	// make: 0=success, 1=error, 2=internal error
	"make": func(exitCode int, stdout, stderr string) CommandSemantic {
		if exitCode == 2 {
			return CommandSemantic{IsError: true, Message: "Make internal error"}
		}
		if exitCode != 0 {
			return CommandSemantic{IsError: true, Message: "Make failed"}
		}
		return CommandSemantic{IsError: false}
	},

	// docker: 0=success, non-zero=error
	"docker": func(exitCode int, stdout, stderr string) CommandSemantic {
		if exitCode != 0 {
			return CommandSemantic{IsError: true, Message: "Docker command failed"}
		}
		return CommandSemantic{IsError: false}
	},

	// curl: 0=success, non-zero=error
	"curl": func(exitCode int, stdout, stderr string) CommandSemantic {
		if exitCode != 0 {
			return CommandSemantic{IsError: true, Message: "curl request failed"}
		}
		return CommandSemantic{IsError: false}
	},

	// wget: 0=success, non-zero=error
	"wget": func(exitCode int, stdout, stderr string) CommandSemantic {
		if exitCode != 0 {
			return CommandSemantic{IsError: true, Message: "wget request failed"}
		}
		return CommandSemantic{IsError: false}
	},

	// pytest: 0=tests passed, 1=tests failed, 2=error
	"pytest": func(exitCode int, stdout, stderr string) CommandSemantic {
		if exitCode >= 2 {
			return CommandSemantic{IsError: true, Message: "pytest error"}
		}
		if exitCode == 1 {
			return CommandSemantic{IsError: true, Message: "pytest: tests failed"}
		}
		return CommandSemantic{IsError: false, Message: "All tests passed"}
	},

	// jest: 0=success, 1=test failure
	"jest": func(exitCode int, stdout, stderr string) CommandSemantic {
		if exitCode != 0 {
			return CommandSemantic{IsError: true, Message: "Jest: tests failed"}
		}
		return CommandSemantic{IsError: false, Message: "All tests passed"}
	},

	// eslint: 0=success, 1=linting errors
	"eslint": func(exitCode int, stdout, stderr string) CommandSemantic {
		if exitCode != 0 {
			return CommandSemantic{IsError: true, Message: "ESLint: linting errors found"}
		}
		return CommandSemantic{IsError: false}
	},
}

// extractBaseCommand extracts the first word from a command
func extractBaseCommand(command string) string {
	command = strings.TrimSpace(command)
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

// heuristicallyExtractBaseCommand extracts the command that determines exit code
// Takes the last command in a pipeline (what determines exit code)
func heuristicallyExtractBaseCommand(command string) string {
	parts := splitCommandForSemantics(command)
	if len(parts) == 0 {
		return extractBaseCommand(command)
	}
	// Take the last command as that's what determines the exit code
	return extractBaseCommand(parts[len(parts)-1])
}

// splitCommandForSemantics splits a command into segments by operators
func splitCommandForSemantics(command string) []string {
	var segments []string
	var current strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	for i := 0; i < len(command); i++ {
		c := rune(command[i])

		if escaped {
			current.WriteRune(c)
			escaped = false
			continue
		}

		if c == '\\' && !inSingleQuote {
			escaped = true
			current.WriteRune(c)
			continue
		}

		if c == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			current.WriteRune(c)
			continue
		}

		if c == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			current.WriteRune(c)
			continue
		}

		if !inSingleQuote && !inDoubleQuote {
			// Check for operators
			if i+1 < len(command) {
				twoChar := command[i : i+2]
				if twoChar == "&&" || twoChar == "||" || twoChar == ">>" || twoChar == ">&" {
					if current.Len() > 0 {
						segments = append(segments, strings.TrimSpace(current.String()))
						current.Reset()
					}
					segments = append(segments, twoChar)
					i++
					continue
				}
			}
			if c == '|' || c == '>' || c == '<' || c == ';' {
				if current.Len() > 0 {
					segments = append(segments, strings.TrimSpace(current.String()))
					current.Reset()
				}
				segments = append(segments, string(c))
				continue
			}
		}

		current.WriteRune(c)
	}

	if current.Len() > 0 {
		segments = append(segments, strings.TrimSpace(current.String()))
	}

	return segments
}

// getCommandSemantic returns the semantic interpreter for a command
func getCommandSemantic(command string) CommandSemanticFunc {
	baseCmd := heuristicallyExtractBaseCommand(command)
	if fn, ok := commandSemantics[baseCmd]; ok {
		return fn
	}
	return defaultSemantic
}

// InterpretCommandResult interprets command execution result based on semantic rules
func InterpretCommandResult(command string, exitCode int, stdout, stderr string) CommandSemantic {
	semantic := getCommandSemantic(command)
	return semantic(exitCode, stdout, stderr)
}

// Itoa converts int to string (exported for use by other packages)
func Itoa(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if negative {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

// Common patterns for command classification
var (
	// gitCommands lists common git subcommands
	gitCommands = map[string]bool{
		"status": true, "log": true, "diff": true, "show": true,
		"branch": true, "checkout": true, "commit": true, "push": true,
		"pull": true, "fetch": true, "clone": true, "init": true,
		"add": true, "reset": true, "rebase": true, "merge": true,
		"stash": true, "tag": true, "describe": true, "rev-parse": true,
	}

	// buildCommands lists common build tools
	buildCommands = map[string]bool{
		"build": true, "test": true, "run": true, "clean": true,
		"install": true, "publish": true, "fmt": true, "vet": true,
		"lint": true, "check": true,
	}
)

// GetCommandCategory returns a category for analytics
func GetCommandCategory(command string) string {
	base := extractBaseCommand(command)
	if base == "" {
		return "other"
	}

	// Check for compound commands like "git commit"
	parts := strings.Fields(command)
	if len(parts) >= 2 {
		subCmd := parts[1]
		if base == "git" && gitCommands[subCmd] {
			return base + "_" + subCmd
		}
		if base == "go" && buildCommands[subCmd] {
			return base + "_" + subCmd
		}
		if base == "cargo" && buildCommands[subCmd] {
			return base + "_" + subCmd
		}
		if base == "npm" {
			return base + "_" + subCmd
		}
		if base == "make" {
			return base + "_" + subCmd
		}
	}

	return base
}

// IsBackgroundCandidate checks if a command is commonly run in background
func IsBackgroundCandidate(command string) bool {
	bgCommands := map[string]bool{
		"npm": true, "yarn": true, "pnpm": true,
		"node": true, "python": true, "python3": true,
		"go": true, "cargo": true, "make": true,
		"docker": true, "docker-compose": true,
		"terraform": true, "webpack": true, "vite": true,
		"jest": true, "pytest": true, "playwright": true,
		"curl": true, "wget": true,
		"serve": true, "watch": true, "dev": true,
		"build": true, "test": true,
	}

	base := extractBaseCommand(command)
	return bgCommands[base]
}

// FormatExitCodeMessage formats a user-friendly exit code message
func FormatExitCodeMessage(exitCode int, command string) string {
	base := extractBaseCommand(command)

	switch base {
	case "grep", "rg", "ag":
		switch exitCode {
		case 0:
			return "Matches found"
		case 1:
			return "No matches found"
		case 2:
			return "Syntax error or file not found"
		default:
			return "grep exited with code " + Itoa(exitCode)
		}
	case "find":
		switch exitCode {
		case 0:
			return "Success"
		case 1:
			return "Some directories inaccessible"
		default:
			return "find exited with code " + Itoa(exitCode)
		}
	case "diff":
		switch exitCode {
		case 0:
			return "Files are identical"
		case 1:
			return "Files differ"
		default:
			return "diff exited with code " + Itoa(exitCode)
		}
	case "git":
		if exitCode == 0 {
			return "Success"
		}
		if strings.Contains(command, "merge") {
			return "Merge conflict or failure"
		}
		if strings.Contains(command, "push") {
			return "Push failed"
		}
		if strings.Contains(command, "pull") {
			return "Pull failed"
		}
		return "git command failed"
	case "npm":
		if exitCode == 0 {
			return "Success"
		}
		if strings.Contains(command, "install") {
			return "npm install failed"
		}
		if strings.Contains(command, "test") {
			return "npm test failed"
		}
		return "npm command failed"
	}

	switch exitCode {
	case 0:
		return "Success"
	case 1:
		return "Command failed"
	case 2:
		return "Misuse of shell command"
	case 126:
		return "Permission denied or command not executable"
	case 127:
		return "Command not found"
	case 128:
		return "Invalid exit argument"
	default:
		if exitCode > 128 {
			signal := exitCode - 128
			signals := map[int]string{
				1:  "Hangup",
				2:  "Interrupt (Ctrl+C)",
				3:  "Quit (Ctrl+\\)",
				4:  "Illegal instruction",
				6:  "Abort",
				9:  "Killed",
				11: "Segmentation fault",
				13: "Broken pipe",
				15: "Terminated",
			}
			if sig, ok := signals[signal]; ok {
				return "Killed by signal: " + sig
			}
			return "Killed by signal " + Itoa(signal)
		}
		return "Command exited with code " + Itoa(exitCode)
	}
}
