package bash

import (
	"fmt"
	"regexp"
	"strings"
)

// Security validation for PowerShell commands

// PSSecurityLevel represents the security level of a PowerShell command
type PSSecurityLevel int

const (
	PSSecurityLevelAllow PSSecurityLevel = iota
	PSSecurityLevelWarning
	PSSecurityLevelAsk
	PSSecurityLevelDeny
)

// PSSecurityResult represents the result of PowerShell security validation
type PSSecurityResult struct {
	Level   PSSecurityLevel
	Message string
}

// validatePowerShellSecurity validates a PowerShell command for security issues
func validatePowerShellSecurity(command string) error {
	result := analyzePowerShellSecurity(command)
	switch result.Level {
	case PSSecurityLevelDeny:
		return fmt.Errorf("command blocked: %s", result.Message)
	case PSSecurityLevelAsk:
		return fmt.Errorf("security warning: %s", result.Message)
	default:
		return nil
	}
}

// analyzePowerShellSecurity analyzes a PowerShell command for security issues
func analyzePowerShellSecurity(command string) PSSecurityResult {
	// Check for empty command
	if strings.TrimSpace(command) == "" {
		return PSSecurityResult{Level: PSSecurityLevelAllow}
	}

	// Check for dangerous patterns
	dangerousPatterns := []struct {
		pattern *regexp.Regexp
		message string
		level   PSSecurityLevel
	}{
		// Invoke-Expression / iex - code execution
		{regexp.MustCompile(`(?i)(invoke-expression|iex)\s+`), "Invoke-Expression/iex can execute arbitrary code", PSSecurityLevelDeny},
		// Download cradles
		{regexp.MustCompile(`(?i)(invoke-webrequest|iwr|curl|wget)\s+.*\|\s*(invoke-expression|iex)`), "Download cradle pattern detected", PSSecurityLevelDeny},
		// Start-BitsTransfer - file download
		{regexp.MustCompile(`(?i)start-bitstransfer\s+`), "Start-BitsTransfer can download files", PSSecurityLevelAsk},
		// Encoded command
		{regexp.MustCompile(`(?i)-encodedcommand\s+`), "Encoded command obscures intent", PSSecurityLevelDeny},
		// Execution policy bypass
		{regexp.MustCompile(`(?i)set-executionpolicy\s+`), "Setting execution policy can weaken security", PSSecurityLevelDeny},
		// Registry modifications to security areas
		{regexp.MustCompile(`(?i)(hklm|hkey_local_machine)\\software\\microsoft\\windows`), "Modifying Windows security registry keys", PSSecurityLevelDeny},
		// Object with dangerous types
		{regexp.MustCompile(`(?i)new-object\s+.*scripting\.filesystemobject`), "New-Object with Scripting.FileSystemObject", PSSecurityLevelAsk},
		{regexp.MustCompile(`(?i)new-object\s+.*wscript\.shell`), "New-Object with WScript.Shell", PSSecurityLevelAsk},
		{regexp.MustCompile(`(?i)new-object\s+.*internetexplorer\.application`), "New-Object with InternetExplorer.Application", PSSecurityLevelAsk},
		// Start-Process with elevation
		{regexp.MustCompile(`(?i)start-process\s+.*-verb\s+runas`), "Start-Process with elevation privilege", PSSecurityLevelAsk},
		// Nested PowerShell invocation
		{regexp.MustCompile(`(?i)(powershell\.exe|pwsh)\s+.*-command`), "Nested PowerShell invocation", PSSecurityLevelAsk},
	}

	for _, dp := range dangerousPatterns {
		if dp.pattern.MatchString(command) {
			return PSSecurityResult{Level: dp.level, Message: dp.message}
		}
	}

	// Check for UNC paths (potential SSRF)
	if hasVulnerableUNCPath(command) {
		return PSSecurityResult{Level: PSSecurityLevelAsk, Message: "UNC path may expose internal resources"}
	}

	return PSSecurityResult{Level: PSSecurityLevelAllow}
}

// hasVulnerableUNCPath checks for UNC paths that could be exploited
func hasVulnerableUNCPath(command string) bool {
	// Check for UNC paths that could expose internal network resources
	// Pattern: \\server\share
	uncRegex := regexp.MustCompile(`\\\\[^\\]+\\[^\\]+`)
	return uncRegex.MatchString(command)
}

// detectBlockedPowerShellSleepPattern detects blocking sleep patterns in PowerShell
func detectBlockedPowerShellSleepPattern(command string) string {
	// Split by PowerShell statement separators: ; | & && || newline
	first := strings.TrimSpace(command)
	if idx := strings.IndexAny(first, ";|&\n"); idx > 0 {
		first = strings.TrimSpace(first[:idx])
	}

	if first == "" {
		return ""
	}

	// Match: Start-Sleep N, Start-Sleep -Seconds N, sleep N
	// sleep is a PowerShell alias for Start-Sleep
	sleepRegex := regexp.MustCompile(`(?i)^(start-sleep|sleep)(?:\s+(-s|-seconds))?\s+(\d+)\s*$`)
	m := sleepRegex.FindStringSubmatch(first)
	if m == nil {
		return ""
	}

	// Parse duration
	secs := 0
	for _, c := range m[3] {
		secs = secs*10 + int(c-'0')
	}

	// Sub-2s sleeps are fine (rate limiting)
	if secs < 2 {
		return ""
	}

	// Return the blocked pattern description
	return fmt.Sprintf("Start-Sleep %d", secs)
}

// Dangerous cmdlets that should be blocked or require permission
var dangerousPowerShellCmdlets = map[string]bool{
	"invoke-expression":   true,
	"iex":                 true,
	"start-bitstransfer":  true,
	"set-executionpolicy": true,
	"invoke-webrequest":   true, // when used with iex
	"download-string":     true,
	"download-file":       true,
}

// Cmdlets that should prompt for permission (ask)
var askPowerShellCmdlets = map[string]bool{
	"remove-item":             true,
	"del":                     true,
	"rm":                      true,
	"rmdir":                   true,
	"erase":                   true,
	"move-item":               true,
	"mv":                      true,
	"rename-item":             true,
	"ren":                     true,
	"copy-item":               true,
	"cp":                      true,
	"new-item":                true,
	"ni":                      true,
	"mkdir":                   true,
	"set-content":             true,
	"sc":                      true,
	"add-content":             true,
	"ac":                      true,
	"clear-content":           true,
	"clc":                     true,
	"set-item":                true,
	"si":                      true,
	"set-itemproperty":        true,
	"sp":                      true,
	"remove-itemproperty":     true,
	"rp":                      true,
	"new-itemproperty":        true,
	"stop-process":            true,
	"kill":                    true,
	"start-process":           true,
	"set-service":             true,
	"stop-service":            true,
	"start-service":           true,
	"remove-service":          true,
	"new-service":             true,
	"disable-netadapter":      true,
	"enable-netadapter":       true,
	"set-netipaddress":        true,
	"remove-netipaddress":     true,
	"new-netipaddress":        true,
	"new-localuser":           true,
	"remove-localuser":        true,
	"set-localuser":           true,
	"new-localgroup":          true,
	"remove-localgroup":       true,
	"add-localgroupmember":    true,
	"remove-localgroupmember": true,
}
