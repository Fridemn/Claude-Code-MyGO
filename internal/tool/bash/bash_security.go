package bash


import (
	"fmt"
	"regexp"
	"strings"
)

// Security validation for shell commands.
// This module provides defense-in-depth security checks to prevent
// command injection and other shell-based attacks.

// SecurityLevel represents the security classification of a command
type SecurityLevel int

const (
	SecurityLevelSafe      SecurityLevel = iota // Safe to execute
	SecurityLevelWarning                        // Execute with warning
	SecurityLevelAsk                            // Ask user for confirmation
	SecurityLevelDeny                           // Deny execution
)

// SecurityResult represents the result of a security check
type SecurityResult struct {
	Level       SecurityLevel
	Message     string
	CheckID     string
	Suggestions []string
}

// Security check IDs for logging
const (
	CheckIDEmptyCommand           = "empty_command"
	CheckIDIncompleteCommand      = "incomplete_command"
	CheckIDCommandSubstitution    = "command_substitution"
	CheckIDProcessSubstitution    = "process_substitution"
	CheckIDParameterSubstitution  = "parameter_substitution"
	CheckIDDestructiveCommand     = "destructive_command"
	CheckIDUnbalancedQuotes       = "unbalanced_quotes"
	CheckIDSuspiciousPattern      = "suspicious_pattern"
	CheckIDMalformedTokens        = "malformed_tokens"
	CheckIDDangerousEnvVar        = "dangerous_env_var"
	CheckIDZshExtension           = "zsh_extension"
	CheckIDBacktickInjection      = "backtick_injection"
	CheckIDNewlineInjection       = "newline_injection"
	CheckIDUnicodeWhitespace      = "unicode_whitespace"
	CheckIDControlCharacters      = "control_characters"
)

// Dangerous command patterns
var (
	// Commands that can cause significant damage
	destructiveCommands = map[string]bool{
		"rm":      true,
		"rmdir":   true,
		"dd":      true,
		"shred":   true,
		"wipe":    true,
		"mkfs":    true,
		"fdisk":   true,
		"parted":  true,
		"format":  true,
		"erase":   true,
		"destroy": true,
	}

	// Commands that can modify system state
	systemCommands = map[string]bool{
		"shutdown":  true,
		"reboot":    true,
		"halt":      true,
		"poweroff":  true,
		"init":      true,
		"systemctl": true,
		"service":   true,
		"apt":       true,
		"yum":       true,
		"dnf":       true,
		"pacman":    true,
		"brew":      true,
		"pip":       true,
		"npm":       true,
		"gem":       true,
		"cargo":     true,
		"go":        true,
	}

	// Commands that access network
	networkCommands = map[string]bool{
		"curl":    true,
		"wget":    true,
		"nc":      true,
		"netcat":  true,
		"telnet":  true,
		"ssh":     true,
		"scp":     true,
		"rsync":   true,
		"ftp":     true,
		"sftp":    true,
	}

	// Safe environment variables that can be stripped
	safeEnvVars = map[string]bool{
		"PATH":    true,
		"HOME":    true,
		"USER":    true,
		"LANG":    true,
		"TERM":    true,
		"SHELL":   true,
		"PWD":     true,
		"OLDPWD":  true,
		"EDITOR":  true,
		"VISUAL":  true,
		"PAGER":   true,
		"LESS":    true,
		"MORE":    true,
		"DISPLAY": true,
	}

	// Zsh dangerous commands
	zshDangerousCommands = map[string]bool{
		"zmodload": true,
		"emulate":  true,
		"sysopen":  true,
		"sysread":  true,
		"syswrite": true,
		"sysseek":  true,
		"zpty":     true,
		"ztcp":     true,
		"zsocket":  true,
		"mapfile":  true,
		"zf_rm":    true,
		"zf_mv":    true,
		"zf_ln":    true,
		"zf_chmod": true,
		"zf_chown": true,
		"zf_mkdir": true,
		"zf_rmdir": true,
		"zf_chgrp": true,
	}

	// Dangerous substitution patterns
	dangerousSubstitutionPatterns = []*regexp.Regexp{
		regexp.MustCompile(`\$\(`),                   // Command substitution
		regexp.MustCompile(`<\(`),                   // Process substitution input
		regexp.MustCompile(`>\(`),                   // Process substitution output
		regexp.MustCompile(`=\(`),                   // Zsh process substitution
		regexp.MustCompile(`\$\{`),                  // Parameter substitution
		regexp.MustCompile(`\$\[`),                  // Legacy arithmetic expansion
		regexp.MustCompile(`~\[`),                   // Zsh parameter expansion
		regexp.MustCompile(`\(e:`),                  // Zsh glob qualifiers
		regexp.MustCompile(`\(\+`),                  // Zsh glob with execution
		regexp.MustCompile(`}\s*always\s*\{`),       // Zsh always block
		regexp.MustCompile(`<#`),                    // PowerShell comment (defense in depth)
	}
)

// SecurityValidator performs security validation on shell commands
type SecurityValidator struct {
	options SecurityValidatorOptions
}

// SecurityValidatorOptions configures the security validator
type SecurityValidatorOptions struct {
	AllowDestructive bool
	AllowNetwork     bool
	AllowSystem      bool
	StrictMode       bool
	ShellType        string // "bash", "zsh", "sh"
}

// CreateSecurityValidator creates a new security validator
func CreateSecurityValidator(options SecurityValidatorOptions) *SecurityValidator {
	return &SecurityValidator{options: options}
}

// Validate performs all security checks on a command
func (v *SecurityValidator) Validate(command string) SecurityResult {
	// Check empty command
	if strings.TrimSpace(command) == "" {
		return SecurityResult{
			Level:   SecurityLevelSafe,
			Message: "Empty command",
			CheckID: CheckIDEmptyCommand,
		}
	}

	// Run all checks in order of severity
	checks := []func(string) SecurityResult{
		v.checkIncompleteCommand,
		v.checkUnbalancedQuotes,
		v.checkMalformedTokens,
		v.checkDangerousSubstitutions,
		v.checkControlCharacters,
		v.checkDestructiveCommand,
		v.checkSystemCommand,
		v.checkNetworkCommand,
		v.checkZshExtensions,
		v.checkSuspiciousPatterns,
	}

	for _, check := range checks {
		result := check(command)
		if result.Level != SecurityLevelSafe {
			return result
		}
	}

	return SecurityResult{
		Level:   SecurityLevelSafe,
		Message: "Command passed security validation",
	}
}

// checkIncompleteCommand checks for incomplete command fragments
func (v *SecurityValidator) checkIncompleteCommand(command string) SecurityResult {
	trimmed := strings.TrimSpace(command)

	// Check for tab at start (incomplete fragment)
	if strings.HasPrefix(command, "\t") {
		return SecurityResult{
			Level:   SecurityLevelAsk,
			Message: "Command appears to be an incomplete fragment (starts with tab)",
			CheckID: CheckIDIncompleteCommand,
		}
	}

	// Check for flags at start
	if strings.HasPrefix(trimmed, "-") {
		return SecurityResult{
			Level:   SecurityLevelAsk,
			Message: "Command appears to be an incomplete fragment (starts with flags)",
			CheckID: CheckIDIncompleteCommand,
		}
	}

	// Check for operators at start
	for _, op := range []string{"&&", "||", ";", ">>", ">", "<"} {
		if strings.HasPrefix(trimmed, op) {
			return SecurityResult{
				Level:   SecurityLevelAsk,
				Message: "Command appears to be a continuation line (starts with operator)",
				CheckID: CheckIDIncompleteCommand,
			}
		}
	}

	return SecurityResult{Level: SecurityLevelSafe}
}

// checkUnbalancedQuotes checks for unbalanced quotes
func (v *SecurityValidator) checkUnbalancedQuotes(command string) SecurityResult {
	singleQuotes := 0
	doubleQuotes := 0
	escaped := false

	for _, char := range command {
		if escaped {
			escaped = false
			continue
		}

		if char == '\\' {
			escaped = true
			continue
		}

		if char == '\'' {
			singleQuotes++
		}
		if char == '"' {
			doubleQuotes++
		}
	}

	if singleQuotes%2 != 0 || doubleQuotes%2 != 0 {
		return SecurityResult{
			Level:   SecurityLevelAsk,
			Message: "Command has unbalanced quotes",
			CheckID: CheckIDUnbalancedQuotes,
			Suggestions: []string{
				"Check for missing closing quote",
				"Escape quotes if they should be literal",
			},
		}
	}

	return SecurityResult{Level: SecurityLevelSafe}
}

// checkMalformedTokens checks for malformed token patterns
func (v *SecurityValidator) checkMalformedTokens(command string) SecurityResult {
	// Check for unusual character sequences that might indicate injection attempts
	patterns := []struct {
		pattern *regexp.Regexp
		msg     string
	}{
		{regexp.MustCompile(`\$\s*\(`), "Suspicious $() spacing"},
		{regexp.MustCompile("`\\s*`"), "Suspicious backtick spacing"},
		{regexp.MustCompile(`;\s*;`), "Double semicolon"},
		{regexp.MustCompile(`\|\s*\|`), "Double pipe"},
	}

	for _, p := range patterns {
		if p.pattern.MatchString(command) {
			return SecurityResult{
				Level:   SecurityLevelWarning,
				Message: p.msg,
				CheckID: CheckIDMalformedTokens,
			}
		}
	}

	return SecurityResult{Level: SecurityLevelSafe}
}

// checkDangerousSubstitutions checks for dangerous substitution patterns
func (v *SecurityValidator) checkDangerousSubstitutions(command string) SecurityResult {
	for _, pattern := range dangerousSubstitutionPatterns {
		if pattern.MatchString(command) {
			return SecurityResult{
				Level:   SecurityLevelWarning,
				Message: "Command contains potentially dangerous substitution patterns",
				CheckID: CheckIDCommandSubstitution,
				Suggestions: []string{
					"Review the command for unintended side effects",
					"Consider using safer alternatives",
				},
			}
		}
	}

	return SecurityResult{Level: SecurityLevelSafe}
}

// checkControlCharacters checks for control characters
func (v *SecurityValidator) checkControlCharacters(command string) SecurityResult {
	for i, char := range command {
		if char < 32 && char != '\t' && char != '\n' && char != '\r' {
			return SecurityResult{
				Level:   SecurityLevelAsk,
				Message: fmt.Sprintf("Command contains control character at position %d", i),
				CheckID: CheckIDControlCharacters,
			}
		}
	}

	return SecurityResult{Level: SecurityLevelSafe}
}

// checkDestructiveCommand checks for destructive commands
func (v *SecurityValidator) checkDestructiveCommand(command string) SecurityResult {
	if v.options.AllowDestructive {
		return SecurityResult{Level: SecurityLevelSafe}
	}

	baseCmd := extractBaseCommand(command)
	if destructiveCommands[baseCmd] {
		return SecurityResult{
			Level:   SecurityLevelAsk,
			Message: fmt.Sprintf("Command '%s' is potentially destructive", baseCmd),
			CheckID: CheckIDDestructiveCommand,
			Suggestions: []string{
				"Review the command carefully before executing",
				"Ensure you have backups if modifying important files",
			},
		}
	}

	return SecurityResult{Level: SecurityLevelSafe}
}

// checkSystemCommand checks for system-modifying commands
func (v *SecurityValidator) checkSystemCommand(command string) SecurityResult {
	if v.options.AllowSystem {
		return SecurityResult{Level: SecurityLevelSafe}
	}

	baseCmd := extractBaseCommand(command)
	if systemCommands[baseCmd] {
		return SecurityResult{
			Level:   SecurityLevelWarning,
			Message: fmt.Sprintf("Command '%s' modifies system state", baseCmd),
			CheckID: CheckIDDestructiveCommand,
		}
	}

	return SecurityResult{Level: SecurityLevelSafe}
}

// checkNetworkCommand checks for network commands
func (v *SecurityValidator) checkNetworkCommand(command string) SecurityResult {
	if v.options.AllowNetwork {
		return SecurityResult{Level: SecurityLevelSafe}
	}

	baseCmd := extractBaseCommand(command)
	if networkCommands[baseCmd] {
		return SecurityResult{
			Level:   SecurityLevelWarning,
			Message: fmt.Sprintf("Command '%s' accesses the network", baseCmd),
			CheckID: CheckIDDestructiveCommand,
		}
	}

	return SecurityResult{Level: SecurityLevelSafe}
}

// checkZshExtensions checks for Zsh-specific dangerous commands
func (v *SecurityValidator) checkZshExtensions(command string) SecurityResult {
	if v.options.ShellType != "zsh" && v.options.ShellType != "" {
		return SecurityResult{Level: SecurityLevelSafe}
	}

	parts := strings.Fields(command)
	for _, part := range parts {
		if zshDangerousCommands[part] {
			return SecurityResult{
				Level:   SecurityLevelDeny,
				Message: fmt.Sprintf("Zsh command '%s' is blocked for security", part),
				CheckID: CheckIDZshExtension,
			}
		}
	}

	return SecurityResult{Level: SecurityLevelSafe}
}

// checkSuspiciousPatterns checks for other suspicious patterns
func (v *SecurityValidator) checkSuspiciousPatterns(command string) SecurityResult {
	// Check for backticks (command substitution)
	if strings.Contains(command, "`") {
		// Check if backticks are balanced
		count := strings.Count(command, "`")
		if count%2 != 0 {
			return SecurityResult{
				Level:   SecurityLevelAsk,
				Message: "Command has unbalanced backticks",
				CheckID: CheckIDBacktickInjection,
			}
		}

		// Check for nested backticks (dangerous)
		if nested := regexp.MustCompile("``.*``"); nested.MatchString(command) {
			return SecurityResult{
				Level:   SecurityLevelWarning,
				Message: "Command contains nested backticks",
				CheckID: CheckIDBacktickInjection,
			}
		}
	}

	// Check for newline in unusual places
	if strings.Contains(command, "\n") && !strings.Contains(command, "<<") {
		lines := strings.Split(command, "\n")
		if len(lines) > 1 && strings.TrimSpace(lines[0]) != "" {
			return SecurityResult{
				Level:   SecurityLevelWarning,
				Message: "Command contains embedded newlines",
				CheckID: CheckIDNewlineInjection,
			}
		}
	}

	// Check for unicode whitespace that could be hiding characters
	for _, char := range command {
		if char == 0x00A0 || char == 0x2000 || char == 0x2001 || char == 0x2002 ||
			char == 0x2003 || char == 0x2004 || char == 0x2005 || char == 0x2006 ||
			char == 0x2007 || char == 0x2008 || char == 0x2009 || char == 0x200A ||
			char == 0x2028 || char == 0x2029 || char == 0x202F || char == 0x205F ||
			char == 0x3000 {
			return SecurityResult{
				Level:   SecurityLevelAsk,
				Message: "Command contains unicode whitespace characters",
				CheckID: CheckIDUnicodeWhitespace,
			}
		}
	}

	return SecurityResult{Level: SecurityLevelSafe}
}

// Global default security validator
var defaultSecurityValidator = CreateSecurityValidator(SecurityValidatorOptions{
	ShellType: "bash",
})

// ValidateCommandSecurity validates a command using the default security settings
func ValidateCommandSecurity(command string) SecurityResult {
	return defaultSecurityValidator.Validate(command)
}

// IsCommandSafe returns true if a command passes all security checks
func IsCommandSafe(command string) bool {
	result := ValidateCommandSecurity(command)
	return result.Level == SecurityLevelSafe
}

// GetSecuritySuggestions returns suggestions for making a command safer
func GetSecuritySuggestions(command string) []string {
	result := ValidateCommandSecurity(command)
	return result.Suggestions
}
