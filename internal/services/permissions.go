package services

import (
	"context"
	"fmt"
	"strings"

	"claude-go/internal/types"
)

// PermissionService handles permission checking and rule management.
type PermissionService struct {
	context *types.ToolPermissionContext
}

// NewPermissionService creates a new permission service.
func NewPermissionService(ctx *types.ToolPermissionContext) *PermissionService {
	if ctx == nil {
		ctx = &types.ToolPermissionContext{
			Mode:                         types.PermissionModeDefault,
			AlwaysAllowRules:             make(types.ToolPermissionRulesBySource),
			AlwaysDenyRules:              make(types.ToolPermissionRulesBySource),
			AlwaysAskRules:               make(types.ToolPermissionRulesBySource),
			AdditionalWorkingDirectories: make(map[string]types.AdditionalWorkingDirectory),
		}
	}
	return &PermissionService{context: ctx}
}

// GetContext returns the current permission context.
func (s *PermissionService) GetContext() *types.ToolPermissionContext {
	return s.context
}

// SetContext updates the permission context.
func (s *PermissionService) SetContext(ctx *types.ToolPermissionContext) {
	s.context = ctx
}

// GetMode returns the current permission mode.
func (s *PermissionService) GetMode() types.PermissionMode {
	if s.context == nil {
		return types.PermissionModeDefault
	}
	return s.context.Mode
}

// SetMode updates the permission mode.
func (s *PermissionService) SetMode(mode types.PermissionMode) {
	if s.context != nil {
		s.context.Mode = mode
	}
}

// GetAllowRules returns all allow rules from all sources.
func (s *PermissionService) GetAllowRules() []types.PermissionRule {
	return s.getRulesByBehavior(types.PermissionBehaviorAllow)
}

// GetDenyRules returns all deny rules from all sources.
func (s *PermissionService) GetDenyRules() []types.PermissionRule {
	return s.getRulesByBehavior(types.PermissionBehaviorDeny)
}

// GetAskRules returns all ask rules from all sources.
func (s *PermissionService) GetAskRules() []types.PermissionRule {
	return s.getRulesByBehavior(types.PermissionBehaviorAsk)
}

// getRulesByBehavior returns all rules with the specified behavior.
func (s *PermissionService) getRulesByBehavior(behavior types.PermissionBehavior) []types.PermissionRule {
	var rules []types.PermissionRule

	var rulesBySource types.ToolPermissionRulesBySource
	switch behavior {
	case types.PermissionBehaviorAllow:
		rulesBySource = s.context.AlwaysAllowRules
	case types.PermissionBehaviorDeny:
		rulesBySource = s.context.AlwaysDenyRules
	case types.PermissionBehaviorAsk:
		rulesBySource = s.context.AlwaysAskRules
	}

	for source, ruleStrings := range rulesBySource {
		for _, ruleStr := range ruleStrings {
			ruleValue := types.ParseRuleValue(ruleStr)
			rules = append(rules, types.PermissionRule{
				Source:       source,
				RuleBehavior: behavior,
				RuleValue:    ruleValue,
			})
		}
	}

	return rules
}

// CheckPermission checks if a tool can be used with the given input.
// This implements the core permission checking logic.
func (s *PermissionService) CheckPermission(
	ctx context.Context,
	toolName string,
	toolInput map[string]interface{},
	toolUseID string,
) types.PermissionResult {
	// 1. Check bypass mode FIRST (before any rules)
	// This allows bypass mode to override everything
	if s.IsBypassMode() {
		return types.PermissionResult{
			Behavior:     types.PermissionBehaviorAllow,
			UpdatedInput: toolInput,
			DecisionReason: &types.PermissionDecisionReason{
				Type: "mode",
				Mode: s.context.Mode,
			},
		}
	}

	// 2. Check if the tool is denied by rule
	if denyRule := s.GetDenyRuleForTool(toolName); denyRule != nil {
		return types.PermissionResult{
			Behavior: types.PermissionBehaviorDeny,
			Message:  fmt.Sprintf("Permission to use %s has been denied.", toolName),
			DecisionReason: &types.PermissionDecisionReason{
				Type: "rule",
				Rule: denyRule,
			},
		}
	}

	// 3. Check if the tool has an ask rule
	if askRule := s.GetAskRuleForTool(toolName); askRule != nil {
		return types.PermissionResult{
			Behavior: types.PermissionBehaviorAsk,
			Message:  s.CreatePermissionRequestMessage(toolName, nil),
			DecisionReason: &types.PermissionDecisionReason{
				Type: "rule",
				Rule: askRule,
			},
		}
	}

	// 4. Check if tool is always allowed by rule
	if allowRule := s.GetAllowRuleForTool(toolName); allowRule != nil {
		return types.PermissionResult{
			Behavior:     types.PermissionBehaviorAllow,
			UpdatedInput: toolInput,
			DecisionReason: &types.PermissionDecisionReason{
				Type: "rule",
				Rule: allowRule,
			},
		}
	}

	// 5. Check dontAsk mode - convert ask to deny
	if s.context.Mode == types.PermissionModeDontAsk {
		return types.PermissionResult{
			Behavior: types.PermissionBehaviorDeny,
			Message:  fmt.Sprintf("Permission prompts are disabled. Tool %s requires permission.", toolName),
			DecisionReason: &types.PermissionDecisionReason{
				Type: "mode",
				Mode: types.PermissionModeDontAsk,
			},
		}
	}

	// 6. REPL safeguard in acceptEdits mode.
	// TS parity: REPL should not be auto-allowed by acceptEdits fast-path.
	if s.context.Mode == types.PermissionModeAcceptEdits && strings.EqualFold(strings.TrimSpace(toolName), "REPL") {
		return types.PermissionResult{
			Behavior: types.PermissionBehaviorAsk,
			Message:  s.CreatePermissionRequestMessage(toolName, nil),
			DecisionReason: &types.PermissionDecisionReason{
				Type: "mode",
				Mode: types.PermissionModeAcceptEdits,
			},
		}
	}

	// 6. Check acceptEdits mode for read-only tools
	if s.context.Mode == types.PermissionModeAcceptEdits && types.IsReadOnlyTool(toolName) {
		return types.PermissionResult{
			Behavior:     types.PermissionBehaviorAllow,
			UpdatedInput: toolInput,
			DecisionReason: &types.PermissionDecisionReason{
				Type: "mode",
				Mode: types.PermissionModeAcceptEdits,
			},
		}
	}

	// 7. Default: ask for permission
	return types.PermissionResult{
		Behavior: types.PermissionBehaviorAsk,
		Message:  s.CreatePermissionRequestMessage(toolName, nil),
	}
}

// GetDenyRuleForTool returns the deny rule for a tool, if any.
func (s *PermissionService) GetDenyRuleForTool(toolName string) *types.PermissionRule {
	rules := s.GetDenyRules()
	for _, rule := range rules {
		if s.toolMatchesRule(toolName, rule) {
			return &rule
		}
	}
	return nil
}

// GetAskRuleForTool returns the ask rule for a tool, if any.
func (s *PermissionService) GetAskRuleForTool(toolName string) *types.PermissionRule {
	rules := s.GetAskRules()
	for _, rule := range rules {
		if s.toolMatchesRule(toolName, rule) {
			return &rule
		}
	}
	return nil
}

// GetAllowRuleForTool returns the allow rule for a tool, if any.
func (s *PermissionService) GetAllowRuleForTool(toolName string) *types.PermissionRule {
	rules := s.GetAllowRules()
	for _, rule := range rules {
		if s.toolMatchesRule(toolName, rule) {
			return &rule
		}
	}
	return nil
}

// toolMatchesRule checks if a tool matches a permission rule.
func (s *PermissionService) toolMatchesRule(toolName string, rule types.PermissionRule) bool {
	// If rule has content, it doesn't match the entire tool
	if rule.RuleValue.RuleContent != "" {
		return false
	}

	// Direct name match
	if rule.RuleValue.ToolName == toolName {
		return true
	}

	// Wildcard match
	if rule.RuleValue.ToolName == "*" {
		return true
	}

	// MCP server-level match: rule "mcp__server1" matches tool "mcp__server1__tool1"
	if s.isMCPServerMatch(rule.RuleValue.ToolName, toolName) {
		return true
	}

	return false
}

// isMCPServerMatch checks if an MCP server rule matches a tool.
func (s *PermissionService) isMCPServerMatch(ruleName, toolName string) bool {
	// Check for MCP naming pattern: mcp__server__tool
	if !strings.HasPrefix(toolName, "mcp__") {
		return false
	}

	// Parse MCP info from rule and tool names
	ruleInfo := s.parseMCPInfo(ruleName)
	toolInfo := s.parseMCPInfo(toolName)

	if ruleInfo == nil || toolInfo == nil {
		return false
	}

	// Rule matches if it's a server-level rule (no tool name or wildcard)
	// and the server names match
	if (ruleInfo.toolName == "" || ruleInfo.toolName == "*") && ruleInfo.serverName == toolInfo.serverName {
		return true
	}

	return false
}

// mcpInfo contains parsed MCP server and tool names.
type mcpInfo struct {
	serverName string
	toolName   string
}

// parseMCPInfo parses an MCP tool name into server and tool components.
func (s *PermissionService) parseMCPInfo(name string) *mcpInfo {
	// MCP tool names are: mcp__serverName__toolName
	if !strings.HasPrefix(name, "mcp__") {
		return nil
	}

	parts := strings.SplitN(name[5:], "__", 2)
	if len(parts) == 0 {
		return nil
	}

	info := &mcpInfo{serverName: parts[0]}
	if len(parts) > 1 {
		info.toolName = parts[1]
	}

	return info
}

// IsBypassMode returns true if permissions should be bypassed.
func (s *PermissionService) IsBypassMode() bool {
	if s.context == nil {
		return false
	}

	// Direct bypass mode
	if s.context.Mode == types.PermissionModeBypass {
		return true
	}

	// Plan mode with bypass available
	if s.context.Mode == types.PermissionModePlan && s.context.IsBypassPermissionsModeAvailable {
		return true
	}

	return false
}

// CreatePermissionRequestMessage creates a message explaining why permission is needed.
func (s *PermissionService) CreatePermissionRequestMessage(
	toolName string,
	decisionReason *types.PermissionDecisionReason,
) string {
	if decisionReason != nil {
		switch decisionReason.Type {
		case "rule":
			ruleStr := types.FormatRuleValue(decisionReason.Rule.RuleValue)
			return fmt.Sprintf("Permission rule '%s' requires approval for this %s command", ruleStr, toolName)
		case "mode":
			return fmt.Sprintf("Current permission mode (%s) requires approval for this %s command", decisionReason.Mode, toolName)
		case "hook":
			if decisionReason.Reason != "" {
				return fmt.Sprintf("Hook '%s' blocked this action: %s", decisionReason.HookName, decisionReason.Reason)
			}
			return fmt.Sprintf("Hook '%s' requires approval for this %s command", decisionReason.HookName, toolName)
		case "safetyCheck":
			return decisionReason.Reason
		case "other":
			return decisionReason.Reason
		}
	}

	return fmt.Sprintf("Claude requested permissions to use %s, but you haven't granted it yet.", toolName)
}

// ApplyPermissionUpdate applies a permission update to the context.
func (s *PermissionService) ApplyPermissionUpdate(update types.PermissionUpdate) {
	if s.context == nil {
		return
	}

	switch update.Type {
	case "addRules":
		s.addRules(update)
	case "replaceRules":
		s.replaceRules(update)
	case "removeRules":
		s.removeRules(update)
	case "setMode":
		s.context.Mode = update.Mode
	case "addDirectories":
		s.addDirectories(update)
	case "removeDirectories":
		s.removeDirectories(update)
	}
}

// addRules adds rules to the context.
func (s *PermissionService) addRules(update types.PermissionUpdate) {
	var rulesBySource *types.ToolPermissionRulesBySource
	switch update.Behavior {
	case types.PermissionBehaviorAllow:
		rulesBySource = &s.context.AlwaysAllowRules
	case types.PermissionBehaviorDeny:
		rulesBySource = &s.context.AlwaysDenyRules
	case types.PermissionBehaviorAsk:
		rulesBySource = &s.context.AlwaysAskRules
	}

	if rulesBySource == nil {
		return
	}

	if *rulesBySource == nil {
		*rulesBySource = make(types.ToolPermissionRulesBySource)
	}

	source := types.PermissionRuleSource(update.Destination)
	existing := (*rulesBySource)[source]
	for _, rule := range update.Rules {
		ruleStr := types.FormatRuleValue(rule)
		// Avoid duplicates
		found := false
		for _, r := range existing {
			if r == ruleStr {
				found = true
				break
			}
		}
		if !found {
			existing = append(existing, ruleStr)
		}
	}
	(*rulesBySource)[source] = existing
}

// replaceRules replaces rules in the context.
func (s *PermissionService) replaceRules(update types.PermissionUpdate) {
	var rulesBySource *types.ToolPermissionRulesBySource
	switch update.Behavior {
	case types.PermissionBehaviorAllow:
		rulesBySource = &s.context.AlwaysAllowRules
	case types.PermissionBehaviorDeny:
		rulesBySource = &s.context.AlwaysDenyRules
	case types.PermissionBehaviorAsk:
		rulesBySource = &s.context.AlwaysAskRules
	}

	if rulesBySource == nil {
		return
	}

	if *rulesBySource == nil {
		*rulesBySource = make(types.ToolPermissionRulesBySource)
	}

	source := types.PermissionRuleSource(update.Destination)
	var newRules []string
	for _, rule := range update.Rules {
		newRules = append(newRules, types.FormatRuleValue(rule))
	}
	(*rulesBySource)[source] = newRules
}

// removeRules removes rules from the context.
func (s *PermissionService) removeRules(update types.PermissionUpdate) {
	var rulesBySource *types.ToolPermissionRulesBySource
	switch update.Behavior {
	case types.PermissionBehaviorAllow:
		rulesBySource = &s.context.AlwaysAllowRules
	case types.PermissionBehaviorDeny:
		rulesBySource = &s.context.AlwaysDenyRules
	case types.PermissionBehaviorAsk:
		rulesBySource = &s.context.AlwaysAskRules
	}

	if rulesBySource == nil {
		return
	}

	source := types.PermissionRuleSource(update.Destination)
	existing := (*rulesBySource)[source]

	for _, ruleToRemove := range update.Rules {
		ruleStr := types.FormatRuleValue(ruleToRemove)
		var filtered []string
		for _, r := range existing {
			if r != ruleStr {
				filtered = append(filtered, r)
			}
		}
		existing = filtered
	}

	(*rulesBySource)[source] = existing
}

// addDirectories adds directories to the permission scope.
func (s *PermissionService) addDirectories(update types.PermissionUpdate) {
	if s.context.AdditionalWorkingDirectories == nil {
		s.context.AdditionalWorkingDirectories = make(map[string]types.AdditionalWorkingDirectory)
	}

	source := types.PermissionRuleSource(update.Destination)
	for _, dir := range update.Directories {
		s.context.AdditionalWorkingDirectories[dir] = types.AdditionalWorkingDirectory{
			Path:   dir,
			Source: source,
		}
	}
}

// removeDirectories removes directories from the permission scope.
func (s *PermissionService) removeDirectories(update types.PermissionUpdate) {
	for _, dir := range update.Directories {
		delete(s.context.AdditionalWorkingDirectories, dir)
	}
}

// CheckContentRule checks if content matches a rule pattern.
func (s *PermissionService) CheckContentRule(
	toolName string,
	content string,
	behavior types.PermissionBehavior,
) *types.PermissionRule {
	var rules []types.PermissionRule
	switch behavior {
	case types.PermissionBehaviorAllow:
		rules = s.GetAllowRules()
	case types.PermissionBehaviorDeny:
		rules = s.GetDenyRules()
	case types.PermissionBehaviorAsk:
		rules = s.GetAskRules()
	}

	for _, rule := range rules {
		if rule.RuleValue.ToolName == toolName && rule.RuleValue.RuleContent != "" {
			if types.MatchGlob(rule.RuleValue.RuleContent, content) {
				return &rule
			}
		}
	}

	return nil
}

// GetRuleByContentForTool returns rules with content patterns for a specific tool.
func (s *PermissionService) GetRuleByContentForTool(
	toolName string,
	behavior types.PermissionBehavior,
) map[string]types.PermissionRule {
	rules := make(map[string]types.PermissionRule)

	var allRules []types.PermissionRule
	switch behavior {
	case types.PermissionBehaviorAllow:
		allRules = s.GetAllowRules()
	case types.PermissionBehaviorDeny:
		allRules = s.GetDenyRules()
	case types.PermissionBehaviorAsk:
		allRules = s.GetAskRules()
	}

	for _, rule := range allRules {
		if rule.RuleValue.ToolName == toolName && rule.RuleValue.RuleContent != "" {
			rules[rule.RuleValue.RuleContent] = rule
		}
	}

	return rules
}

// IsWorkingDirectoryAllowed checks if a path is within the allowed directories.
func (s *PermissionService) IsWorkingDirectoryAllowed(path string) bool {
	// Check if path is in additional working directories
	for dir := range s.context.AdditionalWorkingDirectories {
		if strings.HasPrefix(path, dir) {
			return true
		}
	}
	return false
}

// AddWorkingDirectory adds a working directory to the permission scope.
func (s *PermissionService) AddWorkingDirectory(path string, source types.PermissionRuleSource) {
	if s.context.AdditionalWorkingDirectories == nil {
		s.context.AdditionalWorkingDirectories = make(map[string]types.AdditionalWorkingDirectory)
	}
	s.context.AdditionalWorkingDirectories[path] = types.AdditionalWorkingDirectory{
		Path:   path,
		Source: source,
	}
}

// RemoveWorkingDirectory removes a working directory from the permission scope.
func (s *PermissionService) RemoveWorkingDirectory(path string) {
	delete(s.context.AdditionalWorkingDirectories, path)
}

// GetWorkingDirectories returns all additional working directories.
func (s *PermissionService) GetWorkingDirectories() []types.AdditionalWorkingDirectory {
	var dirs []types.AdditionalWorkingDirectory
	for _, dir := range s.context.AdditionalWorkingDirectories {
		dirs = append(dirs, dir)
	}
	return dirs
}
