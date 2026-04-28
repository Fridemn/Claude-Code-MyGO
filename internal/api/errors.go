package api

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// Error types
const (
	ErrorTypeInvalidRequest = "invalid_request_error"
	ErrorTypeAuthentication = "authentication_error"
	ErrorTypePermission     = "permission_error"
	ErrorTypeNotFound       = "not_found_error"
	ErrorTypeRateLimit      = "rate_limit_error"
	ErrorTypeServerError    = "server_error"
	ErrorTypeOverloaded     = "overloaded_error"
	ErrorTypeContentFilter  = "content_filter"
	ErrorTypeLength         = "length_error"
	ErrorTypeConnection     = "connection_error"
	ErrorTypeTimeout        = "timeout_error"
	ErrorTypeSSL            = "ssl_error"
)

// Error message constants (user-facing)
const (
	APIErrorPrefix                    = "API Error"
	PromptTooLongMessage              = "Prompt is too long"
	CreditBalanceTooLowMessage        = "Credit balance is too low"
	InvalidAPIKeyMessage              = "Not logged in · Please run /login"
	InvalidAPIKeyExternalMessage      = "Invalid API key · Fix external API key"
	OrgDisabledEnvKeyWithOAuthMessage = "Your ANTHROPIC_API_KEY belongs to a disabled organization · Unset the environment variable to use your subscription instead"
	OrgDisabledEnvKeyMessage          = "Your ANTHROPIC_API_KEY belongs to a disabled organization · Update or unset the environment variable"
	TokenRevokedMessage               = "OAuth token revoked · Please run /login"
	CCRAuthMessage                    = "Authentication error · This may be a temporary network issue, please try again"
	Repeated529Message                = "Repeated 529 Overloaded errors"
	APITimeoutMessage                 = "Request timed out"
	OAuthOrgNotAllowedMessage         = "Your account does not have access to Claude Code. Please run /login."
)

// APIErrorClassification represents a classification of API errors for analytics
type APIErrorClassification string

const (
	ClassificationAborted              APIErrorClassification = "aborted"
	ClassificationAPITimeout           APIErrorClassification = "api_timeout"
	ClassificationRepeated529          APIErrorClassification = "repeated_529"
	ClassificationCapacityOffSwitch    APIErrorClassification = "capacity_off_switch"
	ClassificationRateLimit            APIErrorClassification = "rate_limit"
	ClassificationServerOverload       APIErrorClassification = "server_overload"
	ClassificationPromptTooLong        APIErrorClassification = "prompt_too_long"
	ClassificationPDFTooLarge          APIErrorClassification = "pdf_too_large"
	ClassificationPDFPasswordProtected APIErrorClassification = "pdf_password_protected"
	ClassificationImageTooLarge        APIErrorClassification = "image_too_large"
	ClassificationToolUseMismatch      APIErrorClassification = "tool_use_mismatch"
	ClassificationUnexpectedToolResult APIErrorClassification = "unexpected_tool_result"
	ClassificationDuplicateToolUseID   APIErrorClassification = "duplicate_tool_use_id"
	ClassificationInvalidModel         APIErrorClassification = "invalid_model"
	ClassificationCreditBalanceLow     APIErrorClassification = "credit_balance_low"
	ClassificationInvalidAPIKey        APIErrorClassification = "invalid_api_key"
	ClassificationTokenRevoked         APIErrorClassification = "token_revoked"
	ClassificationOAuthOrgNotAllowed   APIErrorClassification = "oauth_org_not_allowed"
	ClassificationAuthError            APIErrorClassification = "auth_error"
	ClassificationBedrockModelAccess   APIErrorClassification = "bedrock_model_access"
	ClassificationServerError          APIErrorClassification = "server_error"
	ClassificationClientError          APIErrorClassification = "client_error"
	ClassificationSSLCertError         APIErrorClassification = "ssl_cert_error"
	ClassificationConnectionError      APIErrorClassification = "connection_error"
	ClassificationUnknown              APIErrorClassification = "unknown"
)

// SSLErrorCodes are OpenSSL/TLS error codes that indicate certificate issues
var SSLErrorCodes = map[string]bool{
	// Certificate verification errors
	"UNABLE_TO_VERIFY_LEAF_SIGNATURE":   true,
	"UNABLE_TO_GET_ISSUER_CERT":         true,
	"UNABLE_TO_GET_ISSUER_CERT_LOCALLY": true,
	"CERT_SIGNATURE_FAILURE":            true,
	"CERT_NOT_YET_VALID":                true,
	"CERT_HAS_EXPIRED":                  true,
	"CERT_REVOKED":                      true,
	"CERT_REJECTED":                     true,
	"CERT_UNTRUSTED":                    true,
	// Self-signed certificate errors
	"DEPTH_ZERO_SELF_SIGNED_CERT": true,
	"SELF_SIGNED_CERT_IN_CHAIN":   true,
	// Chain errors
	"CERT_CHAIN_TOO_LONG":  true,
	"PATH_LENGTH_EXCEEDED": true,
	// Hostname/altname errors
	"ERR_TLS_CERT_ALTNAME_INVALID": true,
	"HOSTNAME_MISMATCH":            true,
	// TLS handshake errors
	"ERR_TLS_HANDSHAKE_TIMEOUT":                   true,
	"ERR_SSL_WRONG_VERSION_NUMBER":                true,
	"ERR_SSL_DECRYPTION_FAILED_OR_BAD_RECORD_MAC": true,
	// Go-specific TLS errors
	"tls: handshake failure":             true,
	"tls: certificate is not valid":      true,
	"tls: unknown certificate authority": true,
	"tls: certificate has expired":       true,
	"tls: certificate is not yet valid":  true,
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error *ErrorDetail `json:"error"`
}

// ErrorDetail contains error details
type ErrorDetail struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
	Param   string `json:"param,omitempty"`
}

// APIError represents an API error with full context
type APIError struct {
	StatusCode int
	Type       string
	Message    string
	Code       string
	Cause      error
	Headers    map[string]string
	RetryAfter time.Duration
	RequestID  string
}

func (e *APIError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("API error %d: %s (caused by: %v)", e.StatusCode, e.Message, e.Cause)
	}
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Message)
}

// Unwrap returns the underlying cause
func (e *APIError) Unwrap() error {
	return e.Cause
}

// IsRateLimitError checks if error is a rate limit error
func IsRateLimitError(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 429
	}
	return false
}

// IsAuthenticationError checks if error is an authentication error
func IsAuthenticationError(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 401 || apiErr.StatusCode == 403
	}
	return false
}

// IsPermissionError checks if error is a permission error
func IsPermissionError(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 403
	}
	return false
}

// IsNotFoundError checks if error is a not found error
func IsNotFoundError(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 404
	}
	return false
}

// IsServerError checks if error is a server error (5xx)
func IsServerError(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode >= 500
	}
	return false
}

// IsOverloadedError checks if error is an overloaded/server busy error
func IsOverloadedError(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 529 ||
			strings.Contains(strings.ToLower(apiErr.Message), "overloaded")
	}
	return false
}

// IsTimeoutError checks if error is a timeout error
func IsTimeoutError(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode == 408 || apiErr.Type == ErrorTypeTimeout {
			return true
		}
	}
	// Check for net/url timeout
	var netErr *url.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}
	// Check for TLS handshake timeout
	var tlsCertErr *tls.CertificateVerificationError
	if errors.As(err, &tlsCertErr) {
		return true
	}
	// Check for custom timeout errors (like StreamIdleTimeoutError)
	// Using interface check to avoid circular imports
	if isCustomTimeoutError(err) {
		return true
	}
	return false
}

// TimeoutError is an interface for custom timeout errors
type TimeoutError interface {
	IsTimeoutError() bool
}

// isCustomTimeoutError checks if an error implements the TimeoutError interface
func isCustomTimeoutError(err error) bool {
	if te, ok := err.(TimeoutError); ok {
		return te.IsTimeoutError()
	}
	return false
}

// IsContentFilterError checks if error is a content filter error
func IsContentFilterError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return strings.Contains(strings.ToLower(apiErr.Message), "content filter") ||
			apiErr.Type == ErrorTypeContentFilter
	}
	return false
}

func isPromptTooLongMessageText(message string) bool {
	lower := strings.ToLower(message)
	return strings.Contains(lower, "prompt is too long") ||
		strings.Contains(lower, "prompt exceeds max length") ||
		strings.Contains(lower, "input characters limit") ||
		strings.Contains(lower, "input character limit")
}

// IsLengthError checks if error is due to max tokens
func IsLengthError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.Code == "length" ||
			strings.Contains(strings.ToLower(apiErr.Message), "maximum context length") ||
			isPromptTooLongMessageText(apiErr.Message)
	}
	return false
}

// ContextOverflowError represents an error when input tokens exceed context window
type ContextOverflowError struct {
	InputTokens  int
	MaxTokens    int
	ContextLimit int
	Message      string
}

func (e *ContextOverflowError) Error() string {
	return e.Message
}

// ParseContextOverflowError parses an API error for context overflow details
// Matches TypeScript: parseMaxTokensContextOverflowError
// Example error message: "input length and `max_tokens` exceed context limit: 188059 + 20000 > 200000"
func ParseContextOverflowError(err error) *ContextOverflowError {
	apiErr, ok := err.(*APIError)
	if !ok {
		return nil
	}

	// Must be a 400 error with context overflow message
	if apiErr.StatusCode != 400 {
		return nil
	}

	msg := apiErr.Message

	// Check for context overflow error patterns
	if !strings.Contains(msg, "exceed context limit") &&
		!strings.Contains(msg, "input token limit") &&
		!strings.Contains(msg, "context_window_exceeded") {
		return nil
	}

	// Try to parse the detailed format: "input length and `max_tokens` exceed context limit: 188059 + 20000 > 200000"
	// or: "input token limit is 202752" (simpler format)
	patterns := []string{
		`input length and .max_tokens. exceed context limit: (\d+) \+ (\d+) > (\d+)`,
		`input length exceeds context limit: (\d+) \+ (\d+) > (\d+)`,
		`input token limit is (\d+)`,
	}

	for _, pattern := range patterns {
		// Simple regex simulation since we don't want to import regexp
		result := parseOverflowMatch(msg, pattern)
		if result != nil {
			return result
		}
	}

	// If we found the error but couldn't parse details, return a generic error
	return &ContextOverflowError{
		Message: msg,
	}
}

// parseOverflowMatch attempts to match overflow error patterns
func parseOverflowMatch(msg, pattern string) *ContextOverflowError {
	// Pattern 1: "input length and `max_tokens` exceed context limit: X + Y > Z"
	if strings.Contains(pattern, "exceed context limit:") {
		// Try the detailed format first
		if idx := strings.Index(msg, "exceed context limit:"); idx != -1 {
			rest := msg[idx+len("exceed context limit:"):]
			rest = strings.TrimSpace(rest)

			// Parse "188059 + 20000 > 200000"
			var inputTokens, maxTokens, contextLimit int
			var n int
			// Try to parse using simple string parsing
			parts := strings.Fields(rest)
			for _, part := range parts {
				if part == "+" || part == ">" {
					continue
				}
				if n == 0 {
					fmt.Sscanf(part, "%d", &inputTokens)
					n++
				} else if n == 1 {
					fmt.Sscanf(part, "%d", &maxTokens)
					n++
				} else if n == 2 {
					fmt.Sscanf(part, "%d", &contextLimit)
					break
				}
			}

			if inputTokens > 0 && contextLimit > 0 {
				return &ContextOverflowError{
					InputTokens:  inputTokens,
					MaxTokens:    maxTokens,
					ContextLimit: contextLimit,
					Message:      msg,
				}
			}
		}
	}

	// Pattern 2: "input token limit is X"
	if strings.Contains(pattern, "input token limit is") {
		if idx := strings.Index(msg, "input token limit is"); idx != -1 {
			rest := msg[idx+len("input token limit is"):]
			rest = strings.TrimSpace(rest)
			var limit int
			if n, _ := fmt.Sscanf(rest, "%d", &limit); n == 1 && limit > 0 {
				return &ContextOverflowError{
					InputTokens:  limit, // This is actually the limit, not input tokens
					ContextLimit: limit,
					Message:      msg,
				}
			}
		}
	}

	return nil
}

// IsContextOverflowError checks if error is a context overflow error
func IsContextOverflowError(err error) bool {
	return ParseContextOverflowError(err) != nil
}

// CalculateAdjustedMaxTokens calculates a safe max_tokens to avoid context overflow
// Matches TypeScript logic for FLOOR_OUTPUT_TOKENS and safety buffer
func CalculateAdjustedMaxTokens(overflow *ContextOverflowError, thinkingBudgetTokens int) int {
	const floorOutputTokens = 3000
	const safetyBuffer = 1000

	availableContext := overflow.ContextLimit - overflow.InputTokens - safetyBuffer
	if availableContext < floorOutputTokens {
		availableContext = floorOutputTokens
	}

	// Ensure enough tokens for thinking + at least 1 output token
	minRequired := thinkingBudgetTokens + 1

	adjusted := availableContext
	if adjusted < floorOutputTokens {
		adjusted = floorOutputTokens
	}
	if adjusted < minRequired {
		adjusted = minRequired
	}

	return adjusted
}

// IsConnectionError checks if error is a connection error
func IsConnectionError(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		if apiErr.Type == ErrorTypeConnection {
			return true
		}
	}
	// Check for net errors
	var netOpErr *net.OpError
	if errors.As(err, &netOpErr) {
		return true
	}
	// Check for TLS errors
	var tlsCertErr *tls.CertificateVerificationError
	if errors.As(err, &tlsCertErr) {
		return true
	}
	return false
}

// IsSSLError checks if error is an SSL/TLS error
func IsSSLError(err error) bool {
	// Check API error type
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		if apiErr.Type == ErrorTypeSSL {
			return true
		}
	}

	// Check for TLS certificate verification errors
	var tlsCertErr *tls.CertificateVerificationError
	if errors.As(err, &tlsCertErr) {
		return true
	}

	// Check error message for known SSL error patterns
	errStr := strings.ToLower(err.Error())
	for code := range SSLErrorCodes {
		if strings.Contains(errStr, strings.ToLower(code)) {
			return true
		}
	}

	// Check for common TLS error messages
	tlsPatterns := []string{
		"tls: handshake failure",
		"tls: certificate",
		"x509:",
		"ssl",
		"certificate",
	}
	for _, pattern := range tlsPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// ConnectionErrorDetails contains extracted connection error information
type ConnectionErrorDetails struct {
	Code      string
	Message   string
	IsSSL     bool
	IsTimeout bool
}

// ExtractConnectionErrorDetails extracts details from connection errors
func ExtractConnectionErrorDetails(err error) *ConnectionErrorDetails {
	if err == nil {
		return nil
	}

	details := &ConnectionErrorDetails{
		Message: err.Error(),
	}

	// Check for net.OpError
	if netErr, ok := err.(*net.OpError); ok {
		details.Message = netErr.Error()
		if netErr.Err != nil {
			// Check for specific error codes
			errStr := netErr.Err.Error()
			switch {
			case strings.Contains(errStr, "timeout"):
				details.IsTimeout = true
				details.Code = "ETIMEDOUT"
			case strings.Contains(errStr, "connection refused"):
				details.Code = "ECONNREFUSED"
			case strings.Contains(errStr, "connection reset"):
				details.Code = "ECONNRESET"
			case strings.Contains(errStr, "broken pipe"):
				details.Code = "EPIPE"
			}
		}
	}

	// Check for TLS errors
	if IsSSLError(err) {
		details.IsSSL = true
		details.Code = "SSL_ERROR"
	}

	// Check for timeout
	if IsTimeoutError(err) {
		details.IsTimeout = true
		if details.Code == "" {
			details.Code = "TIMEOUT"
		}
	}

	// Check error string for known codes
	errStr := strings.ToLower(err.Error())
	switch {
	case strings.Contains(errStr, "tls handshake timeout"):
		details.IsSSL = true
		details.IsTimeout = true
		details.Code = "ERR_TLS_HANDSHAKE_TIMEOUT"
	case strings.Contains(errStr, "connection reset"):
		details.Code = "ECONNRESET"
	case strings.Contains(errStr, "broken pipe"):
		details.Code = "EPIPE"
	case strings.Contains(errStr, "timeout"):
		details.IsTimeout = true
		if details.Code == "" {
			details.Code = "ETIMEDOUT"
		}
	}

	return details
}

// IsStaleConnectionError checks if error indicates a stale connection
func IsStaleConnectionError(err error) bool {
	details := ExtractConnectionErrorDetails(err)
	if details == nil {
		return false
	}
	return details.Code == "ECONNRESET" || details.Code == "EPIPE"
}

// ParseError parses an error response body
func ParseError(statusCode int, body []byte) error {
	apiErr := &APIError{
		StatusCode: statusCode,
		Message:    strings.TrimSpace(string(body)),
		Headers:    make(map[string]string),
	}

	var errResp ErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != nil {
		apiErr.Message = errResp.Error.Message
		apiErr.Type = errResp.Error.Type
		apiErr.Code = errResp.Error.Code
	}

	return apiErr
}

// ParseErrorWithHeaders parses an error response with headers
func ParseErrorWithHeaders(statusCode int, body []byte, headers map[string]string) error {
	apiErr := ParseError(statusCode, body).(*APIError)
	apiErr.Headers = headers

	// Extract Retry-After header
	if retryAfter, ok := headers["Retry-After"]; ok {
		if seconds := parseRetryAfter(retryAfter); seconds > 0 {
			apiErr.RetryAfter = time.Duration(seconds) * time.Second
		}
	}

	// Extract request ID
	if reqID, ok := headers["X-Request-Id"]; ok {
		apiErr.RequestID = reqID
	}
	if reqID, ok := headers["Request-Id"]; ok {
		apiErr.RequestID = reqID
	}

	return apiErr
}

// parseRetryAfter parses Retry-After header value
func parseRetryAfter(value string) int {
	// Try parsing as seconds
	seconds := 0
	if _, err := fmt.Sscanf(value, "%d", &seconds); err == nil && seconds > 0 {
		return seconds
	}

	// Try parsing as HTTP date
	if t, err := time.Parse(time.RFC1123, value); err == nil {
		delta := time.Until(t)
		if delta > 0 {
			return int(delta.Seconds())
		}
	}

	return 0
}

// WrapError wraps an error with context
func WrapError(err error, context string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", context, err)
}

// WrapAsAPIError wraps an error as an APIError
func WrapAsAPIError(err error, statusCode int, errorType string) *APIError {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr
	}

	apiErr := &APIError{
		StatusCode: statusCode,
		Type:       errorType,
		Message:    err.Error(),
		Cause:      err,
		Headers:    make(map[string]string),
	}

	// Extract connection details
	if details := ExtractConnectionErrorDetails(err); details != nil {
		if details.IsSSL {
			apiErr.Type = ErrorTypeSSL
		} else if details.IsTimeout {
			apiErr.Type = ErrorTypeTimeout
		} else {
			apiErr.Type = ErrorTypeConnection
		}
	}

	return apiErr
}

// GetRetryAfter extracts retry-after duration from an error (from header only)
func GetRetryAfter(err error) time.Duration {
	if apiErr, ok := err.(*APIError); ok {
		// Only use the actual Retry-After header value, not a default
		if apiErr.RetryAfter > 0 {
			return apiErr.RetryAfter
		}
	}
	return 0
}

// ErrorMessage returns a user-friendly error message
func ErrorMessage(err error) string {
	if err == nil {
		return ""
	}

	if apiErr, ok := err.(*APIError); ok {
		return formatAPIErrorMessage(apiErr)
	}

	// Check for connection errors
	if details := ExtractConnectionErrorDetails(err); details != nil {
		return formatConnectionErrorMessage(details)
	}

	return err.Error()
}

// formatAPIErrorMessage formats an API error message
func formatAPIErrorMessage(apiErr *APIError) string {
	switch apiErr.StatusCode {
	case 401:
		return "Invalid API key. Please check your configuration."
	case 403:
		return "Permission denied. Your API key may not have access to this resource."
	case 404:
		return "Resource not found. The model or endpoint may not exist."
	case 408:
		return "Request timed out. Please try again."
	case 429:
		return "Rate limit exceeded. Please wait a moment and try again."
	case 500, 502, 503, 504:
		return "Server error. The API is temporarily unavailable."
	case 529:
		return "Server is overloaded. Please wait a moment and try again."
	}

	if apiErr.Message != "" {
		return apiErr.Message
	}

	return fmt.Sprintf("API error (status %d)", apiErr.StatusCode)
}

// formatConnectionErrorMessage formats a connection error message
func formatConnectionErrorMessage(details *ConnectionErrorDetails) string {
	switch details.Code {
	case "ETIMEDOUT", "TIMEOUT":
		return "Request timed out. Check your internet connection and proxy settings."
	case "ECONNREFUSED":
		return "Connection refused. The server may be down or unreachable."
	case "ECONNRESET":
		return "Connection reset by server. Please try again."
	case "EPIPE":
		return "Connection broken. Please try again."
	}

	if details.IsSSL {
		return formatSSLErrorMessage(details)
	}

	if details.IsTimeout {
		return "Request timed out. Check your internet connection."
	}

	return fmt.Sprintf("Connection error: %s", details.Message)
}

// formatSSLErrorMessage formats an SSL/TLS error message
func formatSSLErrorMessage(details *ConnectionErrorDetails) string {
	switch details.Code {
	case "UNABLE_TO_VERIFY_LEAF_SIGNATURE", "UNABLE_TO_GET_ISSUER_CERT", "UNABLE_TO_GET_ISSUER_CERT_LOCALLY":
		return "Unable to connect to API: SSL certificate verification failed. Check your proxy or corporate SSL certificates."
	case "CERT_HAS_EXPIRED":
		return "Unable to connect to API: SSL certificate has expired."
	case "CERT_REVOKED":
		return "Unable to connect to API: SSL certificate has been revoked."
	case "DEPTH_ZERO_SELF_SIGNED_CERT", "SELF_SIGNED_CERT_IN_CHAIN":
		return "Unable to connect to API: Self-signed certificate detected. Check your proxy or corporate SSL certificates."
	case "ERR_TLS_CERT_ALTNAME_INVALID", "HOSTNAME_MISMATCH":
		return "Unable to connect to API: SSL certificate hostname mismatch."
	case "CERT_NOT_YET_VALID":
		return "Unable to connect to API: SSL certificate is not yet valid."
	case "ERR_TLS_HANDSHAKE_TIMEOUT":
		return "TLS handshake timed out. Check your network connection and proxy settings."
	default:
		return fmt.Sprintf("SSL certificate error (%s). If you are behind a corporate proxy or TLS-intercepting firewall, check your CA certificates.", details.Code)
	}
}

// SSLErrorHint returns a hint for SSL errors
func SSLErrorHint(err error) string {
	if !IsSSLError(err) {
		return ""
	}
	return "If you are behind a corporate proxy or TLS-intercepting firewall, set NODE_EXTRA_CA_CERTS to your CA bundle path, or ask IT to allowlist the API endpoint."
}

// SanitizeMessage removes HTML content from error messages
func SanitizeMessage(message string) string {
	if strings.Contains(message, "<!DOCTYPE html") || strings.Contains(message, "<html") {
		// Try to extract title
		if idx := strings.Index(message, "<title>"); idx != -1 {
			endIdx := strings.Index(message[idx:], "</title>")
			if endIdx != -1 {
				return strings.TrimSpace(message[idx+7 : idx+endIdx])
			}
		}
		return ""
	}
	return message
}

// PromptTooLongTokenCounts holds parsed token counts from a prompt-too-long error
type PromptTooLongTokenCounts struct {
	ActualTokens int
	LimitTokens  int
	HasValues    bool
}

// ParsePromptTooLongTokenCounts parses actual/limit token counts from a raw
// prompt-too-long API error message like "prompt is too long: 137500 tokens > 135000 maximum".
func ParsePromptTooLongTokenCounts(rawMessage string) PromptTooLongTokenCounts {
	// Pattern: "prompt is too long... X tokens > Y maximum"
	lower := strings.ToLower(rawMessage)
	if !strings.Contains(lower, "prompt is too long") {
		return PromptTooLongTokenCounts{}
	}

	// Find "tokens >" pattern
	tokensIdx := strings.Index(lower, "tokens >")
	if tokensIdx == -1 {
		return PromptTooLongTokenCounts{}
	}

	// Find the two numbers before "tokens >" and after ">"
	// Look for "X tokens > Y maximum" pattern
	beforeTokens := lower[:tokensIdx]
	afterTokens := lower[tokensIdx+len("tokens >"):]

	// Find the actual token count (the number right before "tokens")
	// Scan backwards from "tokens" to find the number
	actualStr := ""
	for i := len(beforeTokens) - 1; i >= 0; i-- {
		c := beforeTokens[i]
		if c >= '0' && c <= '9' {
			// Build number in reverse
			actualStr = string(c) + actualStr
		} else if actualStr != "" {
			// End of number
			break
		}
	}
	if actualStr == "" {
		return PromptTooLongTokenCounts{}
	}

	// Find the limit token count (the number right after ">")
	limitStr := ""
	for i := 0; i < len(afterTokens); i++ {
		c := afterTokens[i]
		if c >= '0' && c <= '9' {
			limitStr += string(c)
		} else if limitStr != "" {
			// End of number
			break
		}
	}
	if limitStr == "" {
		return PromptTooLongTokenCounts{}
	}

	var actual, limit int
	_, _ = fmt.Sscanf(actualStr, "%d", &actual)
	_, _ = fmt.Sscanf(limitStr, "%d", &limit)

	if actual > 0 && limit > 0 {
		return PromptTooLongTokenCounts{
			ActualTokens: actual,
			LimitTokens:  limit,
			HasValues:    true,
		}
	}

	return PromptTooLongTokenCounts{}
}

// GetPromptTooLongTokenGap returns how many tokens over the limit a prompt-too-long error reports
func GetPromptTooLongTokenGap(rawError string) int {
	counts := ParsePromptTooLongTokenCounts(rawError)
	if !counts.HasValues {
		return 0
	}
	gap := counts.ActualTokens - counts.LimitTokens
	if gap > 0 {
		return gap
	}
	return 0
}

// IsMediaSizeError checks if raw error text is a media-size rejection
func IsMediaSizeError(raw string) bool {
	lower := strings.ToLower(raw)
	hasImageExceeds := strings.Contains(lower, "image exceeds") && strings.Contains(lower, "maximum")
	hasImageDimensions := strings.Contains(lower, "image dimensions exceed") && strings.Contains(lower, "many-image")
	hasPDFPages := strings.Contains(lower, "maximum of") && strings.Contains(lower, "pdf pages")
	return hasImageExceeds || hasImageDimensions || hasPDFPages
}

// Is529Error checks if error is a 529 overloaded error
func Is529Error(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	// Check for 529 status code or overloaded error in message
	return apiErr.StatusCode == 529 ||
		strings.Contains(apiErr.Message, `"type":"overloaded_error"`)
}

// IsOAuthTokenRevokedError checks if error indicates OAuth token was revoked
func IsOAuthTokenRevokedError(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.StatusCode == 403 &&
		strings.Contains(apiErr.Message, "OAuth token has been revoked")
}

// IsFastModeNotEnabledError checks if error indicates fast mode is not enabled
func IsFastModeNotEnabledError(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.StatusCode == 400 &&
		strings.Contains(apiErr.Message, "Fast mode is not enabled")
}

// PromptTooLongError represents a prompt too long error with parsed details
type PromptTooLongError struct {
	ActualTokens int
	LimitTokens  int
	RawMessage   string
}

func (e *PromptTooLongError) Error() string {
	return fmt.Sprintf("prompt is too long: %d tokens > %d maximum", e.ActualTokens, e.LimitTokens)
}

// ParsePromptTooLongError extracts token counts from a prompt-too-long error
func ParsePromptTooLongError(err error) *PromptTooLongError {
	if err == nil {
		return nil
	}

	errMsg := err.Error()
	counts := ParsePromptTooLongTokenCounts(errMsg)
	if !counts.HasValues {
		return nil
	}

	return &PromptTooLongError{
		ActualTokens: counts.ActualTokens,
		LimitTokens:  counts.LimitTokens,
		RawMessage:   errMsg,
	}
}

// ClassifyAPIError classifies an API error for analytics tracking
func ClassifyAPIError(err error) APIErrorClassification {
	if err == nil {
		return ClassificationUnknown
	}

	errMsg := err.Error()

	// Aborted requests
	if errMsg == "Request was aborted." {
		return ClassificationAborted
	}

	// Timeout errors
	if IsTimeoutError(err) {
		return ClassificationAPITimeout
	}

	// Repeated 529 errors
	if strings.Contains(errMsg, Repeated529Message) {
		return ClassificationRepeated529
	}

	// Rate limiting
	if IsRateLimitError(err) {
		return ClassificationRateLimit
	}

	// Server overload (529)
	if Is529Error(err) {
		return ClassificationServerOverload
	}

	// Prompt too long
	if strings.Contains(strings.ToLower(errMsg), strings.ToLower(PromptTooLongMessage)) || isPromptTooLongMessageText(errMsg) {
		return ClassificationPromptTooLong
	}

	// PDF errors
	if strings.Contains(errMsg, "maximum of") && strings.Contains(errMsg, "PDF pages") {
		return ClassificationPDFTooLarge
	}
	if strings.Contains(errMsg, "password protected") && strings.Contains(errMsg, "PDF") {
		return ClassificationPDFPasswordProtected
	}

	// Image size errors
	if IsMediaSizeError(errMsg) {
		return ClassificationImageTooLarge
	}

	// Tool use errors
	if strings.Contains(errMsg, "tool_use` ids were found without `tool_result`") {
		return ClassificationToolUseMismatch
	}
	if strings.Contains(errMsg, "unexpected `tool_use_id` found in `tool_result`") {
		return ClassificationUnexpectedToolResult
	}
	if strings.Contains(errMsg, "`tool_use` ids must be unique") {
		return ClassificationDuplicateToolUseID
	}

	// Invalid model errors
	if strings.Contains(strings.ToLower(errMsg), "invalid model name") {
		return ClassificationInvalidModel
	}

	// Credit/billing errors
	if strings.Contains(strings.ToLower(errMsg), strings.ToLower(CreditBalanceTooLowMessage)) {
		return ClassificationCreditBalanceLow
	}

	// Authentication errors
	if strings.Contains(strings.ToLower(errMsg), "x-api-key") {
		return ClassificationInvalidAPIKey
	}
	if IsOAuthTokenRevokedError(err) {
		return ClassificationTokenRevoked
	}
	if strings.Contains(errMsg, "OAuth authentication is currently not allowed for this organization") {
		return ClassificationOAuthOrgNotAllowed
	}
	if IsAuthenticationError(err) {
		return ClassificationAuthError
	}

	// Status code based fallbacks
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode >= 500 {
			return ClassificationServerError
		}
		if apiErr.StatusCode >= 400 {
			return ClassificationClientError
		}
	}

	// Connection errors - check for SSL/TLS issues first
	if IsSSLError(err) {
		return ClassificationSSLCertError
	}
	if IsConnectionError(err) {
		return ClassificationConnectionError
	}

	return ClassificationUnknown
}

// CategorizeRetryableAPIError categorizes an error for retry decisions
func CategorizeRetryableAPIError(err error) string {
	if Is529Error(err) {
		return "rate_limit"
	}
	if IsRateLimitError(err) {
		return "rate_limit"
	}
	if IsAuthenticationError(err) {
		return "authentication_failed"
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode >= 408 {
		return "server_error"
	}
	return "unknown"
}
