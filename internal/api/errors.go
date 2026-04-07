package api

import (
	"crypto/tls"
	"encoding/json"
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
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == 429
	}
	return false
}

// IsAuthenticationError checks if error is an authentication error
func IsAuthenticationError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == 401 || apiErr.StatusCode == 403
	}
	return false
}

// IsPermissionError checks if error is a permission error
func IsPermissionError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == 403
	}
	return false
}

// IsNotFoundError checks if error is a not found error
func IsNotFoundError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == 404
	}
	return false
}

// IsServerError checks if error is a server error (5xx)
func IsServerError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode >= 500
	}
	return false
}

// IsOverloadedError checks if error is an overloaded/server busy error
func IsOverloadedError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == 529 ||
			strings.Contains(strings.ToLower(apiErr.Message), "overloaded")
	}
	return false
}

// IsTimeoutError checks if error is a timeout error
func IsTimeoutError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == 408 || apiErr.Type == ErrorTypeTimeout
	}
	// Check for net/url timeout
	if netErr, ok := err.(*url.Error); ok {
		return netErr.Timeout()
	}
	// Check for TLS handshake timeout
	if _, ok := err.(*tls.CertificateVerificationError); ok {
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

// IsLengthError checks if error is due to max tokens
func IsLengthError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.Code == "length" ||
			strings.Contains(strings.ToLower(apiErr.Message), "maximum context length") ||
			strings.Contains(strings.ToLower(apiErr.Message), "prompt is too long")
	}
	return false
}

// IsConnectionError checks if error is a connection error
func IsConnectionError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.Type == ErrorTypeConnection
	}
	// Check for net errors
	if _, ok := err.(*net.OpError); ok {
		return true
	}
	// Check for TLS errors
	if _, ok := err.(*tls.CertificateVerificationError); ok {
		return true
	}
	return false
}

// IsSSLError checks if error is an SSL/TLS error
func IsSSLError(err error) bool {
	// Check API error type
	if apiErr, ok := err.(*APIError); ok {
		if apiErr.Type == ErrorTypeSSL {
			return true
		}
	}

	// Check for TLS certificate verification errors
	if _, ok := err.(*tls.CertificateVerificationError); ok {
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
