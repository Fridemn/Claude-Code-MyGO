package tests

import (
	"testing"

	"claude-go/internal/services"
	"claude-go/internal/types"
)

func TestPermissionServiceCreation(t *testing.T) {
	ctx := &types.ToolPermissionContext{
		Mode: types.PermissionModeDefault,
		AlwaysAllowRules: types.ToolPermissionRulesBySource{
			types.PermissionSourceUserSettings: {"Bash", "Write"},
		},
	}

	mgr := services.NewPermissionService(ctx)
	if mgr == nil {
		t.Fatal("NewPermissionService returned nil")
	}

	if mgr.GetMode() != types.PermissionModeDefault {
		t.Errorf("expected default mode, got %s", mgr.GetMode())
	}
}

func TestPermissionServiceNilContext(t *testing.T) {
	mgr := services.NewPermissionService(nil)
	if mgr == nil {
		t.Fatal("NewPermissionService with nil returned nil")
	}

	if mgr.GetMode() != types.PermissionModeDefault {
		t.Errorf("expected default mode with nil context, got %s", mgr.GetMode())
	}
}

func TestGetAllowRules(t *testing.T) {
	ctx := &types.ToolPermissionContext{
		Mode: types.PermissionModeDefault,
		AlwaysAllowRules: types.ToolPermissionRulesBySource{
			types.PermissionSourceUserSettings:    {"Bash", "Write"},
			types.PermissionSourceProjectSettings: {"Read"},
		},
	}

	mgr := services.NewPermissionService(ctx)
	rules := mgr.GetAllowRules()

	if len(rules) != 3 {
		t.Errorf("expected 3 rules, got %d", len(rules))
	}
}

func TestGetDenyRules(t *testing.T) {
	ctx := &types.ToolPermissionContext{
		Mode: types.PermissionModeDefault,
		AlwaysDenyRules: types.ToolPermissionRulesBySource{
			types.PermissionSourceUserSettings: {"Bash(rm *)"},
		},
	}

	mgr := services.NewPermissionService(ctx)
	rules := mgr.GetDenyRules()

	if len(rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rules))
	}

	if rules[0].RuleValue.ToolName != "Bash" {
		t.Errorf("expected tool name 'Bash', got '%s'", rules[0].RuleValue.ToolName)
	}

	if rules[0].RuleValue.RuleContent != "rm *" {
		t.Errorf("expected rule content 'rm *', got '%s'", rules[0].RuleValue.RuleContent)
	}
}

func TestGetAskRules(t *testing.T) {
	ctx := &types.ToolPermissionContext{
		Mode: types.PermissionModeDefault,
		AlwaysAskRules: types.ToolPermissionRulesBySource{
			types.PermissionSourceProjectSettings: {"Bash(npm publish:*)"},
		},
	}

	mgr := services.NewPermissionService(ctx)
	rules := mgr.GetAskRules()

	if len(rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rules))
	}
}

func TestCheckPermissionDeny(t *testing.T) {
	ctx := &types.ToolPermissionContext{
		Mode: types.PermissionModeDefault,
		AlwaysDenyRules: types.ToolPermissionRulesBySource{
			types.PermissionSourceUserSettings: {"Bash"},
		},
	}

	mgr := services.NewPermissionService(ctx)
	result := mgr.CheckPermission(nil, "Bash", nil, "test-id")

	if !result.IsDenied() {
		t.Errorf("expected denied, got %s", result.Behavior)
	}
}

func TestCheckPermissionAsk(t *testing.T) {
	ctx := &types.ToolPermissionContext{
		Mode: types.PermissionModeDefault,
		AlwaysAskRules: types.ToolPermissionRulesBySource{
			types.PermissionSourceUserSettings: {"Bash"},
		},
	}

	mgr := services.NewPermissionService(ctx)
	result := mgr.CheckPermission(nil, "Bash", nil, "test-id")

	if !result.RequiresConfirmation() {
		t.Errorf("expected ask, got %s", result.Behavior)
	}
}

func TestCheckPermissionAllow(t *testing.T) {
	ctx := &types.ToolPermissionContext{
		Mode: types.PermissionModeDefault,
		AlwaysAllowRules: types.ToolPermissionRulesBySource{
			types.PermissionSourceUserSettings: {"Bash"},
		},
	}

	mgr := services.NewPermissionService(ctx)
	result := mgr.CheckPermission(nil, "Bash", nil, "test-id")

	if !result.IsAllowed() {
		t.Errorf("expected allowed, got %s", result.Behavior)
	}
}

func TestCheckPermissionBypassMode(t *testing.T) {
	ctx := &types.ToolPermissionContext{
		Mode: types.PermissionModeBypass,
		AlwaysAskRules: types.ToolPermissionRulesBySource{
			types.PermissionSourceUserSettings: {"Bash"},
		},
	}

	mgr := services.NewPermissionService(ctx)
	result := mgr.CheckPermission(nil, "Bash", nil, "test-id")

	if !result.IsAllowed() {
		t.Errorf("expected allowed in bypass mode, got %s", result.Behavior)
	}
}

func TestCheckPermissionDontAskMode(t *testing.T) {
	ctx := &types.ToolPermissionContext{
		Mode: types.PermissionModeDontAsk,
	}

	mgr := services.NewPermissionService(ctx)
	result := mgr.CheckPermission(nil, "Bash", nil, "test-id")

	if !result.IsDenied() {
		t.Errorf("expected denied in dontAsk mode, got %s", result.Behavior)
	}
}

func TestCheckPermissionAcceptEditsReadOnly(t *testing.T) {
	ctx := &types.ToolPermissionContext{
		Mode: types.PermissionModeAcceptEdits,
	}

	mgr := services.NewPermissionService(ctx)
	result := mgr.CheckPermission(nil, "Read", nil, "test-id")

	if !result.IsAllowed() {
		t.Errorf("expected allowed for Read in acceptEdits mode, got %s", result.Behavior)
	}
}

func TestCheckPermissionAcceptEdits_REPLStillAsks(t *testing.T) {
	ctx := &types.ToolPermissionContext{
		Mode: types.PermissionModeAcceptEdits,
	}

	mgr := services.NewPermissionService(ctx)
	result := mgr.CheckPermission(nil, "REPL", map[string]interface{}{"script": `Read({"file_path":"README.md"})`}, "test-id")

	if result.Behavior != types.PermissionBehaviorAsk {
		t.Errorf("expected REPL to require approval in acceptEdits mode, got %s", result.Behavior)
	}
}

func TestGetDenyRuleForTool(t *testing.T) {
	ctx := &types.ToolPermissionContext{
		Mode: types.PermissionModeDefault,
		AlwaysDenyRules: types.ToolPermissionRulesBySource{
			types.PermissionSourceUserSettings: {"Bash"},
		},
	}

	mgr := services.NewPermissionService(ctx)
	rule := mgr.GetDenyRuleForTool("Bash")

	if rule == nil {
		t.Fatal("expected deny rule for Bash, got nil")
	}

	if rule.Source != types.PermissionSourceUserSettings {
		t.Errorf("expected source userSettings, got %s", rule.Source)
	}
}

func TestGetAskRuleForTool(t *testing.T) {
	ctx := &types.ToolPermissionContext{
		Mode: types.PermissionModeDefault,
		AlwaysAskRules: types.ToolPermissionRulesBySource{
			types.PermissionSourceProjectSettings: {"Bash"},
		},
	}

	mgr := services.NewPermissionService(ctx)
	rule := mgr.GetAskRuleForTool("Bash")

	if rule == nil {
		t.Fatal("expected ask rule for Bash, got nil")
	}
}

func TestGetAllowRuleForTool(t *testing.T) {
	ctx := &types.ToolPermissionContext{
		Mode: types.PermissionModeDefault,
		AlwaysAllowRules: types.ToolPermissionRulesBySource{
			types.PermissionSourceUserSettings: {"Bash"},
		},
	}

	mgr := services.NewPermissionService(ctx)
	rule := mgr.GetAllowRuleForTool("Bash")

	if rule == nil {
		t.Fatal("expected allow rule for Bash, got nil")
	}
}

func TestToolMatchesRule(t *testing.T) {
	tests := []struct {
		toolName    string
		ruleTool    string
		expectMatch bool
	}{
		{"Bash", "Bash", true},
		{"Bash", "Write", false},
		{"Bash", "*", true},
		{"mcp__server1__tool1", "mcp__server1", true},
		{"mcp__server1__tool1", "mcp__server1__*", true},
		{"mcp__server1__tool1", "mcp__server2__*", false},
		{"mcp__server1__tool1", "mcp__server1__tool1", true},
	}

	for _, tc := range tests {
		t.Run(tc.toolName+"_"+tc.ruleTool, func(t *testing.T) {
			ctx := &types.ToolPermissionContext{
				Mode: types.PermissionModeDefault,
				AlwaysAllowRules: types.ToolPermissionRulesBySource{
					types.PermissionSourceUserSettings: {tc.ruleTool},
				},
			}

			mgr := services.NewPermissionService(ctx)
			rule := mgr.GetAllowRuleForTool(tc.toolName)

			if tc.expectMatch && rule == nil {
				t.Errorf("expected match for %s with rule %s", tc.toolName, tc.ruleTool)
			}
			if !tc.expectMatch && rule != nil {
				t.Errorf("expected no match for %s with rule %s", tc.toolName, tc.ruleTool)
			}
		})
	}
}

func TestIsBypassMode(t *testing.T) {
	tests := []struct {
		mode         types.PermissionMode
		isBypass     bool
		isPlanBypass bool
	}{
		{types.PermissionModeDefault, false, false},
		{types.PermissionModeBypass, true, false},
		{types.PermissionModeDontAsk, false, false},
		{types.PermissionModePlan, false, false},
	}

	for _, tc := range tests {
		t.Run(string(tc.mode), func(t *testing.T) {
			ctx := &types.ToolPermissionContext{
				Mode: tc.mode,
			}
			mgr := services.NewPermissionService(ctx)
			if tc.isBypass != mgr.IsBypassMode() {
				t.Errorf("expected IsBypassMode=%v for %s, got %v", tc.isBypass, tc.mode, mgr.IsBypassMode())
			}
		})
	}
}

func TestPlanModeWithBypass(t *testing.T) {
	ctx := &types.ToolPermissionContext{
		Mode:                             types.PermissionModePlan,
		IsBypassPermissionsModeAvailable: true,
		AlwaysAskRules: types.ToolPermissionRulesBySource{
			types.PermissionSourceUserSettings: {"Bash"},
		},
	}

	mgr := services.NewPermissionService(ctx)
	if !mgr.IsBypassMode() {
		t.Error("expected bypass mode in plan mode with bypass available")
	}
}

func TestApplyAddRulesUpdate(t *testing.T) {
	ctx := &types.ToolPermissionContext{
		Mode: types.PermissionModeDefault,
	}

	mgr := services.NewPermissionService(ctx)
	mgr.ApplyPermissionUpdate(types.PermissionUpdate{
		Type:        "addRules",
		Destination: types.DestUserSettings,
		Rules: []types.PermissionRuleValue{
			{ToolName: "Bash"},
			{ToolName: "Write"},
		},
		Behavior: types.PermissionBehaviorAllow,
	})

	rules := mgr.GetAllowRules()
	if len(rules) != 2 {
		t.Errorf("expected 2 rules after add, got %d", len(rules))
	}
}

func TestApplyReplaceRulesUpdate(t *testing.T) {
	ctx := &types.ToolPermissionContext{
		Mode: types.PermissionModeDefault,
		AlwaysAllowRules: types.ToolPermissionRulesBySource{
			types.PermissionSourceUserSettings: {"Bash", "Write", "Edit"},
		},
	}

	mgr := services.NewPermissionService(ctx)
	mgr.ApplyPermissionUpdate(types.PermissionUpdate{
		Type:        "replaceRules",
		Destination: types.DestUserSettings,
		Rules: []types.PermissionRuleValue{
			{ToolName: "Read"},
		},
		Behavior: types.PermissionBehaviorAllow,
	})

	rules := mgr.GetAllowRules()
	if len(rules) != 1 {
		t.Errorf("expected 1 rule after replace, got %d", len(rules))
	}

	if rules[0].RuleValue.ToolName != "Read" {
		t.Errorf("expected tool name 'Read', got '%s'", rules[0].RuleValue.ToolName)
	}
}

func TestApplyRemoveRulesUpdate(t *testing.T) {
	ctx := &types.ToolPermissionContext{
		Mode: types.PermissionModeDefault,
		AlwaysAllowRules: types.ToolPermissionRulesBySource{
			types.PermissionSourceUserSettings: {"Bash", "Write", "Edit"},
		},
	}

	mgr := services.NewPermissionService(ctx)
	mgr.ApplyPermissionUpdate(types.PermissionUpdate{
		Type:        "removeRules",
		Destination: types.DestUserSettings,
		Rules: []types.PermissionRuleValue{
			{ToolName: "Write"},
		},
		Behavior: types.PermissionBehaviorAllow,
	})

	rules := mgr.GetAllowRules()
	if len(rules) != 2 {
		t.Errorf("expected 2 rules after remove, got %d", len(rules))
	}
}

func TestApplySetModeUpdate(t *testing.T) {
	ctx := &types.ToolPermissionContext{
		Mode: types.PermissionModeDefault,
	}

	mgr := services.NewPermissionService(ctx)
	mgr.ApplyPermissionUpdate(types.PermissionUpdate{
		Type:        "setMode",
		Destination: types.DestUserSettings,
		Mode:        types.PermissionModeBypass,
	})

	if mgr.GetMode() != types.PermissionModeBypass {
		t.Errorf("expected mode bypass, got %s", mgr.GetMode())
	}
}

func TestCheckContentRule(t *testing.T) {
	ctx := &types.ToolPermissionContext{
		Mode: types.PermissionModeDefault,
		AlwaysAskRules: types.ToolPermissionRulesBySource{
			types.PermissionSourceUserSettings: {"Bash(npm publish:*)"},
		},
	}

	mgr := services.NewPermissionService(ctx)
	rule := mgr.CheckContentRule("Bash", "npm publish:mypackage", types.PermissionBehaviorAsk)

	if rule == nil {
		t.Fatal("expected rule match for npm publish, got nil")
	}
}

func TestCheckContentRuleNoMatch(t *testing.T) {
	ctx := &types.ToolPermissionContext{
		Mode: types.PermissionModeDefault,
		AlwaysAskRules: types.ToolPermissionRulesBySource{
			types.PermissionSourceUserSettings: {"Bash(npm publish:*)"},
		},
	}

	mgr := services.NewPermissionService(ctx)
	rule := mgr.CheckContentRule("Bash", "git commit", types.PermissionBehaviorAsk)

	if rule != nil {
		t.Error("expected no rule match for git commit, got a rule")
	}
}

func TestGetRuleByContentForTool(t *testing.T) {
	ctx := &types.ToolPermissionContext{
		Mode: types.PermissionModeDefault,
		AlwaysAskRules: types.ToolPermissionRulesBySource{
			types.PermissionSourceUserSettings: {"Bash(npm publish:*)", "Bash(git push:*)"},
		},
	}

	mgr := services.NewPermissionService(ctx)
	rules := mgr.GetRuleByContentForTool("Bash", types.PermissionBehaviorAsk)

	if len(rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(rules))
	}

	if _, ok := rules["npm publish:*"]; !ok {
		t.Error("expected rule for 'npm publish:*'")
	}
	if _, ok := rules["git push:*"]; !ok {
		t.Error("expected rule for 'git push:*'")
	}
}

func TestIsWorkingDirectoryAllowed(t *testing.T) {
	ctx := &types.ToolPermissionContext{
		Mode: types.PermissionModeDefault,
		AdditionalWorkingDirectories: map[string]types.AdditionalWorkingDirectory{
			"/home/user/project": {Path: "/home/user/project", Source: types.PermissionSourceUserSettings},
			"/tmp/claude":        {Path: "/tmp/claude", Source: types.PermissionSourceCLIArg},
		},
	}

	mgr := services.NewPermissionService(ctx)

	if !mgr.IsWorkingDirectoryAllowed("/home/user/project/src") {
		t.Error("expected /home/user/project/src to be allowed")
	}

	if !mgr.IsWorkingDirectoryAllowed("/tmp/claude/sessions") {
		t.Error("expected /tmp/claude/sessions to be allowed")
	}

	if mgr.IsWorkingDirectoryAllowed("/var/other") {
		t.Error("expected /var/other to be not allowed")
	}
}

func TestAddWorkingDirectory(t *testing.T) {
	ctx := &types.ToolPermissionContext{
		Mode: types.PermissionModeDefault,
	}

	mgr := services.NewPermissionService(ctx)
	mgr.AddWorkingDirectory("/home/user/project", types.PermissionSourceUserSettings)

	if !mgr.IsWorkingDirectoryAllowed("/home/user/project") {
		t.Error("expected directory to be allowed after add")
	}
}

func TestRemoveWorkingDirectory(t *testing.T) {
	ctx := &types.ToolPermissionContext{
		Mode: types.PermissionModeDefault,
		AdditionalWorkingDirectories: map[string]types.AdditionalWorkingDirectory{
			"/tmp/dir": {Path: "/tmp/dir", Source: types.PermissionSourceUserSettings},
		},
	}

	mgr := services.NewPermissionService(ctx)
	mgr.RemoveWorkingDirectory("/tmp/dir")

	if mgr.IsWorkingDirectoryAllowed("/tmp/dir") {
		t.Error("expected directory to be not allowed after remove")
	}
}

func TestGetWorkingDirectories(t *testing.T) {
	ctx := &types.ToolPermissionContext{
		Mode: types.PermissionModeDefault,
		AdditionalWorkingDirectories: map[string]types.AdditionalWorkingDirectory{
			"/tmp/dir1": {Path: "/tmp/dir1", Source: types.PermissionSourceUserSettings},
			"/tmp/dir2": {Path: "/tmp/dir2", Source: types.PermissionSourceCLIArg},
		},
	}

	mgr := services.NewPermissionService(ctx)
	dirs := mgr.GetWorkingDirectories()

	if len(dirs) != 2 {
		t.Errorf("expected 2 directories, got %d", len(dirs))
	}
}

func TestCreatePermissionRequestMessage(t *testing.T) {
	ctx := &types.ToolPermissionContext{
		Mode: types.PermissionModeDefault,
	}

	mgr := services.NewPermissionService(ctx)

	msg := mgr.CreatePermissionRequestMessage("Bash", nil)
	expected := "Claude requested permissions to use Bash, but you haven't granted it yet."
	if msg != expected {
		t.Errorf("expected message '%s', got '%s'", expected, msg)
	}
}

func TestCreatePermissionRequestMessageWithReason(t *testing.T) {
	ctx := &types.ToolPermissionContext{
		Mode: types.PermissionModeDefault,
	}

	mgr := services.NewPermissionService(ctx)

	rule := &types.PermissionRule{
		Source:       types.PermissionSourceUserSettings,
		RuleBehavior: types.PermissionBehaviorAsk,
		RuleValue: types.PermissionRuleValue{
			ToolName: "Bash",
		},
	}

	msg := mgr.CreatePermissionRequestMessage("Bash", &types.PermissionDecisionReason{
		Type: "rule",
		Rule: rule,
	})

	if msg == "" {
		t.Error("expected non-empty message with reason")
	}
}

func TestParseRuleValue(t *testing.T) {
	tests := []struct {
		input           string
		expectedTool    string
		expectedContent string
	}{
		{"Bash", "Bash", ""},
		{"Bash(git *)", "Bash", "git *"},
		{"Write", "Write", ""},
		{"Bash(rm -rf /*)", "Bash", "rm -rf /*"},
		{"Agent(Explore)", "Agent", "Explore"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := types.ParseRuleValue(tc.input)
			if result.ToolName != tc.expectedTool {
				t.Errorf("expected tool name '%s', got '%s'", tc.expectedTool, result.ToolName)
			}
			if result.RuleContent != tc.expectedContent {
				t.Errorf("expected rule content '%s', got '%s'", tc.expectedContent, result.RuleContent)
			}
		})
	}
}

func TestFormatRuleValue(t *testing.T) {
	tests := []struct {
		input    types.PermissionRuleValue
		expected string
	}{
		{types.PermissionRuleValue{ToolName: "Bash"}, "Bash"},
		{types.PermissionRuleValue{ToolName: "Bash", RuleContent: "git *"}, "Bash(git *)"},
		{types.PermissionRuleValue{ToolName: "Write"}, "Write"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			result := types.FormatRuleValue(tc.input)
			if result != tc.expected {
				t.Errorf("expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern string
		text    string
		match   bool
	}{
		{"", "anything", true},
		{"*", "anything", true},
		{"git *", "git commit", true},
		{"git *", "hg commit", false},
		{"*.js", "test.js", true}, // Suffix pattern - matches
		{"exact", "exact", true},
		{"exact", "different", false},
	}

	for _, tc := range tests {
		t.Run(tc.pattern+"_"+tc.text, func(t *testing.T) {
			result := types.MatchGlob(tc.pattern, tc.text)
			if result != tc.match {
				t.Errorf("MatchGlob('%s', '%s') = %v, expected %v", tc.pattern, tc.text, result, tc.match)
			}
		})
	}
}

func TestIsReadOnlyTool(t *testing.T) {
	readOnlyTools := []string{"Read", "Glob", "Grep", "WebFetch", "Agent", "TaskList", "TaskGet"}
	for _, tool := range readOnlyTools {
		if !types.IsReadOnlyTool(tool) {
			t.Errorf("expected %s to be read-only", tool)
		}
	}

	dangerousTools := []string{"Bash", "Write", "Edit", "Delete"}
	for _, tool := range dangerousTools {
		if types.IsReadOnlyTool(tool) {
			t.Errorf("expected %s to not be read-only", tool)
		}
	}
}

func TestIsDangerousTool(t *testing.T) {
	dangerousTools := []string{"Bash", "Write", "Edit", "Delete"}
	for _, tool := range dangerousTools {
		if !types.IsDangerousTool(tool) {
			t.Errorf("expected %s to be dangerous", tool)
		}
	}

	readOnlyTools := []string{"Read", "Glob", "Grep", "WebFetch"}
	for _, tool := range readOnlyTools {
		if types.IsDangerousTool(tool) {
			t.Errorf("expected %s to not be dangerous", tool)
		}
	}
}

func TestIsValidPermissionMode(t *testing.T) {
	validModes := []string{
		"default", "auto", "dontAsk", "bypassPermissions",
		"acceptEdits", "plan", "bubble",
	}
	for _, mode := range validModes {
		if !types.IsValidPermissionMode(mode) {
			t.Errorf("expected '%s' to be valid", mode)
		}
	}

	invalidModes := []string{"invalid", "", "unknown", "ADMIN"}
	for _, mode := range invalidModes {
		if types.IsValidPermissionMode(mode) {
			t.Errorf("expected '%s' to be invalid", mode)
		}
	}
}

func TestIsExternalPermissionMode(t *testing.T) {
	externalModes := []types.PermissionMode{
		types.PermissionModeDefault,
		types.PermissionModeDontAsk,
		types.PermissionModeBypass,
		types.PermissionModeAcceptEdits,
		types.PermissionModePlan,
	}
	for _, mode := range externalModes {
		if !types.IsExternalPermissionMode(mode) {
			t.Errorf("expected '%s' to be external", mode)
		}
	}

	internalModes := []types.PermissionMode{
		types.PermissionModeAuto,
		types.PermissionModeBubble,
	}
	for _, mode := range internalModes {
		if types.IsExternalPermissionMode(mode) {
			t.Errorf("expected '%s' to not be external", mode)
		}
	}
}

func TestPermissionResultHelpers(t *testing.T) {
	result := types.PermissionResult{
		Behavior: types.PermissionBehaviorAllow,
		Message:  "test",
	}

	if !result.IsAllowed() {
		t.Error("expected IsAllowed to return true")
	}
	if result.IsDenied() {
		t.Error("expected IsDenied to return false")
	}
	if result.RequiresConfirmation() {
		t.Error("expected RequiresConfirmation to return false")
	}

	result.Behavior = types.PermissionBehaviorDeny
	if result.IsAllowed() {
		t.Error("expected IsAllowed to return false")
	}
	if !result.IsDenied() {
		t.Error("expected IsDenied to return true")
	}

	result.Behavior = types.PermissionBehaviorAsk
	if !result.RequiresConfirmation() {
		t.Error("expected RequiresConfirmation to return true")
	}
}

func TestNewPermissionResults(t *testing.T) {
	allow := types.NewAllowResult("allowed because")
	if !allow.IsAllowed() {
		t.Error("expected allow result to be allowed")
	}

	deny := types.NewDenyResult("denied because")
	if !deny.IsDenied() {
		t.Error("expected deny result to be denied")
	}

	ask := types.NewAskResult("please confirm")
	if !ask.RequiresConfirmation() {
		t.Error("expected ask result to require confirmation")
	}
}
