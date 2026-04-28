package search

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"claude-go/internal/tool"
)

// --- GrepTool ---
// GrepTool searches for regex patterns in file contents.
// Based on TS src/tools/GrepTool/GrepTool.ts

// GrepOutputMode defines the output format for grep results
type GrepOutputMode string

const (
	GrepModeContent          GrepOutputMode = "content"
	GrepModeFilesWithMatches GrepOutputMode = "files_with_matches"
	GrepModeCount            GrepOutputMode = "count"
)

// GrepInput represents the input parameters for GrepTool
type GrepInput struct {
	Pattern         string
	Path            string
	Glob            string
	Type            string
	OutputMode      GrepOutputMode
	ContextBefore   int
	ContextAfter    int
	Context         int
	ShowLineNumbers bool
	CaseInsensitive bool
	HeadLimit       int
	Offset          int
	Multiline       bool
}

// GrepOutput represents the output of GrepTool
type GrepOutput struct {
	Mode          GrepOutputMode `json:"mode,omitempty"`
	NumFiles      int            `json:"numFiles"`
	Filenames     []string       `json:"filenames"`
	Content       string         `json:"content,omitempty"`
	NumLines      int            `json:"numLines,omitempty"`
	NumMatches    int            `json:"numMatches,omitempty"`
	AppliedLimit  int            `json:"appliedLimit,omitempty"`
	AppliedOffset int            `json:"appliedOffset,omitempty"`
}

type GrepTool struct{}

func (GrepTool) Name() string { return "Grep" }

func (GrepTool) Description() string {
	return `A powerful search tool built on regex

Usage:
- ALWAYS use Grep for search tasks. NEVER invoke ` + "`grep` or `rg`" + ` as a Bash command.
- Supports full regex syntax (e.g., "log.*Error", "function\\s+\\w+")
- Filter files with glob parameter (e.g., "*.js", "**/*.tsx") or type parameter (e.g., "js", "py", "rust")
- Output modes: "content" shows matching lines, "files_with_matches" shows only file paths (default), "count" shows match counts
- Pattern syntax: Uses regex - literal braces need escaping (use ` + "`interface\\{\\}`" + ` to find ` + "`interface{}`" + ` in Go code)
- Multiline matching: By default patterns match within single lines only. For cross-line patterns, use multiline: true`
}

func (GrepTool) IsReadOnly(tool.Input) bool { return true }

// IsSearchOrReadCommand indicates that Grep is always a collapsible search operation
func (GrepTool) IsSearchOrReadCommand(in tool.Input) tool.SearchOrReadResult {
	return tool.SearchOrReadResult{
		IsCollapsible: true,
		IsSearch:      true,
	}
}

func (GrepTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"pattern":     tool.SchemaString("The regular expression pattern to search for in file contents"),
		"path":        tool.SchemaString("File or directory to search in. Defaults to current working directory."),
		"glob":        tool.SchemaString("Glob pattern to filter files (e.g. \"*.js\", \"*.{ts,tsx}\")"),
		"output_mode": tool.SchemaEnumString("Output mode: \"content\" shows matching lines, \"files_with_matches\" shows file paths, \"count\" shows match counts. Defaults to \"files_with_matches\".", "content", "files_with_matches", "count"),
		"-B":          tool.SchemaInteger("Number of lines to show before each match. Requires output_mode: \"content\"."),
		"-A":          tool.SchemaInteger("Number of lines to show after each match. Requires output_mode: \"content\"."),
		"-C":          tool.SchemaInteger("Alias for context. Number of lines to show before and after each match."),
		"context":     tool.SchemaInteger("Number of lines to show before and after each match. Requires output_mode: \"content\"."),
		"-n":          tool.SchemaBoolean("Show line numbers in output. Defaults to true for content mode."),
		"-i":          tool.SchemaBoolean("Case insensitive search"),
		"type":        tool.SchemaString("File type to search (e.g. \"js\", \"py\", \"rust\", \"go\")"),
		"head_limit":  tool.SchemaInteger("Limit output to first N lines/entries. Defaults to 250. Pass 0 for unlimited."),
		"offset":      tool.SchemaInteger("Skip first N lines/entries before applying head_limit. Defaults to 0."),
		"multiline":   tool.SchemaBoolean("Enable multiline mode where . matches newlines. Default: false."),
	}, "pattern")
}

func (GrepTool) Call(ctx context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	// Extract input parameters
	pattern := tool.GetString(in, "pattern")
	if strings.TrimSpace(pattern) == "" {
		return tool.Result{}, fmt.Errorf("pattern is required")
	}

	path := tool.GetString(in, "path")
	globPattern := tool.GetString(in, "glob")
	fileType := tool.GetString(in, "type")

	// Get output mode
	outputMode := GrepModeFilesWithMatches
	if mode, ok := in["output_mode"].(string); ok {
		switch mode {
		case "content":
			outputMode = GrepModeContent
		case "files_with_matches":
			outputMode = GrepModeFilesWithMatches
		case "count":
			outputMode = GrepModeCount
		}
	}

	// Context settings
	contextBefore := tool.GetInt(in, "-B", 0)
	contextAfter := tool.GetInt(in, "-A", 0)
	contextLines := tool.GetInt(in, "-C", 0)
	if contextOverride := tool.GetInt(in, "context", 0); contextOverride > 0 {
		contextLines = contextOverride
	}
	if contextLines > 0 {
		contextBefore = contextLines
		contextAfter = contextLines
	}

	// Other settings
	showLineNumbers := true
	if v, ok := in["-n"].(bool); ok {
		showLineNumbers = v
	}
	caseInsensitive := tool.GetBool(in, "-i")
	multiline := tool.GetBool(in, "multiline")
	headLimit := tool.GetInt(in, "head_limit", 250)
	offset := tool.GetInt(in, "offset", 0)

	// Get the search path
	searchPath := path
	if strings.TrimSpace(searchPath) == "" {
		if runtime.Store != nil {
			searchPath = runtime.Store.GetCWD()
		} else {
			searchPath = "."
		}
	}
	searchPath = cleanToolPath(searchPath)

	// Build the regex pattern
	regexFlags := ""
	if caseInsensitive {
		regexFlags = "(?i)"
	}
	regexPattern := regexFlags + pattern

	var re *regexp.Regexp
	var err error
	if multiline {
		re, err = regexp.Compile(regexFlags + "(?s)" + pattern)
	} else {
		re, err = regexp.Compile(regexPattern)
	}
	if err != nil {
		return tool.Result{}, fmt.Errorf("invalid regex pattern: %w", err)
	}

	// Convert type to glob pattern
	if fileType != "" {
		typeGlobs := map[string]string{
			"go":   "*.go",
			"js":   "*.js",
			"ts":   "*.ts",
			"tsx":  "*.tsx",
			"py":   "*.py",
			"rust": "*.rs",
			"java": "*.java",
			"c":    "*.c",
			"cpp":  "*.cpp",
			"h":    "*.h",
			"hpp":  "*.hpp",
		}
		if g, ok := typeGlobs[fileType]; ok {
			if globPattern != "" {
				globPattern = g + "," + globPattern
			} else {
				globPattern = g
			}
		}
	}

	// Walk the directory and search
	var matches []string
	var fileMatches []string
	var countResults []string

	err = walkAndSearch(ctx, searchPath, globPattern, re, outputMode, showLineNumbers,
		contextBefore, contextAfter, multiline, &matches, &fileMatches, &countResults)

	if err != nil && err != context.Canceled {
		return tool.Result{}, err
	}

	// Apply head_limit and offset
	var output GrepOutput

	switch outputMode {
	case GrepModeContent:
		limitedMatches := applyHeadLimitStrings(matches, headLimit, offset)
		output = GrepOutput{
			Mode:      GrepModeContent,
			Content:   strings.Join(limitedMatches.items, "\n"),
			NumLines:  len(limitedMatches.items),
			NumFiles:  0,
			Filenames: []string{},
		}
		if limitedMatches.applied {
			output.AppliedLimit = headLimit
		}
		if offset > 0 {
			output.AppliedOffset = offset
		}

	case GrepModeCount:
		limitedCount := applyHeadLimitStrings(countResults, headLimit, offset)
		totalMatches := 0
		fileCount := 0
		for _, line := range limitedCount.items {
			colonIdx := strings.LastIndex(line, ":")
			if colonIdx > 0 {
				countStr := line[colonIdx+1:]
				count := parseInt(countStr)
				totalMatches += count
				fileCount++
			}
		}
		output = GrepOutput{
			Mode:       GrepModeCount,
			Content:    strings.Join(limitedCount.items, "\n"),
			NumMatches: totalMatches,
			NumFiles:   fileCount,
			Filenames:  []string{},
		}
		if limitedCount.applied {
			output.AppliedLimit = headLimit
		}
		if offset > 0 {
			output.AppliedOffset = offset
		}

	case GrepModeFilesWithMatches:
		// Sort by modification time (most recent first)
		sortFilesByModTime(fileMatches, searchPath)
		limitedFiles := applyHeadLimitStrings(fileMatches, headLimit, offset)
		output = GrepOutput{
			Mode:      GrepModeFilesWithMatches,
			Filenames: limitedFiles.items,
			NumFiles:  len(limitedFiles.items),
		}
		if limitedFiles.applied {
			output.AppliedLimit = headLimit
		}
		if offset > 0 {
			output.AppliedOffset = offset
		}
	}

	// Format output content for Result
	var resultContent string
	if output.NumFiles == 0 && len(output.Filenames) == 0 && output.Content == "" {
		resultContent = "No files found"
	} else {
		resultContent = formatGrepOutput(output)
	}

	return tool.Result{Content: resultContent, Meta: map[string]any{
		"mode":          output.Mode,
		"numFiles":      output.NumFiles,
		"filenames":     output.Filenames,
		"content":       output.Content,
		"numLines":      output.NumLines,
		"numMatches":    output.NumMatches,
		"appliedLimit":  output.AppliedLimit,
		"appliedOffset": output.AppliedOffset,
	}}, nil
}

// walkAndSearch walks a directory and searches for pattern matches
func walkAndSearch(ctx context.Context, root string, globPattern string, re *regexp.Regexp,
	outputMode GrepOutputMode, showLineNumbers bool, contextBefore, contextAfter int,
	multiline bool, matches, fileMatches, countResults *[]string) error {

	// Parse glob patterns
	var globMatchers []globMatcher
	if globPattern != "" {
		for _, p := range parseGlobPatterns(globPattern) {
			globMatchers = append(globMatchers, createGlobMatcher(p))
		}
	}

	// VCS directories to exclude
	vcsDirs := []string{".git", ".svn", ".hg", ".bzr", ".jj", ".sl"}

	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip VCS directories
		if d.IsDir() {
			for _, vcs := range vcsDirs {
				if d.Name() == vcs {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Check glob pattern
		if len(globMatchers) > 0 {
			matched := false
			for _, m := range globMatchers {
				if m.Match(path) {
					matched = true
					break
				}
			}
			if !matched {
				return nil
			}
		}

		// Open and read the file
		f, err := os.Open(path)
		if err != nil {
			return nil // Skip unreadable files
		}
		defer f.Close()

		// Make path relative
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			relPath = path
		}

		scanner := bufio.NewScanner(f)
		lineNo := 0
		var lines []string
		hasMatch := false
		matchCount := 0

		// Read all lines for multiline matching or context
		if multiline || contextBefore > 0 || contextAfter > 0 {
			for scanner.Scan() {
				lines = append(lines, scanner.Text())
			}
		}

		if multiline {
			// Multiline matching
			fullContent := strings.Join(lines, "\n")
			for _, matchIdx := range re.FindAllIndex([]byte(fullContent), -1) {
				hasMatch = true
				matchCount++
				if outputMode == GrepModeContent {
					startLine := findLineIndex(fullContent, matchIdx[0])
					endLine := findLineIndex(fullContent, matchIdx[1])
					if showLineNumbers {
						*matches = append(*matches, fmt.Sprintf("%s:%d-%d:%s", relPath, startLine+1, endLine+1, fullContent[matchIdx[0]:matchIdx[1]]))
					} else {
						*matches = append(*matches, fullContent[matchIdx[0]:matchIdx[1]])
					}
				}
			}
		} else {
			// Line-by-line matching
			if len(lines) > 0 {
				// Already read all lines
				for i, line := range lines {
					lineNo = i + 1
					if re.MatchString(line) {
						hasMatch = true
						matchCount++
						if outputMode == GrepModeContent {
							*matches = append(*matches, formatContentLine(relPath, lineNo, line, showLineNumbers, lines, contextBefore, contextAfter, i))
						}
					}
				}
			} else {
				// Read line by line
				for scanner.Scan() {
					lineNo++
					line := scanner.Text()
					if re.MatchString(line) {
						hasMatch = true
						matchCount++
						if outputMode == GrepModeContent {
							*matches = append(*matches, formatLine(relPath, lineNo, line, showLineNumbers))
						}
					}
				}
			}
		}

		if hasMatch {
			*fileMatches = append(*fileMatches, relPath)
			if outputMode == GrepModeCount {
				*countResults = append(*countResults, fmt.Sprintf("%s:%d", relPath, matchCount))
			}
		}

		return nil
	})
}

func formatLine(path string, lineNo int, line string, showLineNumbers bool) string {
	if showLineNumbers {
		return fmt.Sprintf("%s:%d:%s", path, lineNo, line)
	}
	return fmt.Sprintf("%s:%s", path, line)
}

func formatContentLine(path string, lineNo int, line string, showLineNumbers bool, lines []string, contextBefore, contextAfter, currentIdx int) string {
	// For context, we return the matched line only (context handling is complex)
	return formatLine(path, lineNo, line, showLineNumbers)
}

func findLineIndex(content string, byteIndex int) int {
	count := 0
	for i, c := range content {
		if i >= byteIndex {
			return count
		}
		if c == '\n' {
			count++
		}
	}
	return count
}

func parseInt(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}

type headLimitResult struct {
	items   []string
	applied bool
}

func applyHeadLimitStrings(items []string, limit, offset int) headLimitResult {
	if limit == 0 {
		return headLimitResult{items: items[offset:], applied: false}
	}
	if len(items)-offset > limit {
		return headLimitResult{items: items[offset : offset+limit], applied: true}
	}
	return headLimitResult{items: items[offset:], applied: false}
}

func sortFilesByModTime(files []string, basePath string) {
	// Get file stats and sort by modification time
	sort.Slice(files, func(i, j int) bool {
		fullPathI := filepath.Join(basePath, files[i])
		fullPathJ := filepath.Join(basePath, files[j])
		infoI, errI := os.Stat(fullPathI)
		infoJ, errJ := os.Stat(fullPathJ)
		if errI != nil || errJ != nil {
			return files[i] < files[j] // Sort by name on error
		}
		return infoI.ModTime().After(infoJ.ModTime())
	})
}

func formatGrepOutput(output GrepOutput) string {
	switch output.Mode {
	case GrepModeContent:
		var result strings.Builder
		result.WriteString(output.Content)
		if output.AppliedLimit > 0 || output.AppliedOffset > 0 {
			result.WriteString("\n\n[Showing results with pagination = ")
			if output.AppliedLimit > 0 {
				result.WriteString(fmt.Sprintf("limit: %d", output.AppliedLimit))
			}
			if output.AppliedOffset > 0 {
				if output.AppliedLimit > 0 {
					result.WriteString(", ")
				}
				result.WriteString(fmt.Sprintf("offset: %d", output.AppliedOffset))
			}
			result.WriteString("]")
		}
		return result.String()

	case GrepModeCount:
		var result strings.Builder
		result.WriteString(output.Content)
		result.WriteString("\n\nFound ")
		result.WriteString(fmt.Sprintf("%d", output.NumMatches))
		if output.NumMatches == 1 {
			result.WriteString(" occurrence across ")
		} else {
			result.WriteString(" occurrences across ")
		}
		result.WriteString(fmt.Sprintf("%d", output.NumFiles))
		if output.NumFiles == 1 {
			result.WriteString(" file.")
		} else {
			result.WriteString(" files.")
		}
		if output.AppliedLimit > 0 || output.AppliedOffset > 0 {
			result.WriteString(" with pagination = ")
			if output.AppliedLimit > 0 {
				result.WriteString(fmt.Sprintf("limit: %d", output.AppliedLimit))
			}
			if output.AppliedOffset > 0 {
				if output.AppliedLimit > 0 {
					result.WriteString(", ")
				}
				result.WriteString(fmt.Sprintf("offset: %d", output.AppliedOffset))
			}
		}
		return result.String()

	case GrepModeFilesWithMatches:
		if len(output.Filenames) == 0 {
			return "No files found"
		}
		var result strings.Builder
		result.WriteString(fmt.Sprintf("Found %d files", output.NumFiles))
		if output.AppliedLimit > 0 || output.AppliedOffset > 0 {
			result.WriteString(" with pagination = ")
			if output.AppliedLimit > 0 {
				result.WriteString(fmt.Sprintf("limit: %d", output.AppliedLimit))
			}
			if output.AppliedOffset > 0 {
				if output.AppliedLimit > 0 {
					result.WriteString(", ")
				}
				result.WriteString(fmt.Sprintf("offset: %d", output.AppliedOffset))
			}
		}
		result.WriteString("\n")
		result.WriteString(strings.Join(output.Filenames, "\n"))
		return result.String()

	default:
		return "No files found"
	}
}

// --- GlobTool ---
// GlobTool finds files by name pattern.
// Based on TS src/tools/GlobTool/GlobTool.ts

type GlobTool struct{}

func (GlobTool) Name() string { return "Glob" }

func (GlobTool) Description() string {
	return `- Fast file pattern matching tool that works with any codebase size
- Supports glob patterns like "**/*.js" or "src/**/*.ts"
- Returns matching file paths sorted by modification time
- Use this tool when you need to find files by name patterns
- When you are doing an open ended search that may require multiple rounds of globbing and grepping, use the Agent tool instead`
}

func (GlobTool) IsReadOnly(tool.Input) bool { return true }

// IsSearchOrReadCommand indicates that Glob is always a collapsible search operation
func (GlobTool) IsSearchOrReadCommand(in tool.Input) tool.SearchOrReadResult {
	return tool.SearchOrReadResult{
		IsCollapsible: true,
		IsSearch:      true,
	}
}

func (GlobTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"pattern": tool.SchemaString("The glob pattern to match files against"),
		"path":    tool.SchemaString("The directory to search in. Defaults to current working directory."),
	}, "pattern")
}

// GlobOutput represents the output of the Glob tool
type GlobOutput struct {
	Filenames  []string `json:"filenames"`
	NumFiles   int      `json:"numFiles"`
	DurationMs int64    `json:"durationMs"`
	Truncated  bool     `json:"truncated"`
}

func (GlobTool) Call(ctx context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	pattern := strings.TrimSpace(tool.GetString(in, "pattern"))
	path := tool.GetString(in, "path")

	if pattern == "" {
		return tool.Result{}, fmt.Errorf("pattern is required")
	}

	start := time.Now()

	// Handle absolute, exact file paths directly to avoid false negatives when
	// the caller passes an absolute path as pattern (e.g. "/tmp/a.json").
	if filepath.IsAbs(pattern) && !isGlobPattern(pattern) {
		files, err := resolveExactGlobPath(pattern)
		if err != nil {
			return tool.Result{}, err
		}
		if len(files) == 0 {
			return tool.Result{Content: "No files found"}, nil
		}
		return buildGlobResult(files, start, false), nil
	}

	// Use current working directory if path not specified
	searchPath := path
	if strings.TrimSpace(searchPath) == "" {
		if runtime.Store != nil {
			searchPath = runtime.Store.GetCWD()
		} else {
			searchPath = "."
		}
	}
	searchPath = cleanToolPath(searchPath)

	// If caller omitted path and used an absolute glob pattern (e.g.
	// "/tmp/*.json"), derive search root from the literal prefix so the pattern
	// can match instead of being interpreted relative to CWD.
	if strings.TrimSpace(path) == "" {
		if derivedRoot, relativePattern, ok := deriveSearchRootFromAbsolutePattern(pattern); ok {
			searchPath = derivedRoot
			pattern = relativePattern
		}
	}

	// Limit results to 100 by default (matches TS implementation)
	limit := 100

	var files []string
	var truncated bool

	// Create glob matcher
	matcher := createGlobMatcher(pattern)

	// VCS directories to exclude
	vcsDirs := []string{".git", ".svn", ".hg", ".bzr", ".jj", ".sl"}

	err := filepath.WalkDir(searchPath, func(filePath string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip VCS directories
		if d.IsDir() {
			for _, vcs := range vcsDirs {
				if d.Name() == vcs {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Match the pattern
		relPath, err := filepath.Rel(searchPath, filePath)
		if err != nil {
			relPath = filePath
		}

		if matcher.Match(relPath) || matcher.Match(filepath.Base(filePath)) {
			files = append(files, relPath)

			if len(files) >= limit {
				truncated = true
				return filepath.SkipAll
			}
		}

		return nil
	})

	if err != nil && err != context.Canceled && err != filepath.SkipAll {
		return tool.Result{}, err
	}

	// Sort by modification time (most recent first)
	sortFilesByModTime(files, searchPath)

	// If no files found, return a message
	if len(files) == 0 {
		return tool.Result{Content: "No files found"}, nil
	}
	return buildGlobResult(files, start, truncated), nil
}

// --- Glob Matcher Helpers ---
// Supports both simple patterns (*.go) and double-star patterns (**/*.go)

type globMatcher struct {
	pattern       string
	hasDoubleStar bool
	regex         *regexp.Regexp
}

func parseGlobPatterns(globStr string) []string {
	var patterns []string
	// Split by comma or space, preserving brace patterns
	rawPatterns := strings.Fields(globStr)
	for _, p := range rawPatterns {
		if strings.Contains(p, "{") && strings.Contains(p, "}") {
			patterns = append(patterns, p)
		} else {
			for _, sub := range strings.Split(p, ",") {
				if sub != "" {
					patterns = append(patterns, sub)
				}
			}
		}
	}
	return patterns
}

func createGlobMatcher(pattern string) globMatcher {
	hasDoubleStar := strings.Contains(pattern, "**")

	// Convert glob pattern to regex
	regexPattern := globToRegex(pattern)

	re := regexp.MustCompile(regexPattern)

	return globMatcher{
		pattern:       pattern,
		hasDoubleStar: hasDoubleStar,
		regex:         re,
	}
}

func (m globMatcher) Match(path string) bool {
	return m.regex.MatchString(path)
}

func globToRegex(pattern string) string {
	// Convert glob pattern to regex
	var result strings.Builder
	result.WriteString("^")

	i := 0
	for i < len(pattern) {
		c := pattern[i]
		switch c {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				// Double star - match any path
				result.WriteString(".*")
				i += 2
				// Skip following / if present
				if i < len(pattern) && pattern[i] == '/' {
					i++
				}
			} else {
				// Single star - match anything except /
				result.WriteString("[^/]*")
				i++
			}
		case '?':
			result.WriteString("[^/]")
			i++
		case '[':
			// Handle character class
			j := i + 1
			for j < len(pattern) && pattern[j] != ']' {
				j++
			}
			if j < len(pattern) {
				result.WriteString(pattern[i : j+1])
				i = j + 1
			} else {
				result.WriteString("\\[")
				i++
			}
		case '.', '^', '$', '+', '|', '(', ')', '{', '}':
			result.WriteString("\\")
			result.WriteByte(c)
			i++
		case '/':
			result.WriteString("/")
			i++
		default:
			result.WriteByte(c)
			i++
		}
	}

	result.WriteString("$")
	return result.String()
}

func isGlobPattern(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[")
}

func resolveExactGlobPath(pattern string) ([]string, error) {
	cleaned := cleanToolPath(pattern)
	info, err := os.Stat(cleaned)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if info.IsDir() {
		return nil, nil
	}
	return []string{cleaned}, nil
}

func deriveSearchRootFromAbsolutePattern(pattern string) (searchRoot, relativePattern string, ok bool) {
	if !filepath.IsAbs(pattern) || !isGlobPattern(pattern) {
		return "", "", false
	}

	firstGlobIdx := strings.IndexAny(pattern, "*?[")
	if firstGlobIdx <= 0 {
		return "", "", false
	}

	literalPrefix := pattern[:firstGlobIdx]
	root := filepath.Clean(filepath.Dir(literalPrefix))
	if root == "" {
		root = string(filepath.Separator)
	}

	rel, err := filepath.Rel(root, pattern)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", "", false
	}

	return root, filepath.ToSlash(rel), true
}

func buildGlobResult(files []string, start time.Time, truncated bool) tool.Result {
	output := GlobOutput{
		Filenames:  files,
		NumFiles:   len(files),
		DurationMs: time.Since(start).Milliseconds(),
		Truncated:  truncated,
	}

	var content strings.Builder
	for _, f := range files {
		content.WriteString(f)
		content.WriteString("\n")
	}
	if truncated {
		content.WriteString("(Results are truncated. Consider using a more specific path or pattern.)\n")
	}

	return tool.Result{
		Content: content.String(),
		Meta: map[string]any{
			"filenames":  output.Filenames,
			"numFiles":   output.NumFiles,
			"durationMs": output.DurationMs,
			"truncated":  output.Truncated,
		},
	}
}

// cleanToolPath cleans and normalizes a path
func cleanToolPath(path string) string {
	if path == "" {
		return "."
	}
	return filepath.Clean(path)
}

func RegisterSearchTools(r *tool.Registry) {
	r.Register(GrepTool{})
	r.Register(GlobTool{})
}
