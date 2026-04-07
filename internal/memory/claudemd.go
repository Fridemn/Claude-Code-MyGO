package memory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"claude-code-go/internal/types"
	"claude-code-go/internal/utils"
)

// ClaudeMdLoader handles discovery and loading of CLAUDE.md memory files.
type ClaudeMdLoader struct {
	originalCwd            string
	homeDir                string
	configHomeDir          string
	managedPath            string
	additionalDirs         []string
	userSettingsEnabled    bool
	projectSettingsEnabled bool
	localSettingsEnabled   bool
	autoMemEnabled         bool

	// Cache
	cachedFiles []types.MemoryFileInfo
	cacheMu     sync.RWMutex
}

// LoaderOptions contains options for creating a ClaudeMdLoader.
type LoaderOptions struct {
	OriginalCwd            string
	HomeDir                string
	ConfigHomeDir          string
	ManagedPath            string
	AdditionalDirs         []string
	UserSettingsEnabled    bool
	ProjectSettingsEnabled bool
	LocalSettingsEnabled   bool
	AutoMemEnabled         bool
}

// ClaudeMdLoader creates a new memory file loader.
func CreateClaudeMdLoader(opts LoaderOptions) *ClaudeMdLoader {
	if opts.HomeDir == "" {
		opts.HomeDir, _ = os.UserHomeDir()
	}
	if opts.ConfigHomeDir == "" {
		opts.ConfigHomeDir = filepath.Join(opts.HomeDir, ".claude")
	}
	return &ClaudeMdLoader{
		originalCwd:            opts.OriginalCwd,
		homeDir:                opts.HomeDir,
		configHomeDir:          opts.ConfigHomeDir,
		managedPath:            opts.ManagedPath,
		additionalDirs:         opts.AdditionalDirs,
		userSettingsEnabled:    opts.UserSettingsEnabled,
		projectSettingsEnabled: opts.ProjectSettingsEnabled,
		localSettingsEnabled:   opts.LocalSettingsEnabled,
		autoMemEnabled:         opts.AutoMemEnabled,
	}
}

// GetMemoryFiles returns all discovered memory files.
// Files are loaded in priority order (later = higher priority).
func (l *ClaudeMdLoader) GetMemoryFiles(ctx context.Context) ([]types.MemoryFileInfo, error) {
	// Check cache
	l.cacheMu.RLock()
	if l.cachedFiles != nil {
		result := make([]types.MemoryFileInfo, len(l.cachedFiles))
		copy(result, l.cachedFiles)
		l.cacheMu.RUnlock()
		return result, nil
	}
	l.cacheMu.RUnlock()

	var result []types.MemoryFileInfo
	processedPaths := make(map[string]bool)

	// 1. Managed memory (global policy)
	if l.managedPath != "" {
		files, _ := l.processMemoryFile(ctx, l.managedPath, types.MemoryTypeManaged, processedPaths, false)
		result = append(result, files...)
	}

	// 2. Managed rules
	if l.managedPath != "" {
		rulesDir := filepath.Join(filepath.Dir(l.managedPath), ".claude", "rules")
		files, _ := l.processMdRules(ctx, rulesDir, types.MemoryTypeManaged, processedPaths, false, false)
		result = append(result, files...)
	}

	// 3. User memory
	if l.userSettingsEnabled {
		userPath := filepath.Join(l.configHomeDir, "CLAUDE.md")
		files, _ := l.processMemoryFile(ctx, userPath, types.MemoryTypeUser, processedPaths, true)
		result = append(result, files...)
	}

	// 4. User rules
	if l.userSettingsEnabled {
		userRulesDir := filepath.Join(l.configHomeDir, "rules")
		files, _ := l.processMdRules(ctx, userRulesDir, types.MemoryTypeUser, processedPaths, true, false)
		result = append(result, files...)
	}

	// 5. Project and Local files (from CWD upward to root)
	dirs := l.getDirectoriesToProcess()
	for _, dir := range dirs {
		// Project CLAUDE.md
		if l.projectSettingsEnabled {
			projectPath := filepath.Join(dir, "CLAUDE.md")
			files, _ := l.processMemoryFile(ctx, projectPath, types.MemoryTypeProject, processedPaths, false)
			result = append(result, files...)

			// .claude/CLAUDE.md
			dotClaudePath := filepath.Join(dir, ".claude", "CLAUDE.md")
			files, _ = l.processMemoryFile(ctx, dotClaudePath, types.MemoryTypeProject, processedPaths, false)
			result = append(result, files...)

			// .claude/rules/*.md (unconditional)
			rulesDir := filepath.Join(dir, ".claude", "rules")
			files, _ = l.processMdRules(ctx, rulesDir, types.MemoryTypeProject, processedPaths, false, false)
			result = append(result, files...)
		}

		// Local CLAUDE.local.md
		if l.localSettingsEnabled {
			localPath := filepath.Join(dir, "CLAUDE.local.md")
			files, _ := l.processMemoryFile(ctx, localPath, types.MemoryTypeLocal, processedPaths, false)
			result = append(result, files...)
		}
	}

	// 6. Additional directories (--add-dir)
	for _, dir := range l.additionalDirs {
		if l.projectSettingsEnabled {
			projectPath := filepath.Join(dir, "CLAUDE.md")
			files, _ := l.processMemoryFile(ctx, projectPath, types.MemoryTypeProject, processedPaths, false)
			result = append(result, files...)

			dotClaudePath := filepath.Join(dir, ".claude", "CLAUDE.md")
			files, _ = l.processMemoryFile(ctx, dotClaudePath, types.MemoryTypeProject, processedPaths, false)
			result = append(result, files...)

			rulesDir := filepath.Join(dir, ".claude", "rules")
			files, _ = l.processMdRules(ctx, rulesDir, types.MemoryTypeProject, processedPaths, false, false)
			result = append(result, files...)
		}
	}

	// 7. AutoMem entrypoint (MEMORY.md)
	if l.autoMemEnabled {
		autoMemPath := l.GetAutoMemPath()
		if autoMemPath != "" {
			entryPath := filepath.Join(autoMemPath, EntrypointName)
			if info, _ := l.safelyReadMemoryFile(ctx, entryPath, types.MemoryTypeAutoMem); info != nil {
				normalizedPath := normalizePathForComparison(info.Path)
				if !processedPaths[normalizedPath] {
					processedPaths[normalizedPath] = true
					result = append(result, *info)
				}
			}
		}
	}

	// Cache result
	l.cacheMu.Lock()
	l.cachedFiles = make([]types.MemoryFileInfo, len(result))
	copy(l.cachedFiles, result)
	l.cacheMu.Unlock()

	return result, nil
}

// GetClaudeMds formats memory files as a string for injection into prompts.
func GetClaudeMds(files []types.MemoryFileInfo, filter func(types.MemoryType) bool) string {
	var memories []string

	for _, file := range files {
		if filter != nil && !filter(file.Type) {
			continue
		}
		if file.Content == "" {
			continue
		}

		description := getTypeDescription(file.Type)
		content := strings.TrimSpace(file.Content)

		if content != "" {
			memories = append(memories, formatMemoryContent(file.Path, description, content))
		}
	}

	if len(memories) == 0 {
		return ""
	}

	return MemoryInstructionPrompt + "\n\n" + strings.Join(memories, "\n\n")
}

func getTypeDescription(t types.MemoryType) string {
	switch t {
	case types.MemoryTypeProject:
		return " (project instructions, checked into the codebase)"
	case types.MemoryTypeLocal:
		return " (user's private project instructions, not checked in)"
	case types.MemoryTypeTeamMem:
		return " (shared team memory, synced across the organization)"
	case types.MemoryTypeAutoMem:
		return " (user's auto-memory, persists across conversations)"
	default:
		return " (user's private global instructions for all projects)"
	}
}

func formatMemoryContent(path, description, content string) string {
	return "Contents of " + path + description + ":\n\n" + content
}

// getDirectoriesToProcess returns directories from root to CWD.
func (l *ClaudeMdLoader) getDirectoriesToProcess() []string {
	var dirs []string
	currentDir := l.originalCwd

	for {
		dirs = append([]string{currentDir}, dirs...)
		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			break
		}
		currentDir = parent
	}

	return dirs
}

// processMemoryFile reads and processes a single memory file.
func (l *ClaudeMdLoader) processMemoryFile(ctx context.Context, filePath string, memType types.MemoryType, processedPaths map[string]bool, includeExternal bool) ([]types.MemoryFileInfo, error) {
	return l.processMemoryFileRecursive(ctx, filePath, memType, processedPaths, includeExternal, 0, "")
}

func (l *ClaudeMdLoader) processMemoryFileRecursive(ctx context.Context, filePath string, memType types.MemoryType, processedPaths map[string]bool, includeExternal bool, depth int, parent string) ([]types.MemoryFileInfo, error) {
	// Check max depth
	if depth >= MaxIncludeDepth {
		return nil, nil
	}

	// Check already processed
	normalizedPath := normalizePathForComparison(filePath)
	if processedPaths[normalizedPath] {
		return nil, nil
	}

	// Mark as processed
	processedPaths[normalizedPath] = true

	// Read file
	info, err := l.safelyReadMemoryFile(ctx, filePath, memType)
	if err != nil || info == nil {
		return nil, err
	}

	if strings.TrimSpace(info.Content) == "" {
		return nil, nil
	}

	if parent != "" {
		info.Parent = parent
	}

	var result []types.MemoryFileInfo
	result = append(result, *info)

	// Process @include directives
	for _, includePath := range info.Globs {
		// TODO: Resolve include paths relative to the file
		_ = includePath
	}

	return result, nil
}

// processMdRules processes all .md files in a rules directory.
func (l *ClaudeMdLoader) processMdRules(ctx context.Context, rulesDir string, memType types.MemoryType, processedPaths map[string]bool, includeExternal bool, conditionalOnly bool) ([]types.MemoryFileInfo, error) {
	var result []types.MemoryFileInfo

	entries, err := os.ReadDir(rulesDir)
	if err != nil {
		return nil, nil // Directory doesn't exist
	}

	for _, entry := range entries {
		fullPath := filepath.Join(rulesDir, entry.Name())

		if entry.IsDir() {
			// Recurse into subdirectories
			files, _ := l.processMdRules(ctx, fullPath, memType, processedPaths, includeExternal, conditionalOnly)
			result = append(result, files...)
		} else if strings.HasSuffix(entry.Name(), ".md") {
			files, _ := l.processMemoryFile(ctx, fullPath, memType, processedPaths, includeExternal)
			for _, f := range files {
				// Filter based on conditional rules
				if conditionalOnly {
					if len(f.Globs) == 0 {
						continue
					}
				} else {
					if len(f.Globs) > 0 {
						continue
					}
				}
				result = append(result, f)
			}
		}
	}

	return result, nil
}

// safelyReadMemoryFile reads a memory file and handles errors gracefully.
func (l *ClaudeMdLoader) safelyReadMemoryFile(ctx context.Context, filePath string, memType types.MemoryType) (*types.MemoryFileInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	info, _ := ParseMemoryFileContent(string(content), filePath, memType)
	return info, nil
}

// GetAutoMemPath returns the path to the auto-memory directory.
func (l *ClaudeMdLoader) GetAutoMemPath() string {
	// ~/.claude/projects/<project-hash>/memory/
	projectHash := getProjectHash(l.originalCwd)
	return filepath.Join(l.configHomeDir, "projects", projectHash, "memory")
}

// ClearCache clears the memory file cache.
func (l *ClaudeMdLoader) ClearCache() {
	l.cacheMu.Lock()
	l.cachedFiles = nil
	l.cacheMu.Unlock()
}

// normalizePathForComparison normalizes a path for deduplication.
func normalizePathForComparison(path string) string {
	// Lowercase for case-insensitive comparison (Windows)
	path = strings.ToLower(path)
	// Convert backslashes to forward slashes
	path = strings.ReplaceAll(path, "\\", "/")
	return path
}

// getProjectHash generates a hash for the project path.
func getProjectHash(cwd string) string {
	// Use djb2-based sanitization matching TS implementation
	return utils.SanitizePath(cwd)
}
