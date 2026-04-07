package utils

import (
	"math"
	"regexp"
	"strconv"
)

const MaxSanitizedLength = 200

// DJB2Hash computes the djb2 hash of a string.
// This is a fast non-cryptographic hash returning a signed 32-bit int.
// Deterministic across runtimes, suitable for cache directory names.
func DJB2Hash(str string) int32 {
	var hash int32 = 0
	for i := 0; i < len(str); i++ {
		hash = ((hash << 5) - hash + int32(str[i])) | 0
	}
	return hash
}

// SimpleHash returns the absolute djb2 hash value as a base36 string.
func SimpleHash(str string) string {
	hash := DJB2Hash(str)
	absHash := int32(math.Abs(float64(hash)))
	return strconv.FormatInt(int64(absHash), 36)
}

// SanitizePath makes a string safe for use as a directory or file name.
// Replaces all non-alphanumeric characters with hyphens.
// For paths that would exceed MaxSanitizedLength, truncates and appends a hash suffix.
func SanitizePath(name string) string {
	// Replace all non-alphanumeric characters with hyphens
	re := regexp.MustCompile(`[^a-zA-Z0-9]`)
	sanitized := re.ReplaceAllString(name, "-")

	if len(sanitized) <= MaxSanitizedLength {
		return sanitized
	}

	// Truncate and append hash suffix for uniqueness
	hash := SimpleHash(name)
	return sanitized[:MaxSanitizedLength] + "-" + hash
}