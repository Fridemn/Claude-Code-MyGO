package edit

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"claude-code-go/internal/tool/bash"
	"claude-code-go/internal/utils"
)

// SedEditInfo contains parsed sed edit command information
type SedEditInfo struct {
	FilePath       string
	Pattern        string
	Replacement    string
	Flags          string
	ExtendedRegex  bool
}

// sedDelimiters matches common sed delimiter characters
var sedDelimiters = regexp.MustCompile(`^s(.)`)

// ParseSedEditCommand parses a sed in-place edit command
// Returns nil if the command is not a valid sed in-place edit
func ParseSedEditCommand(command string) *SedEditInfo {
	trimmed := strings.TrimSpace(command)

	// Must start with sed
	if !strings.HasPrefix(trimmed, "sed ") {
		return nil
	}

	withoutSed := strings.TrimPrefix(trimmed, "sed ")
	withoutSed = strings.TrimSpace(withoutSed)

	// Parse the sed command
	var hasInPlaceFlag bool
	var extendedRegex bool
	var expression string
	var filePath string

	// Split into tokens (simple parser)
	tokens := tokenizeSedArgs(withoutSed)
	if tokens == nil {
		return nil
	}

	i := 0
	for i < len(tokens) {
		arg := tokens[i]

		// Handle -i flag (with or without backup suffix)
		if arg == "-i" || arg == "--in-place" {
			hasInPlaceFlag = true
			i++
			// On macOS, -i requires a suffix argument (even if empty string)
			if i < len(tokens) {
				nextArg := tokens[i]
				// If next arg is empty string or starts with dot, it's a backup suffix
				if strings.HasPrefix(nextArg, ".") && !strings.HasPrefix(nextArg, "-") {
					i++ // Skip the backup suffix
				}
			}
			continue
		}

		// Handle -i.bak or similar (inline suffix)
		if strings.HasPrefix(arg, "-i") && len(arg) > 2 {
			hasInPlaceFlag = true
			i++
			continue
		}

		// Handle extended regex flags
		if arg == "-E" || arg == "-r" || arg == "--regexp-extended" {
			extendedRegex = true
			i++
			continue
		}

		// Handle -e flag with expression
		if arg == "-e" || arg == "--expression" {
			if i+1 < len(tokens) {
				if expression != "" {
					return nil // Only support single expression
				}
				expression = tokens[i+1]
				i += 2
				continue
			}
			return nil
		}

		if strings.HasPrefix(arg, "--expression=") {
			if expression != "" {
				return nil
			}
			expression = strings.TrimPrefix(arg, "--expression=")
			i++
			continue
		}

		// Skip other flags we don't understand
		if strings.HasPrefix(arg, "-") {
			i++
			continue
		}

		// Non-flag argument
		if expression == "" {
			expression = arg
		} else if filePath == "" {
			filePath = arg
		} else {
			// More than one file - not supported
			return nil
		}

		i++
	}

	// Must have -i flag, expression, and file path
	if !hasInPlaceFlag || expression == "" || filePath == "" {
		return nil
	}

	// Parse the substitution expression: s/pattern/replacement/flags
	// Try common delimiters
	delimiters := []rune{'/', '@', '#', '|', ':', ';'}
	var parsed *SedEditInfo

	for _, delim := range delimiters {
		pattern := "s" + string(delim)
		if strings.HasPrefix(expression, pattern) {
			rest := strings.TrimPrefix(expression, pattern)
			info := parseSubstitution(rest, delim, filePath, extendedRegex)
			if info != nil {
				parsed = info
				break
			}
		}
	}

	return parsed
}

// parseSubstitution parses the pattern/replacement/flags part of sed command
func parseSubstitution(rest string, delim rune, filePath string, extendedRegex bool) *SedEditInfo {
	var pattern, replacement, flags strings.Builder
	state := 0 // 0=pattern, 1=replacement, 2=flags
	escaped := false

	for i := 0; i < len(rest); i++ {
		c := rune(rest[i])

		if escaped {
			switch state {
			case 0:
				pattern.WriteRune(c)
			case 1:
				replacement.WriteRune(c)
			case 2:
				flags.WriteRune(c)
			}
			escaped = false
			continue
		}

		if c == '\\' {
			escaped = true
			switch state {
			case 0:
				pattern.WriteRune(c)
			case 1:
				replacement.WriteRune(c)
			case 2:
				flags.WriteRune(c)
			}
			continue
		}

		if c == delim {
			state++
			if state > 2 {
				return nil // Extra delimiter
			}
			continue
		}

		switch state {
		case 0:
			pattern.WriteRune(c)
		case 1:
			replacement.WriteRune(c)
		case 2:
			flags.WriteRune(c)
		}
	}

	// Must have found all parts (pattern, replacement delimiter, and optional flags)
	if state != 2 {
		return nil
	}

	// Validate flags - only allow safe substitution flags
	validFlags := regexp.MustCompile(`^[gpimIG]*$`)
	if !validFlags.MatchString(flags.String()) {
		return nil
	}

	return &SedEditInfo{
		FilePath:      filePath,
		Pattern:       pattern.String(),
		Replacement:   replacement.String(),
		Flags:         flags.String(),
		ExtendedRegex: extendedRegex,
	}
}

// tokenizeSedArgs splits sed arguments into tokens
func tokenizeSedArgs(input string) []string {
	var tokens []string
	var current strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	for i := 0; i < len(input); i++ {
		c := rune(input[i])

		if escaped {
			current.WriteRune(c)
			escaped = false
			continue
		}

		if c == '\\' {
			current.WriteRune(c)
			escaped = true
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

		if !inSingleQuote && !inDoubleQuote && c == ' ' {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			continue
		}

		current.WriteRune(c)
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// IsSedInPlaceEdit checks if a command is a sed in-place edit
func IsSedInPlaceEdit(command string) bool {
	return ParseSedEditCommand(command) != nil
}

// ApplySedSubstitution applies a sed substitution to file content
func ApplySedSubstitution(content string, info *SedEditInfo) string {
	if info == nil {
		return content
	}

	// Convert sed pattern to Go regex
	jsPattern := info.Pattern

	// In non-extended mode, metacharacters have opposite escaping
	if !info.ExtendedRegex {
		// BRE to ERE conversion placeholders
		backslashPH := generatePlaceholder("BACKSLASH")
		plusPH := generatePlaceholder("PLUS")
		questionPH := generatePlaceholder("QUESTION")
		pipePH := generatePlaceholder("PIPE")
		lparenPH := generatePlaceholder("LPAREN")
		rparenPH := generatePlaceholder("RPAREN")

		// Step 1: Protect literal backslashes
		jsPattern = strings.ReplaceAll(jsPattern, "\\\\", backslashPH)
		// Step 2: Replace escaped metacharacters with placeholders
		jsPattern = strings.ReplaceAll(jsPattern, "\\+", plusPH)
		jsPattern = strings.ReplaceAll(jsPattern, "\\?", questionPH)
		jsPattern = strings.ReplaceAll(jsPattern, "\\|", pipePH)
		jsPattern = strings.ReplaceAll(jsPattern, "\\(", lparenPH)
		jsPattern = strings.ReplaceAll(jsPattern, "\\)", rparenPH)
		// Step 3: Escape unescaped metacharacters
		jsPattern = strings.ReplaceAll(jsPattern, "+", "\\+")
		jsPattern = strings.ReplaceAll(jsPattern, "?", "\\?")
		jsPattern = strings.ReplaceAll(jsPattern, "|", "\\|")
		jsPattern = strings.ReplaceAll(jsPattern, "(", "\\(")
		jsPattern = strings.ReplaceAll(jsPattern, ")", "\\)")
		// Step 4: Replace placeholders with JS equivalents
		jsPattern = strings.ReplaceAll(jsPattern, backslashPH, "\\\\")
		jsPattern = strings.ReplaceAll(jsPattern, plusPH, "+")
		jsPattern = strings.ReplaceAll(jsPattern, questionPH, "?")
		jsPattern = strings.ReplaceAll(jsPattern, pipePH, "|")
		jsPattern = strings.ReplaceAll(jsPattern, lparenPH, "(")
		jsPattern = strings.ReplaceAll(jsPattern, rparenPH, ")")
	}

	// Unescape \/ to /
	jsPattern = strings.ReplaceAll(jsPattern, "\\/", "/")

	// Build replacement string
	jsReplacement := info.Replacement
	// Unescape \/ to /
	jsReplacement = strings.ReplaceAll(jsReplacement, "\\/", "/")
	// Convert & to $& (full match)
	jsReplacement = strings.ReplaceAll(jsReplacement, "\\&", "&SPECIAL_AMP&")
	jsReplacement = strings.ReplaceAll(jsReplacement, "&", "$&")
	jsReplacement = strings.ReplaceAll(jsReplacement, "&SPECIAL_AMP&", "&")
	// Convert \n to newline
	jsReplacement = strings.ReplaceAll(jsReplacement, "\\n", "\n")
	// Convert \t to tab
	jsReplacement = strings.ReplaceAll(jsReplacement, "\\t", "\t")

	// Build regex flags
	var regexFlag bytes.Buffer
	if strings.Contains(info.Flags, "i") || strings.Contains(info.Flags, "I") {
		regexFlag.WriteString("i")
	}

	// Compile regex
	re, err := regexp.Compile("(?:" + jsPattern + ")")
	if err != nil {
		return content
	}

	// Apply replacement
	result := re.ReplaceAllString(content, jsReplacement)

	return result
}

// generatePlaceholder creates a unique placeholder string
func generatePlaceholder(name string) string {
	b := make([]byte, 4)
	rand.Read(b)
	return "\x00" + name + hex.EncodeToString(b) + "\x00"
}

// ApplySedToFile applies sed substitution to a file and writes back
// Preserves encoding and line endings
func ApplySedToFile(filePath string, info *SedEditInfo) error {
	// Detect encoding and line endings
	encoding := utils.DetectFileEncoding(filePath)
	lineEndings := utils.DetectLineEndings(filePath, encoding)

	// Read file content with detected encoding
	content, encoding, lineEndings, err := utils.ReadFileWithMetadata(filePath)
	if err != nil {
		return err
	}

	// Apply substitution
	newContent := ApplySedSubstitution(content, info)

	// Write back with preserved encoding and line endings
	return utils.WriteTextContent(filePath, newContent, encoding, lineEndings)
}

// SimulatedSedEditResult represents the result of a simulated sed edit
type SimulatedSedEditResult struct {
	Stdout     string
	Stderr     string
	Interrupted bool
	Error      error
}

// ApplySimulatedSedEdit applies a sed edit directly without running sed
// Preserves file encoding and line endings
func ApplySimulatedSedEdit(filePath string, newContent string) *SimulatedSedEditResult {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return &SimulatedSedEditResult{
			Stderr: "sed: " + filePath + ": No such file or directory",
			Error:  err,
		}
	}

	// Check if file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return &SimulatedSedEditResult{
			Stderr: "sed: " + filePath + ": No such file or directory",
		}
	}

	// Detect encoding and line endings from original file
	encoding := utils.DetectFileEncoding(absPath)
	lineEndings := utils.DetectLineEndings(absPath, encoding)

	// Write new content with preserved encoding and line endings
	if err := utils.WriteTextContent(absPath, newContent, encoding, lineEndings); err != nil {
		return &SimulatedSedEditResult{
			Stderr: "sed: " + filePath + ": " + err.Error(),
			Error:  err,
		}
	}

	return &SimulatedSedEditResult{
		Stdout: "",
		Stderr: "",
	}
}

// BuildSedEditPreview generates a preview of what a sed edit would do
func BuildSedEditPreview(filePath string, sedInfo *SedEditInfo) (original, modified string, err error) {
	content, _, _, err := utils.ReadFileWithMetadata(filePath)
	if err != nil {
		return "", "", err
	}

	original = content
	modified = ApplySedSubstitution(original, sedInfo)
	return original, modified, nil
}

// CountSedMatches counts how many matches a sed pattern would make
func CountSedMatches(filePath string, sedInfo *SedEditInfo) (int, error) {
	content, _, _, err := utils.ReadFileWithMetadata(filePath)
	if err != nil {
		return 0, err
	}

	// Build the same pattern as ApplySedSubstitution
	pattern := sedInfo.Pattern
	if !sedInfo.ExtendedRegex {
		// Simple BRE to ERE conversion
		pattern = strings.ReplaceAll(pattern, "\\(", "(")
		pattern = strings.ReplaceAll(pattern, "\\)", ")")
		pattern = strings.ReplaceAll(pattern, "\\+", "+")
		pattern = strings.ReplaceAll(pattern, "\\?", "?")
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return 0, err
	}

	matches := re.FindAllStringIndex(content, -1)
	return len(matches), nil
}

// EchoCommandResult provides a summary for echo-like commands
func EchoCommandResult(stdout string) bash.CommandSemantic {
	// echo typically always succeeds
	return bash.CommandSemantic{
		IsError: false,
		Message: strings.TrimSpace(stdout),
	}
}

// CopyWithProgress copies a file with progress tracking
func CopyWithProgress(src, dst string, onProgress func(written, total int64)) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	total, err := io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	if onProgress != nil {
		onProgress(total, total)
	}

	return nil
}
