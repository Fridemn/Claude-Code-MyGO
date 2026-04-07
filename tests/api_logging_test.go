package tests

import (
	"net/http"
	"os"
	"testing"

	"claude-code-go/internal/api"
)

func TestGetAPIProvider(t *testing.T) {
	origBedrock := getEnv("CLAUDE_CODE_USE_BEDROCK")
	origVertex := getEnv("CLAUDE_CODE_USE_VERTEX")
	origFoundry := getEnv("CLAUDE_CODE_USE_FOUNDRY")
	defer setEnv("CLAUDE_CODE_USE_BEDROCK", origBedrock)
	defer setEnv("CLAUDE_CODE_USE_VERTEX", origVertex)
	defer setEnv("CLAUDE_CODE_USE_FOUNDRY", origFoundry)

	setEnv("CLAUDE_CODE_USE_BEDROCK", "")
	setEnv("CLAUDE_CODE_USE_VERTEX", "")
	setEnv("CLAUDE_CODE_USE_FOUNDRY", "")
	if api.GetAPIProvider() != api.ProviderFirstParty {
		t.Error("Default provider should be firstParty")
	}

	setEnv("CLAUDE_CODE_USE_BEDROCK", "true")
	if api.GetAPIProvider() != api.ProviderBedrock {
		t.Error("Provider should be bedrock when CLAUDE_CODE_USE_BEDROCK=true")
	}

	setEnv("CLAUDE_CODE_USE_BEDROCK", "")
	setEnv("CLAUDE_CODE_USE_VERTEX", "true")
	if api.GetAPIProvider() != api.ProviderVertex {
		t.Error("Provider should be vertex when CLAUDE_CODE_USE_VERTEX=true")
	}
}

func TestParseCustomHeaders(t *testing.T) {
	result := api.ParseCustomHeaders("X-Custom: value\nX-Another: test")
	if result["X-Custom"] != "value" {
		t.Errorf("Header X-Custom = %q, want 'value'", result["X-Custom"])
	}
	if result["X-Another"] != "test" {
		t.Errorf("Header X-Another = %q, want 'test'", result["X-Another"])
	}
}

func TestIsFirstPartyAnthropicBaseURL(t *testing.T) {
	if !api.IsFirstPartyAnthropicBaseURL("") {
		t.Error("Empty URL should be first-party")
	}
	if !api.IsFirstPartyAnthropicBaseURL("https://api.anthropic.com/v1/messages") {
		t.Error("api.anthropic.com should be first-party")
	}
	if api.IsFirstPartyAnthropicBaseURL("https://custom.api.com/v1") {
		t.Error("custom URL should not be first-party")
	}
}

func TestDetectGateway(t *testing.T) {
	tests := []struct {
		name     string
		headers  http.Header
		baseURL  string
		expected api.KnownGateway
	}{
		{"no gateway", nil, "", ""},
		{"litellm", http.Header{"X-Litellm-Request-Id": []string{"123"}}, "", api.GatewayLiteLLM},
		{"helicone", http.Header{"Helicone-Id": []string{"456"}}, "", api.GatewayHelicone},
		{"portkey", http.Header{"X-Portkey-Request-Id": []string{"789"}}, "", api.GatewayPortkey},
		{"cloudflare", http.Header{"Cf-Aig-Request-Id": []string{"abc"}}, "", api.GatewayCloudflareAIG},
		{"kong", http.Header{"X-Kong-Proxy-Latency": []string{"100"}}, "", api.GatewayKong},
		{"braintrust", http.Header{"X-Bt-Cached": []string{"true"}}, "", api.GatewayBraintrust},
		{"databricks", nil, "https://my.cloud.databricks.com/serving", api.GatewayDatabricks},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := api.DetectGateway(tt.headers, tt.baseURL)
			if result != tt.expected {
				t.Errorf("DetectGateway() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestAPIUsage(t *testing.T) {
	usage := api.APIUsage{InputTokens: 1000, OutputTokens: 500, CacheReadTokens: 200, CacheCreationTokens: 100}
	delta := api.APIUsage{InputTokens: 500, OutputTokens: 250, CacheReadTokens: 100, CacheCreationTokens: 50}
	updated := api.UpdateUsage(usage, delta)
	if updated.InputTokens != 1500 {
		t.Errorf("InputTokens = %d, want 1500", updated.InputTokens)
	}
}

func TestCalculateCost(t *testing.T) {
	usage := api.APIUsage{InputTokens: 1000000, OutputTokens: 500000}
	cost := api.CalculateCost(usage, "claude-sonnet-4-5")
	if cost < 10.0 || cost > 50.0 {
		t.Errorf("CalculateCost() = %v, want between 10 and 50", cost)
	}
}

func TestAPIFormatDuration(t *testing.T) {
	tests := []struct {
		ms       int64
		expected string
	}{
		{100, "100ms"},
		{1500, "1.5s"},
		{90000, "1.5m"},
	}
	for _, tt := range tests {
		result := api.FormatDuration(tt.ms)
		if result != tt.expected {
			t.Errorf("FormatDuration(%d) = %s, want %s", tt.ms, result, tt.expected)
		}
	}
}

func TestPromptCacheDetector(t *testing.T) {
	detector := api.CreatePromptCacheDetector()

	snapshot := api.PromptStateSnapshot{
		System:       []map[string]any{{"type": "text", "text": "system prompt"}},
		ToolSchemas:  []map[string]any{{"name": "bash", "description": "run bash"}},
		QuerySource:  "repl_main_thread",
		Model:        "claude-sonnet-4-5",
	}

	detector.RecordPromptState(snapshot)
	result := detector.CheckResponseForCacheBreak("repl_main_thread", "", 10000, 0)
	if result != nil {
		t.Error("First call should return nil")
	}

	detector.RecordPromptState(snapshot)
	detector.CheckResponseForCacheBreak("repl_main_thread", "", 10000, 0)

	detector.RecordPromptState(snapshot)
	result = detector.CheckResponseForCacheBreak("repl_main_thread", "", 1000, 0)
	if result == nil {
		t.Error("Cache break should be detected")
	}
	if result != nil && !result.Detected {
		t.Error("Result.Detected should be true")
	}
}

func TestPromptCacheDetectorChanges(t *testing.T) {
	detector := api.CreatePromptCacheDetector()

	snapshot := api.PromptStateSnapshot{
		System:       []map[string]any{{"type": "text", "text": "system prompt"}},
		ToolSchemas:  []map[string]any{{"name": "bash", "description": "run bash"}},
		QuerySource:  "repl_main_thread",
		Model:        "claude-sonnet-4-5",
	}

	detector.RecordPromptState(snapshot)
	detector.CheckResponseForCacheBreak("repl_main_thread", "", 10000, 0)

	changedSnapshot := api.PromptStateSnapshot{
		System:       []map[string]any{{"type": "text", "text": "different system prompt"}},
		ToolSchemas:  []map[string]any{{"name": "bash", "description": "run bash"}},
		QuerySource:  "repl_main_thread",
		Model:        "claude-sonnet-4-5",
	}
	detector.RecordPromptState(changedSnapshot)

	result := detector.CheckResponseForCacheBreak("repl_main_thread", "", 1000, 0)
	if result == nil {
		t.Error("Cache break should be detected")
	}
	if result != nil && result.Changes == nil {
		t.Error("Result.Changes should not be nil when system prompt changed")
	}
}

func TestPromptCacheDetectorReset(t *testing.T) {
	detector := api.CreatePromptCacheDetector()
	snapshot := api.PromptStateSnapshot{QuerySource: "repl_main_thread", Model: "claude-sonnet-4-5"}
	detector.RecordPromptState(snapshot)
	detector.Reset()
	result := detector.CheckResponseForCacheBreak("repl_main_thread", "", 1000, 0)
	if result != nil {
		t.Error("After reset, first call should return nil")
	}
}

// Helper functions for env manipulation in tests
func getEnv(key string) string {
	return os.Getenv(key)
}
func setEnv(key, value string) {
	os.Setenv(key, value)
}