package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

func GenerateID(prefix string, bytes int) (string, error) {
	if bytes <= 0 {
		bytes = 8
	}

	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return prefix + hex.EncodeToString(buf), nil
}
