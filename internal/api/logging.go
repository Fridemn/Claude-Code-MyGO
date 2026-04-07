package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// KnownGateway represents detected AI gateway types
// Matches TypeScript logging.ts:56-64
type KnownGateway string

const (
	GatewayLiteLLM       KnownGateway = "litellm"
	GatewayHelicone      KnownGateway = "helicone"
	GatewayPortkey       KnownGateway = "portkey"
	GatewayCloudflareAIG KnownGateway = "cloudflare-ai-gateway"
	GatewayKong          KnownGateway = "kong"
	GatewayBraintrust    KnownGateway = "braintrust"
	GatewayDatabricks    KnownGateway = "databricks"
)

// GatewayFingerprints maps gateway types to their header prefixes
var GatewayFingerprints = map[KnownGateway][]string{
	GatewayLiteLLM:       {"x-litellm-"},
	GatewayHelicone:      {"helicone-"},
	GatewayPortkey:       {"x-portkey-"},
	GatewayCloudflareAIG: {"cf-aig-"},
	GatewayKong:          {"x-kong-"},
	GatewayBraintrust:    {"x-bt-"},
}

// GatewayHostSuffixes maps gateway types to their domain suffixes
var GatewayHostSuffixes = map[KnownGateway][]string{
	GatewayDatabricks: {
		".cloud.databricks.com",
		".azuredatabricks.net",
		".gcp.databricks.com",
	},
}

// DetectGateway detects the AI gateway from response headers or base URL
// Matches TypeScript detectGateway() in logging.ts:107-139
func DetectGateway(headers http.Header, baseURL string) KnownGateway {
	if headers != nil {
		for gateway, prefixes := range GatewayFingerprints {
			for _, prefix := range prefixes {
				for key := range headers {
					if strings.HasPrefix(strings.ToLower(key), prefix) {
						return gateway
					}
				}
			}
		}
	}

	if baseURL != "" {
		host := extractHost(baseURL)
		host = strings.ToLower(host)
		for gateway, suffixes := range GatewayHostSuffixes {
			for _, suffix := range suffixes {
				if strings.HasSuffix(host, suffix) {
					return gateway
				}
			}
		}
	}

	return ""
}

func extractHost(urlStr string) string {
	urlStr = strings.TrimSpace(urlStr)
	if urlStr == "" {
		return ""
	}
	if strings.HasPrefix(urlStr, "https://") {
		urlStr = strings.TrimPrefix(urlStr, "https://")
	} else if strings.HasPrefix(urlStr, "http://") {
		urlStr = strings.TrimPrefix(urlStr, "http://")
	}
	if idx := strings.Index(urlStr, "/"); idx != -1 {
		urlStr = urlStr[:idx]
	}
	if idx := strings.Index(urlStr, ":"); idx != -1 {
		urlStr = urlStr[:idx]
	}
	return urlStr
}

// APIUsage represents token usage from an API response
type APIUsage struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	CacheReadTokens     int `json:"cache_read_input_tokens,omitempty"`
	CacheCreationTokens int `json:"cache_creation_input_tokens,omitempty"`
}

// EmptyUsage returns an empty usage struct
var EmptyUsage = APIUsage{}

// APIRequestLog represents data logged for an API request
type APIRequestLog struct {
	Model          string
	MessagesLength int
	Temperature    float64
	Betas          []string
	QuerySource    string
	ThinkingType   string
	FastMode       bool
}

// APIResponseLog represents data logged for an API response
type APIResponseLog struct {
	Model                     string
	PreNormalizedModel        string
	MessageCount              int
	MessageTokens             int
	Usage                     APIUsage
	DurationMs                int64
	DurationIncludingRetries  int64
	Attempt                   int
	TTFTMs                    int64
	RequestID                 string
	StopReason                string
	CostUSD                   float64
	DidFallbackToNonStreaming bool
	QuerySource               string
	Gateway                   KnownGateway
	Provider                  APIProvider
	FastMode                  bool
}

// APIErrorLog represents data logged for an API error
type APIErrorLog struct {
	Model                     string
	Error                     string
	Status                    string
	ErrorType                 string
	MessageCount              int
	MessageTokens             int
	DurationMs                int64
	DurationIncludingRetries  int64
	Attempt                   int
	RequestID                 string
	ClientRequestID           string
	DidFallbackToNonStreaming bool
	Gateway                   KnownGateway
	QuerySource               string
	FastMode                  bool
}

// Logger interface for API logging
type Logger interface {
	LogAPIQuery(log APIRequestLog)
	LogAPIError(log APIErrorLog)
	LogAPISuccess(log APIResponseLog)
	LogDebug(message string)
	LogDebugError(message string)
}

// DefaultLogger is a no-op logger
type DefaultLogger struct{}

func (l *DefaultLogger) LogAPIQuery(_ APIRequestLog)    {}
func (l *DefaultLogger) LogAPIError(_ APIErrorLog)      {}
func (l *DefaultLogger) LogAPISuccess(_ APIResponseLog) {}
func (l *DefaultLogger) LogDebug(_ string)              {}
func (l *DefaultLogger) LogDebugError(_ string)         {}

var globalLogger Logger = &DefaultLogger{}

// SetLogger sets the global API logger
func SetLogger(logger Logger) { globalLogger = logger }

// GetLogger returns the global API logger
func GetLogger() Logger { return globalLogger }

// LogAPIQuery logs an API query request
func LogAPIQuery(log APIRequestLog) { globalLogger.LogAPIQuery(log) }

// LogAPIError logs an API error
func LogAPIError(log APIErrorLog) { globalLogger.LogAPIError(log) }

// LogAPISuccess logs a successful API response
func LogAPISuccess(log APIResponseLog) { globalLogger.LogAPISuccess(log) }

// LogDebug logs a debug message
func LogDebug(message string) { globalLogger.LogDebug(message) }

// LogDebugError logs a debug error message
func LogDebugError(message string) { globalLogger.LogDebugError(message) }

// UpdateUsage merges usage deltas
func UpdateUsage(current, delta APIUsage) APIUsage {
	return APIUsage{
		InputTokens:         current.InputTokens + delta.InputTokens,
		OutputTokens:        current.OutputTokens + delta.OutputTokens,
		CacheReadTokens:     current.CacheReadTokens + delta.CacheReadTokens,
		CacheCreationTokens: current.CacheCreationTokens + delta.CacheCreationTokens,
	}
}

// CalculateCost calculates the cost in USD based on usage
func CalculateCost(usage APIUsage, model string) float64 {
	inputPrice := 3.0
	outputPrice := 15.0
	cacheReadPrice := 0.3
	cacheWritePrice := 3.75

	modelLower := strings.ToLower(model)
	if strings.Contains(modelLower, "opus") {
		inputPrice = 15.0
		outputPrice = 75.0
		cacheReadPrice = 1.5
		cacheWritePrice = 18.75
	} else if strings.Contains(modelLower, "haiku") {
		inputPrice = 0.25
		outputPrice = 1.25
		cacheReadPrice = 0.03
		cacheWritePrice = 0.30
	}

	inputCost := float64(usage.InputTokens) / 1_000_000 * inputPrice
	outputCost := float64(usage.OutputTokens) / 1_000_000 * outputPrice
	cacheReadCost := float64(usage.CacheReadTokens) / 1_000_000 * cacheReadPrice
	cacheWriteCost := float64(usage.CacheCreationTokens) / 1_000_000 * cacheWritePrice

	return inputCost + outputCost + cacheReadCost + cacheWriteCost
}

// FormatDuration formats a duration in milliseconds
func FormatDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	if ms < 60000 {
		return fmt.Sprintf("%.1fs", float64(ms)/1000)
	}
	return fmt.Sprintf("%.1fm", float64(ms)/60000)
}

// APIMetrics tracks cumulative API metrics
type APIMetrics struct {
	TotalRequests     int64
	TotalErrors       int64
	TotalTokens       int64
	TotalInputTokens  int64
	TotalOutputTokens int64
	TotalCost         float64
	TotalDuration     time.Duration
	mu                chan struct{}
}

// APIMetrics creates a new metrics tracker
func CreateAPIMetrics() *APIMetrics {
	return &APIMetrics{mu: make(chan struct{}, 1)}
}

// RecordRequest records a request
func (m *APIMetrics) RecordRequest() {
	<-m.mu
	m.TotalRequests++
	m.mu <- struct{}{}
}

// RecordError records an error
func (m *APIMetrics) RecordError() {
	<-m.mu
	m.TotalErrors++
	m.mu <- struct{}{}
}

// RecordUsage records usage from a response
func (m *APIMetrics) RecordUsage(usage APIUsage, cost float64, duration time.Duration) {
	<-m.mu
	m.TotalTokens += int64(usage.InputTokens + usage.OutputTokens)
	m.TotalInputTokens += int64(usage.InputTokens)
	m.TotalOutputTokens += int64(usage.OutputTokens)
	m.TotalCost += cost
	m.TotalDuration += duration
	m.mu <- struct{}{}
}

// GetMetrics returns current metrics
func (m *APIMetrics) GetMetrics() (requests, errors, tokens int64, cost float64, duration time.Duration) {
	<-m.mu
	requests = m.TotalRequests
	errors = m.TotalErrors
	tokens = m.TotalTokens
	cost = m.TotalCost
	duration = m.TotalDuration
	m.mu <- struct{}{}
	return
}
