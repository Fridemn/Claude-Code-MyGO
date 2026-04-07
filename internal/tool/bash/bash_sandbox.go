package bash


import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// SandboxConfig configures sandbox behavior
type SandboxConfig struct {
	Enabled              bool
	AllowedDirs          []string          // Directories the command can access
	AllowedPaths         []string          // Specific path patterns allowed
	DeniedPaths          []string          // Path patterns to deny
	NetworkAllowed       bool              // Whether network access is allowed
	AllowedNetworkHosts  []string          // Allowed network hosts
	MaxMemoryMB          int               // Max memory in MB
	MaxCPUSeconds        int               // Max CPU time
	Timeout              time.Duration      // Execution timeout
	EnvironmentWhitelist []string          // Allowed env vars
}

// SandboxManager manages sandbox execution for bash commands
type SandboxManager struct {
	mu      sync.RWMutex
	enabled bool
	config  SandboxConfig

	// Statistics
	stats struct {
		TotalExecutions int
		AllowedCount    int
		DeniedCount     int
		Violations      int
	}
}

// SandboxResult represents the result of sandboxed execution
type SandboxResult struct {
	Allowed   bool
	Output    string
	Error     error
	Violation *SandboxViolation
	Duration  time.Duration
}

// SandboxViolation represents a sandbox security violation
type SandboxViolation struct {
	Type    ViolationType
	Message string
	Path    string
	Command string
}

// ViolationType classifies the type of violation
type ViolationType int

const (
	ViolationNone ViolationType = iota
	ViolationPathAccess
	ViolationNetworkAccess
	ViolationMemoryLimit
	ViolationCPULimit
	ViolationTimeout
)

// DefaultSandboxConfig returns the default sandbox configuration
func DefaultSandboxConfig() SandboxConfig {
	return SandboxConfig{
		Enabled:     true,
		AllowedDirs: []string{".", "/tmp", "/var/tmp"}, // Default allowed directories
		DeniedPaths: []string{
			"/etc/shadow",
			"/etc/sudoers",
			"/.ssh/",
			"/.aws/",
			"/.gcp/",
			"/.docker/",
		},
		NetworkAllowed:      false,
		AllowedNetworkHosts: []string{},
		MaxMemoryMB:         512,
		MaxCPUSeconds:       60,
		Timeout:             2 * time.Minute,
		EnvironmentWhitelist: []string{
			"PATH", "HOME", "USER", "SHELL", "TERM",
			"LANG", "LC_ALL", "PWD", "OLDPWD",
		},
	}
}

// SandboxManager creates a new sandbox manager
func CreateSandboxManager() *SandboxManager {
	return &SandboxManager{
		enabled: true,
		config:  DefaultSandboxConfig(),
	}
}

// IsEnabled returns whether sandboxing is enabled
func (sm *SandboxManager) IsEnabled() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.enabled
}

// SetEnabled enables or disables sandboxing
func (sm *SandboxManager) SetEnabled(enabled bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.enabled = enabled
}

// GetConfig returns the current sandbox configuration
func (sm *SandboxManager) GetConfig() SandboxConfig {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.config
}

// SetConfig updates the sandbox configuration
func (sm *SandboxManager) SetConfig(config SandboxConfig) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.config = config
}

// SetAllowedDirs sets directories that commands can access
func (sm *SandboxManager) SetAllowedDirs(dirs []string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.config.AllowedDirs = dirs
}

// SetDeniedPaths sets paths that are always denied
func (sm *SandboxManager) SetDeniedPaths(paths []string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.config.DeniedPaths = paths
}

// Stats returns sandbox execution statistics
func (sm *SandboxManager) Stats() (total, allowed, denied, violations int) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.stats.TotalExecutions, sm.stats.AllowedCount,
		sm.stats.DeniedCount, sm.stats.Violations
}

// Execute runs a command in the sandbox
func (sm *SandboxManager) Execute(ctx context.Context, command, cwd string) SandboxResult {
	sm.mu.Lock()
	sm.stats.TotalExecutions++
	sm.mu.Unlock()

	start := time.Now()

	// Check if sandboxing is enabled
	if !sm.IsEnabled() {
		return SandboxResult{
			Allowed:  true,
			Duration: time.Since(start),
		}
	}

	// Check path permissions
	if violation := sm.checkPathAccess(command, cwd); violation != nil {
		sm.mu.Lock()
		sm.stats.DeniedCount++
		sm.stats.Violations++
		sm.mu.Unlock()
		return SandboxResult{
			Allowed:   false,
			Violation:  violation,
			Duration:   time.Since(start),
		}
	}

	// Check for dangerous commands
	if violation := sm.checkDangerousCommand(command); violation != nil {
		sm.mu.Lock()
		sm.stats.DeniedCount++
		sm.stats.Violations++
		sm.mu.Unlock()
		return SandboxResult{
			Allowed:   false,
			Violation:  violation,
			Duration:   time.Since(start),
		}
	}

	// Execute with restrictions
	output, err := sm.executeRestricted(ctx, command, cwd)
	duration := time.Since(start)

	if err != nil {
		// Check if it's a timeout
		if strings.Contains(err.Error(), "context deadline exceeded") {
			sm.mu.Lock()
			sm.stats.Violations++
			sm.mu.Unlock()
			return SandboxResult{
				Allowed: false,
				Output:  output,
				Error:   fmt.Errorf("command timed out after %v", duration),
				Violation: &SandboxViolation{
					Type:    ViolationTimeout,
					Message: fmt.Sprintf("command timed out after %v", duration),
					Command: command,
				},
				Duration: duration,
			}
		}
	}

	sm.mu.Lock()
	sm.stats.AllowedCount++
	sm.mu.Unlock()

	return SandboxResult{
		Allowed:  true,
		Output:   output,
		Error:    err,
		Duration: duration,
	}
}

// checkPathAccess checks if the command accesses allowed paths
func (sm *SandboxManager) checkPathAccess(command, cwd string) *SandboxViolation {
	sm.mu.RLock()
	config := sm.config
	sm.mu.RUnlock()

	// Extract paths from the command
	paths := extractPathsFromCommand(command)

	for _, path := range paths {
		// Check denied paths first
		for _, denied := range config.DeniedPaths {
			if matchesPathPattern(path, denied) {
				return &SandboxViolation{
					Type:    ViolationPathAccess,
					Message: fmt.Sprintf("access denied to path: %s", path),
					Path:    path,
					Command: command,
				}
			}
		}

		// Check if path is within allowed directories
		if !sm.isPathAllowed(path, cwd) {
			// For paths starting with /, check against allowed dirs
			if strings.HasPrefix(path, "/") {
				allowed := false
				for _, dir := range config.AllowedDirs {
					allowedDir := dir
					if dir == "." {
						allowedDir, _ = os.Getwd()
					}
					if strings.HasPrefix(path, allowedDir) {
						allowed = true
						break
					}
				}
				if !allowed {
					return &SandboxViolation{
						Type:    ViolationPathAccess,
						Message: fmt.Sprintf("path not in allowed directories: %s", path),
						Path:    path,
						Command: command,
					}
				}
			}
		}
	}

	return nil
}

// isPathAllowed checks if a path is within allowed directories
func (sm *SandboxManager) isPathAllowed(path, cwd string) bool {
	sm.mu.RLock()
	config := sm.config
	sm.mu.RUnlock()

	// Resolve the path
	var absPath string
	if filepath.IsAbs(path) {
		absPath = path
	} else {
		absPath = filepath.Join(cwd, path)
	}
	absPath = filepath.Clean(absPath)

	for _, allowed := range config.AllowedDirs {
		var allowedAbs string
		if filepath.IsAbs(allowed) {
			allowedAbs = allowed
		} else if allowed == "." {
			allowedAbs, _ = os.Getwd()
		} else {
			allowedAbs = filepath.Join(cwd, allowed)
		}
		allowedAbs = filepath.Clean(allowedAbs)

		if strings.HasPrefix(absPath, allowedAbs) {
			return true
		}
	}

	return false
}

// checkDangerousCommand checks for dangerous command patterns
func (sm *SandboxManager) checkDangerousCommand(command string) *SandboxViolation {
	dangerousPatterns := []struct {
		pattern string
		message string
	}{
		{`rm\s+-rf\s+/\s*`, "recursive delete from root denied"},
		{`dd\s+.*=/dev/(sd|hd|nvme)`, "direct disk access denied"},
		{`mkfs`, "filesystem creation denied"},
		{`fdisk`, "disk partitioning denied"},
		{`parted`, "disk partitioning denied"},
		{`chattr\s+-[ia]\s+`, "immutable attribute modification denied"},
		{`>`, "output redirection denied"},
		{`&\s*>\s*`, "output redirection denied"},
	}

	for _, dp := range dangerousPatterns {
		if strings.Contains(command, dp.pattern) || regexMatch(dp.pattern, command) {
			return &SandboxViolation{
				Type:    ViolationPathAccess,
				Message: dp.message,
				Command: command,
			}
		}
	}

	return nil
}

// regexMatch performs a simple regex match (for complex patterns)
func regexMatch(pattern, text string) bool {
	// Simplified - just check if pattern is contained
	return strings.Contains(text, pattern)
}

// executeRestricted executes the command with restrictions
func (sm *SandboxManager) executeRestricted(ctx context.Context, command, cwd string) (string, error) {
	sm.mu.RLock()
	config := sm.config
	sm.mu.RUnlock()

	// Create command
	cmd := exec.CommandContext(ctx, "bash", "-lc", command)
	cmd.Dir = cwd

	// Set up environment with whitelist
	env := []string{}
	for _, e := range os.Environ() {
		key := strings.SplitN(e, "=", 2)[0]
		if contains(config.EnvironmentWhitelist, key) {
			env = append(env, e)
		}
	}
	cmd.Env = env

	// Set process limits
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// Note: Ptrace setting requires CAP_SYS_PTRACE
		// On most systems, this won't work without privileges
		// This is a defense-in-depth measure, not a hard security boundary
	}

	// Execute
	output, err := cmd.CombinedOutput()

	return string(output), err
}

// extractPathsFromCommand extracts file paths from a command
func extractPathsFromCommand(command string) []string {
	var paths []string

	// Simple extraction - look for paths
	words := strings.Fields(command)
	for _, word := range words {
		// Skip options
		if strings.HasPrefix(word, "-") {
			continue
		}

		// Check if it looks like a path
		if strings.HasPrefix(word, "/") ||
			strings.HasPrefix(word, "./") ||
			strings.HasPrefix(word, "../") ||
			strings.HasPrefix(word, "~") {
			// Remove trailing punctuation
			path := strings.TrimRight(word, ",;)|")
			if path != "" {
				paths = append(paths, path)
			}
		}
	}

	return paths
}

// matchesPathPattern checks if a path matches a pattern
func matchesPathPattern(path, pattern string) bool {
	// Handle trailing wildcard
	if strings.HasSuffix(pattern, "/") {
		pattern = pattern[:len(pattern)-1]
		return strings.HasPrefix(path, pattern)
	}

	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(path, prefix)
	}

	return path == pattern
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Global sandbox manager instance
var globalSandboxManager = CreateSandboxManager()

// GetSandboxManager returns the global sandbox manager
func GetCreateSandboxManager() *SandboxManager {
	return globalSandboxManager
}

// IsSandboxingEnabled returns whether sandboxing is globally enabled
func IsSandboxingEnabled() bool {
	return globalSandboxManager.IsEnabled()
}

// SetSandboxingEnabled enables or disables global sandboxing
func SetSandboxingEnabled(enabled bool) {
	globalSandboxManager.SetEnabled(enabled)
}

// ExecuteSandboxed executes a command using the global sandbox
func ExecuteSandboxed(ctx context.Context, command, cwd string) SandboxResult {
	return globalSandboxManager.Execute(ctx, command, cwd)
}

// SandboxResultToBashOutput converts SandboxResult to BashOutput
func SandboxResultToBashOutput(result SandboxResult) BashOutput {
	output := BashOutput{
		Interrupted: result.Violation != nil && result.Violation.Type == ViolationTimeout,
	}

	if result.Violation != nil {
		output.Stderr = result.Violation.Message
	} else if result.Error != nil {
		output.Stderr = result.Error.Error()
	}

	// Parse stdout from result.Output
	if strings.Contains(result.Output, "\n") {
		parts := strings.SplitN(result.Output, "\n", 2)
		output.Stdout = parts[0]
		if len(parts) > 1 {
			output.Stderr = parts[1]
		}
	} else {
		output.Stdout = result.Output
	}

	return output
}
