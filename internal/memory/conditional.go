package memory

import (
	"path/filepath"
	"strings"

	"claude-go/internal/types"
)

// ConditionalRuleMatcher matches conditional rules based on file paths.
type ConditionalRuleMatcher struct {
	originalCwd string
}

// ConditionalRuleMatcher creates a new conditional rule matcher.
func CreateConditionalRuleMatcher(originalCwd string) *ConditionalRuleMatcher {
	return &ConditionalRuleMatcher{originalCwd: originalCwd}
}

// MatchRule checks if a memory file's globs match the target path.
func (m *ConditionalRuleMatcher) MatchRule(rule types.MemoryFileInfo, targetPath string) bool {
	if len(rule.Globs) == 0 {
		return true // No globs = unconditional rule
	}

	for _, pattern := range rule.Globs {
		if m.matchGlob(pattern, targetPath, rule.Path) {
			return true
		}
	}

	return false
}

// matchGlob checks if a single glob pattern matches the target path.
func (m *ConditionalRuleMatcher) matchGlob(pattern, targetPath, rulePath string) bool {
	// For Project rules: glob patterns are relative to the directory containing .claude
	// For Managed/User rules: glob patterns are relative to the original CWD

	var baseDir string
	if filepath.IsAbs(pattern) {
		// Absolute pattern - match directly
		return matchGlobPattern(pattern, targetPath)
	}

	// Determine base directory
	if strings.Contains(rulePath, ".claude"+string(filepath.Separator)+"rules") {
		// Project rule - base is parent of .claude
		claudeIdx := strings.Index(rulePath, string(filepath.Separator)+".claude"+string(filepath.Separator))
		if claudeIdx > 0 {
			baseDir = rulePath[:claudeIdx]
		} else {
			baseDir = m.originalCwd
		}
	} else {
		// Managed/User rule - base is original CWD
		baseDir = m.originalCwd
	}

	// Make target path relative to base
	var relativePath string
	if filepath.IsAbs(targetPath) {
		rel, err := filepath.Rel(baseDir, targetPath)
		if err != nil {
			return false
		}
		relativePath = rel
	} else {
		relativePath = targetPath
	}

	// Normalize path separators
	relativePath = filepath.ToSlash(relativePath)
	pattern = filepath.ToSlash(pattern)

	// Check if path escapes base (starts with ..) or is absolute
	if strings.HasPrefix(relativePath, "..") || filepath.IsAbs(relativePath) {
		return false
	}

	return matchGlobPattern(pattern, relativePath)
}

// matchGlobPattern matches a glob pattern against a path using ignore-style matching.
func matchGlobPattern(pattern, path string) bool {
	// Handle ** patterns (match any depth)
	if strings.Contains(pattern, "**") {
		return matchDoubleStar(pattern, path)
	}

	// Handle * patterns (match single directory level)
	return matchSingleStar(pattern, path)
}

// matchDoubleStar handles ** patterns that match any depth.
func matchDoubleStar(pattern, path string) bool {
	// Convert ** pattern to a prefix/suffix match
	parts := strings.Split(pattern, "**")
	if len(parts) == 1 {
		// No ** in pattern
		return matchSingleStar(pattern, path)
	}

	// Check prefix
	if parts[0] != "" {
		prefix := strings.TrimSuffix(parts[0], "/")
		if !strings.HasPrefix(path, prefix) && path != prefix {
			return false
		}
	}

	// Check suffix
	if len(parts) > 1 && parts[1] != "" {
		suffix := strings.TrimPrefix(parts[1], "/")
		if !strings.HasSuffix(path, suffix) && !strings.HasSuffix(path+"/"+suffix, suffix) {
			return false
		}
	}

	return true
}

// matchSingleStar handles * patterns that match within a single directory level.
func matchSingleStar(pattern, path string) bool {
	// Simple glob implementation
	// Convert pattern to regex-like matching

	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")

	return matchParts(patternParts, pathParts, 0, 0)
}

func matchParts(patternParts, pathParts []string, pi, ppi int) bool {
	for pi < len(patternParts) {
		if ppi >= len(pathParts) {
			return false
		}

		pp := patternParts[pi]
		ppp := pathParts[ppi]

		if pp == "*" {
			// Match any single part
			pi++
			ppi++
			continue
		}

		if strings.Contains(pp, "*") {
			// Partial wildcard in part
			if !matchPartWithWildcard(pp, ppp) {
				return false
			}
			pi++
			ppi++
			continue
		}

		// Exact match
		if pp != ppp {
			return false
		}
		pi++
		ppi++
	}

	return ppi == len(pathParts)
}

func matchPartWithWildcard(pattern, part string) bool {
	// Convert pattern with * to a simple match
	starIdx := strings.Index(pattern, "*")
	if starIdx == -1 {
		return pattern == part
	}

	// Check prefix
	if !strings.HasPrefix(part, pattern[:starIdx]) {
		return false
	}

	// Check suffix
	suffix := pattern[starIdx+1:]
	if suffix == "" {
		return true
	}
	return strings.HasSuffix(part, suffix)
}

// FilterConditionalRules filters memory files to only include those that match the target path.
func FilterConditionalRules(files []types.MemoryFileInfo, targetPath string, matcher *ConditionalRuleMatcher) []types.MemoryFileInfo {
	var result []types.MemoryFileInfo
	for _, f := range files {
		if len(f.Globs) == 0 {
			// Unconditional rule - include
			result = append(result, f)
			continue
		}

		if matcher.MatchRule(f, targetPath) {
			result = append(result, f)
		}
	}
	return result
}

// GetConditionalRulesForPath gets conditional rules that match a specific path.
func GetConditionalRulesForPath(rules []types.MemoryFileInfo, targetPath string, matcher *ConditionalRuleMatcher) []types.MemoryFileInfo {
	var result []types.MemoryFileInfo
	for _, rule := range rules {
		if len(rule.Globs) > 0 && matcher.MatchRule(rule, targetPath) {
			result = append(result, rule)
		}
	}
	return result
}

// GetUnconditionalRules gets rules without path restrictions.
func GetUnconditionalRules(rules []types.MemoryFileInfo) []types.MemoryFileInfo {
	var result []types.MemoryFileInfo
	for _, rule := range rules {
		if len(rule.Globs) == 0 {
			result = append(result, rule)
		}
	}
	return result
}
