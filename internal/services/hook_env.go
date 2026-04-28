package services

import "strings"

// buildHookEnvVar formats key/value for hook process env and strips NUL bytes
// to avoid exec failures like "environment variable contains NUL".
func buildHookEnvVar(key, value string) string {
	return key + "=" + sanitizeHookEnvValue(value)
}

func sanitizeHookEnvValue(value string) string {
	return strings.ReplaceAll(value, "\x00", "")
}
