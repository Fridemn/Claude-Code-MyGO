// Package utils provides utility functions for the Claude Code CLI.
package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"time"
)

const PasteStoreDirName = "paste-cache"

// HashPastedText generates a SHA256 hash of pasted content for storage.
// Returns the first 16 hex characters, matching the TS hashPastedText().
// Ported from src/utils/pasteStore.ts:hashPastedText
func HashPastedText(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])[:16]
}

// GetPasteStoreDir returns the path to the paste cache directory.
// Ported from src/utils/pasteStore.ts:getPasteStoreDir
func GetPasteStoreDir() string {
	return filepath.Join(GetConfigDir(), PasteStoreDirName)
}

// GetPastePath returns the file path for a paste by its content hash.
// Ported from src/utils/pasteStore.ts:getPastePath
func GetPastePath(hash string) string {
	return filepath.Join(GetPasteStoreDir(), hash+".txt")
}

// StorePastedText stores pasted text content to disk.
// Fire-and-forget: errors are logged but not returned, matching TS behavior.
// Ported from src/utils/pasteStore.ts:storePastedText
func StorePastedText(hash string, content string) {
	go func() {
		dir := GetPasteStoreDir()
		if err := os.MkdirAll(dir, 0700); err != nil {
			return
		}

		pastePath := GetPastePath(hash)
		// Content-addressable: same hash = same content, so overwriting is safe
		if err := os.WriteFile(pastePath, []byte(content), 0600); err != nil {
			return
		}
	}()
}

// RetrievePastedText retrieves pasted text content by its hash.
// Returns nil if not found or on error.
// Ported from src/utils/pasteStore.ts:retrievePastedText
func RetrievePastedText(hash string) []byte {
	pastePath := GetPastePath(hash)
	data, err := os.ReadFile(pastePath)
	if err != nil {
		return nil
	}
	return data
}

// CleanupOldPastes removes paste files older than the given cutoff date.
// Ported from src/utils/pasteStore.ts:cleanupOldPastes
func CleanupOldPastes(cutoff time.Time) error {
	pasteDir := GetPasteStoreDir()

	entries, err := os.ReadDir(pasteDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if len(name) < 4 || name[len(name)-4:] != ".txt" {
			continue
		}

		filePath := filepath.Join(pasteDir, name)
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(filePath)
		}
	}
	return nil
}