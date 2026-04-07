package api

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"
)

// APIProvider represents the API provider type
// Matches TypeScript: src/utils/model/providers.ts
type APIProvider string

const (
	ProviderFirstParty APIProvider = "firstParty"
	ProviderBedrock    APIProvider = "bedrock"
	ProviderVertex     APIProvider = "vertex"
	ProviderFoundry    APIProvider = "foundry"
)

// GetAPIProvider detects the API provider from environment variables
// Matches TypeScript getAPIProvider()
func GetAPIProvider() APIProvider {
	if isEnvTruthy(os.Getenv("CLAUDE_CODE_USE_BEDROCK")) {
		return ProviderBedrock
	}
	if isEnvTruthy(os.Getenv("CLAUDE_CODE_USE_VERTEX")) {
		return ProviderVertex
	}
	if isEnvTruthy(os.Getenv("CLAUDE_CODE_USE_FOUNDRY")) {
		return ProviderFoundry
	}
	return ProviderFirstParty
}

// ProviderConfig holds provider-specific configuration
type ProviderConfig struct {
	Provider      APIProvider
	BaseURL       string
	APIKey        string
	AuthToken     string // OAuth token
	Model         string
	Timeout       time.Duration
	MaxRetries    int
	CustomHeaders map[string]string

	// Bedrock-specific
	AWSRegion       string
	AWSAccessKey    string
	AWSSecretKey    string
	AWSSessionToken string
	SkipBedrockAuth bool

	// Vertex-specific
	GCPProjectID string
	GCPRegion    string

	// Foundry-specific
	FoundryResource string
}

// ProviderClient is the interface for provider-specific clients
type ProviderClient interface {
	Chat(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error)
	ChatStream(ctx context.Context, req ChatCompletionRequest, onChunk func(*StreamChunk) error) (*ChatCompletionResponse, error)
	SetModel(model string)
	GetModel() string
}

// DefaultProviderConfig returns the default provider configuration from environment
func DefaultProviderConfig() ProviderConfig {
	cfg := ProviderConfig{
		Provider:      GetAPIProvider(),
		BaseURL:       getEnvWithDefault("ANTHROPIC_BASE_URL", ""),
		APIKey:        getAPIKeyFromEnv(),
		AuthToken:     getAuthTokenFromEnv(),
		Model:         os.Getenv("ANTHROPIC_MODEL"),
		Timeout:       time.Duration(getEnvInt("API_TIMEOUT_MS", 600000)) * time.Millisecond,
		MaxRetries:    getEnvInt("ANTHROPIC_MAX_RETRIES", 10),
		CustomHeaders: ParseCustomHeaders(os.Getenv("ANTHROPIC_CUSTOM_HEADERS")),
	}

	// Set default base URL based on provider
	if cfg.BaseURL == "" {
		switch cfg.Provider {
		case ProviderFirstParty:
			cfg.BaseURL = "https://api.anthropic.com/v1/messages"
		case ProviderBedrock:
			cfg.AWSRegion = getEnvWithDefault("AWS_REGION", "us-east-1")
		case ProviderVertex:
			cfg.GCPProjectID = os.Getenv("ANTHROPIC_VERTEX_PROJECT_ID")
			cfg.GCPRegion = getVertexRegion()
		case ProviderFoundry:
			cfg.FoundryResource = os.Getenv("ANTHROPIC_FOUNDRY_RESOURCE")
		}
	}

	// Bedrock-specific config
	if cfg.Provider == ProviderBedrock {
		cfg.SkipBedrockAuth = isEnvTruthy(os.Getenv("CLAUDE_CODE_SKIP_BEDROCK_AUTH"))
		cfg.AWSAccessKey = os.Getenv("AWS_ACCESS_KEY_ID")
		cfg.AWSSecretKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
		cfg.AWSSessionToken = os.Getenv("AWS_SESSION_TOKEN")

		if os.Getenv("AWS_BEARER_TOKEN_BEDROCK") != "" {
			cfg.SkipBedrockAuth = true
			if cfg.CustomHeaders == nil {
				cfg.CustomHeaders = make(map[string]string)
			}
			cfg.CustomHeaders["Authorization"] = "Bearer " + os.Getenv("AWS_BEARER_TOKEN_BEDROCK")
		}
	}

	// Vertex-specific config
	if cfg.Provider == ProviderVertex {
		cfg.GCPRegion = getVertexRegion()
	}

	// Foundry-specific config
	if cfg.Provider == ProviderFoundry && cfg.FoundryResource != "" {
		cfg.BaseURL = "https://" + cfg.FoundryResource + ".services.ai.azure.com/anthropic/v1/messages"
	}

	return cfg
}

// ProviderClient creates a client for the specified provider
func CreateProviderClient(cfg ProviderConfig) ProviderClient {
	switch cfg.Provider {
	case ProviderBedrock:
		return CreateBedrockClient(cfg)
	case ProviderVertex:
		return CreateVertexClient(cfg)
	case ProviderFoundry:
		return CreateFoundryClient(cfg)
	default:
		return CreateFirstPartyClient(cfg)
	}
}

// FirstPartyClient creates a first-party Anthropic API client
func CreateFirstPartyClient(cfg ProviderConfig) *OpenAICompatibleClient {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1/messages"
	}

	return &OpenAICompatibleClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     firstNonEmptyString(cfg.APIKey, cfg.AuthToken),
		model:      cfg.Model,
		maxRetries: cfg.MaxRetries,
		retryDelay: DefaultBaseDelay,
	}
}

// BedrockClient implements AWS Bedrock API client
type BedrockClient struct {
	region        string
	accessKey     string
	secretKey     string
	sessionToken  string
	model         string
	skipAuth      bool
	customHeaders map[string]string
	maxRetries    int
}

// BedrockClient creates a new Bedrock client
func CreateBedrockClient(cfg ProviderConfig) *BedrockClient {
	return &BedrockClient{
		region:        cfg.AWSRegion,
		accessKey:     cfg.AWSAccessKey,
		secretKey:     cfg.AWSSecretKey,
		sessionToken:  cfg.AWSSessionToken,
		model:         cfg.Model,
		skipAuth:      cfg.SkipBedrockAuth,
		customHeaders: cfg.CustomHeaders,
		maxRetries:    cfg.MaxRetries,
	}
}

// Chat implements ProviderClient for Bedrock
func (c *BedrockClient) Chat(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	return nil, &APIError{
		Type:    "not_implemented",
		Message: "Bedrock SDK integration requires AWS SDK - use OpenAI-compatible gateway or set CLAUDE_CODE_USE_BEDROCK=false",
	}
}

// ChatStream implements ProviderClient for Bedrock
func (c *BedrockClient) ChatStream(ctx context.Context, req ChatCompletionRequest, onChunk func(*StreamChunk) error) (*ChatCompletionResponse, error) {
	return nil, &APIError{
		Type:    "not_implemented",
		Message: "Bedrock streaming requires AWS SDK - use OpenAI-compatible gateway or set CLAUDE_CODE_USE_BEDROCK=false",
	}
}

// SetModel sets the model
func (c *BedrockClient) SetModel(model string) { c.model = model }

// GetModel returns the model
func (c *BedrockClient) GetModel() string { return c.model }

// VertexClient implements GCP Vertex AI API client
type VertexClient struct {
	projectID   string
	region      string
	model       string
	accessToken string
	maxRetries  int
}

// VertexClient creates a new Vertex client
func CreateVertexClient(cfg ProviderConfig) *VertexClient {
	return &VertexClient{
		projectID:   cfg.GCPProjectID,
		region:      cfg.GCPRegion,
		model:       cfg.Model,
		accessToken: cfg.AuthToken,
		maxRetries:  cfg.MaxRetries,
	}
}

// Chat implements ProviderClient for Vertex
func (c *VertexClient) Chat(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	return nil, &APIError{
		Type:    "not_implemented",
		Message: "Vertex AI integration requires GCP SDK - use OpenAI-compatible gateway or set CLAUDE_CODE_USE_VERTEX=false",
	}
}

// ChatStream implements ProviderClient for Vertex
func (c *VertexClient) ChatStream(ctx context.Context, req ChatCompletionRequest, onChunk func(*StreamChunk) error) (*ChatCompletionResponse, error) {
	return nil, &APIError{
		Type:    "not_implemented",
		Message: "Vertex streaming requires GCP SDK - use OpenAI-compatible gateway or set CLAUDE_CODE_USE_VERTEX=false",
	}
}

// SetModel sets the model
func (c *VertexClient) SetModel(model string) { c.model = model }

// GetModel returns the model
func (c *VertexClient) GetModel() string { return c.model }

// FoundryClient implements Azure Foundry API client
type FoundryClient struct {
	resource    string
	model       string
	apiKey      string
	accessToken string
	baseURL     string
	maxRetries  int
}

// FoundryClient creates a new Foundry client
func CreateFoundryClient(cfg ProviderConfig) *FoundryClient {
	return &FoundryClient{
		resource:    cfg.FoundryResource,
		model:       cfg.Model,
		apiKey:      cfg.APIKey,
		accessToken: cfg.AuthToken,
		baseURL:     cfg.BaseURL,
		maxRetries:  cfg.MaxRetries,
	}
}

// Chat implements ProviderClient for Foundry
func (c *FoundryClient) Chat(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	if c.baseURL != "" {
		client := &OpenAICompatibleClient{
			baseURL:    c.baseURL,
			apiKey:     firstNonEmptyString(c.apiKey, c.accessToken),
			model:      c.model,
			maxRetries: c.maxRetries,
			retryDelay: DefaultBaseDelay,
		}
		return client.Chat(ctx, req)
	}
	return nil, &APIError{
		Type:    "not_implemented",
		Message: "Foundry requires ANTHROPIC_FOUNDRY_RESOURCE or ANTHROPIC_FOUNDRY_BASE_URL",
	}
}

// ChatStream implements ProviderClient for Foundry
func (c *FoundryClient) ChatStream(ctx context.Context, req ChatCompletionRequest, onChunk func(*StreamChunk) error) (*ChatCompletionResponse, error) {
	if c.baseURL != "" {
		client := &OpenAICompatibleClient{
			baseURL:    c.baseURL,
			apiKey:     firstNonEmptyString(c.apiKey, c.accessToken),
			model:      c.model,
			maxRetries: c.maxRetries,
			retryDelay: DefaultBaseDelay,
		}
		return client.ChatStream(ctx, req, onChunk)
	}
	return nil, &APIError{
		Type:    "not_implemented",
		Message: "Foundry requires ANTHROPIC_FOUNDRY_RESOURCE or ANTHROPIC_FOUNDRY_BASE_URL",
	}
}

// SetModel sets the model
func (c *FoundryClient) SetModel(model string) { c.model = model }

// GetModel returns the model
func (c *FoundryClient) GetModel() string { return c.model }

// Helper functions

func isEnvTruthy(val string) bool {
	if val == "" {
		return false
	}
	lower := strings.ToLower(val)
	return lower == "true" || lower == "1" || lower == "yes"
}

func getAPIKeyFromEnv() string {
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		return key
	}
	if key := os.Getenv("CLAUDE_CODE_API_KEY"); key != "" {
		return key
	}
	return ""
}

func getAuthTokenFromEnv() string {
	if token := os.Getenv("ANTHROPIC_AUTH_TOKEN"); token != "" {
		return token
	}
	if token := os.Getenv("CLAUDE_CODE_OAUTH_TOKEN"); token != "" {
		return token
	}
	return ""
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvWithDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getVertexRegion() string {
	if region := os.Getenv("CLOUD_ML_REGION"); region != "" {
		return region
	}
	return "us-east5"
}

// ParseCustomHeaders parses custom headers from environment variable
// Format: "Name: Value\nName2: Value2" (matches TypeScript)
func ParseCustomHeaders(envVal string) map[string]string {
	headers := make(map[string]string)
	if envVal == "" {
		return headers
	}

	lines := strings.Split(envVal, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx := strings.Index(line, ":")
		if idx == -1 {
			continue
		}
		name := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		if name != "" {
			headers[name] = value
		}
	}
	return headers
}

// IsFirstPartyAnthropicBaseURL checks if the base URL is a first-party Anthropic URL
func IsFirstPartyAnthropicBaseURL(baseURL string) bool {
	if baseURL == "" {
		return true
	}

	allowedHosts := []string{"api.anthropic.com"}
	if os.Getenv("USER_TYPE") == "ant" {
		allowedHosts = append(allowedHosts, "api-staging.anthropic.com")
	}

	host := baseURL
	if strings.HasPrefix(host, "https://") {
		host = strings.TrimPrefix(host, "https://")
	} else if strings.HasPrefix(host, "http://") {
		host = strings.TrimPrefix(host, "http://")
	}
	if idx := strings.Index(host, "/"); idx != -1 {
		host = host[:idx]
	}

	for _, allowed := range allowedHosts {
		if host == allowed {
			return true
		}
	}
	return false
}
