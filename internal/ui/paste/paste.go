// Package paste provides functionality for handling pasted text in the input.
// When large text is pasted, it's displayed as a collapsed reference like
// "[Pasted text #1 +10 lines]" to keep the input clean.
package paste

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"claude-go/internal/utils"
)

// PasteThreshold is the character count threshold for treating input as a paste.
// Text longer than this will be collapsed into a pasted text reference.
// Ported from src/utils/imagePaste.ts:PASTE_THRESHOLD
const PasteThreshold = 800

// MaxPastedContentLength is the max inline content length for history storage.
// Pastes longer than this are stored to disk with a hash reference.
// Ported from src/history.ts:MAX_PASTED_CONTENT_LENGTH
const MaxPastedContentLength = 1024

// StoredPastedContent represents pasted content as stored in history.
// Either Content (inline, for small pastes) or ContentHash (disk reference, for large pastes).
// Ported from src/history.ts:StoredPastedContent
type StoredPastedContent struct {
	ID         int    `json:"id"`
	Type       string `json:"type"` // "text" or "image"
	Content    string `json:"content,omitempty"`
	ContentHash string `json:"contentHash,omitempty"`
	MediaType  string `json:"mediaType,omitempty"`
	Filename   string `json:"filename,omitempty"`
}

// PastedContent represents stored pasted content.
type PastedContent struct {
	ID      int    // Sequential numeric ID
	Type    string // "text" or "image"
	Content string // The actual content
}

// GetPastedTextRefNumLines counts the number of line breaks in text.
// This matches the original TS behavior where "line1\nline2\nline3" has +2 lines.
func GetPastedTextRefNumLines(text string) int {
	// Count line breaks (\r\n, \r, or \n)
	re := regexp.MustCompile(`\r\n|\r|\n`)
	return len(re.FindAllStringIndex(text, -1))
}

// FormatPastedTextRef formats a pasted text reference for display.
// Returns "[Pasted text #N +X lines]" or "[Pasted text #N]" if no lines.
func FormatPastedTextRef(id int, numLines int) string {
	if numLines == 0 {
		return formatRef("Pasted text", id, 0)
	}
	return formatRef("Pasted text", id, numLines)
}

// FormatImageRef formats an image reference for display.
func FormatImageRef(id int) string {
	return formatRef("Image", id, 0)
}

// FormatTruncatedTextRef formats a truncated text reference for display.
func FormatTruncatedTextRef(id int, numLines int) string {
	return formatRefTruncated(id, numLines)
}

func formatRef(prefix string, id int, numLines int) string {
	if numLines == 0 {
		return "[" + prefix + " #" + intToStr(id) + "]"
	}
	return "[" + prefix + " #" + intToStr(id) + " +" + intToStr(numLines) + " lines]"
}

func formatRefTruncated(id int, numLines int) string {
	if numLines == 0 {
		return "[...Truncated text #" + intToStr(id) + "...]"
	}
	return "[...Truncated text #" + intToStr(id) + " +" + intToStr(numLines) + " lines...]"
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	var result []byte
	negative := n < 0
	if negative {
		n = -n
	}
	for n > 0 {
		result = append([]byte{byte('0' + n%10)}, result...)
		n /= 10
	}
	if negative {
		result = append([]byte{'-'}, result...)
	}
	return string(result)
}

// Reference represents a parsed paste reference in the input.
type Reference struct {
	ID    int    // The numeric ID
	Match string // The full matched string
	Index int    // Position in the input
}

// referencePattern matches paste references like:
// - [Pasted text #1 +10 lines]
// - [Image #2]
// - [...Truncated text #3 +5 lines...]
var referencePattern = regexp.MustCompile(`\[(Pasted text|Image|\.\.\.Truncated text) #(\d+)(?: \+\d+ lines)?(\.)*\]`)

// ParseReferences finds all paste references in the input.
func ParseReferences(input string) []Reference {
	matches := referencePattern.FindAllStringSubmatchIndex(input, -1)
	if matches == nil {
		return nil
	}

	var refs []Reference
	for _, match := range matches {
		if len(match) < 6 {
			continue
		}
		// match[0] and match[1] are the full match start/end
		// match[2] and match[3] are the prefix (Pasted text/Image/etc)
		// match[4] and match[5] are the ID
		fullStart := match[0]
		fullEnd := match[1]
		idStart := match[4]
		idEnd := match[5]

		if idStart < 0 || idEnd < 0 {
			continue
		}

		idStr := input[idStart:idEnd]
		id := strToInt(idStr)
		if id <= 0 {
			continue
		}

		refs = append(refs, Reference{
			ID:    id,
			Match: input[fullStart:fullEnd],
			Index: fullStart,
		})
	}
	return refs
}

// ExpandPastedTextRefs replaces [Pasted text #N] placeholders with actual content.
// Image refs are left alone — they become content blocks, not inlined text.
func ExpandPastedTextRefs(input string, pastedContents map[int]*PastedContent) string {
	refs := ParseReferences(input)
	if len(refs) == 0 {
		return input
	}

	// Process in reverse order to keep earlier offsets valid after replacements
	expanded := input
	for i := len(refs) - 1; i >= 0; i-- {
		ref := refs[i]
		content := pastedContents[ref.ID]
		if content == nil || content.Type != "text" {
			continue
		}
		// Replace the reference with the actual content
		expanded = expanded[:ref.Index] + content.Content + expanded[ref.Index+len(ref.Match):]
	}
	return expanded
}

// Manager manages pasted content state.
// For small pastes (<=MaxPastedContentLength bytes), content is stored inline.
// For large pastes, content is stored to disk and referenced by hash.
// Ported from src/history.ts and src/ui/paste/paste.ts
type Manager struct {
	contents     map[int]*PastedContent
	nextID      int
	contentHashes map[int]string // Maps ID to content hash (for large pastes)
}

// NewManager creates a new paste manager.
func NewManager() *Manager {
	return &Manager{
		contents:      make(map[int]*PastedContent),
		nextID:       1,
		contentHashes: make(map[int]string),
	}
}

// AddPaste adds pasted text and returns the formatted reference.
// If the text is short (below threshold) and single-line, returns the text unchanged.
// For large pastes, stores to disk if over MaxPastedContentLength.
// Ported from src/history.ts:addToPromptHistory (paste storage logic)
func (m *Manager) AddPaste(text string) string {
	numLines := GetPastedTextRefNumLines(text)

	// Short text AND <= 2 lines: return as-is (not a paste)
	if len(text) <= PasteThreshold && numLines <= 2 {
		return text
	}

	// Create a new paste entry
	id := m.nextID
	m.nextID++

	m.contents[id] = &PastedContent{
		ID:      id,
		Type:    "text",
		Content: text,
	}

	// For large pastes, also store to disk (fire-and-forget)
	if len(text) > MaxPastedContentLength {
		m.storeToDisk(id, text)
	}

	return FormatPastedTextRef(id, numLines)
}

// storeToDisk stores large paste content to disk using content-addressable hash.
func (m *Manager) storeToDisk(id int, content string) {
	hash := contentHash(content)
	m.contentHashes[id] = hash
	// Fire-and-forget disk write
	go func() {
		dir := getDiskPasteDir()
		if err := mkdirAll(dir); err != nil {
			return
		}
		writeFile(getDiskPastePath(dir, hash), content)
	}()
}

// AddPasteWithThreshold adds pasted text with a custom threshold.
func (m *Manager) AddPasteWithThreshold(text string, threshold int) string {
	if threshold <= 0 {
		threshold = PasteThreshold
	}
	if len(text) <= threshold {
		return text
	}
	return m.AddPaste(text)
}

// GetContent retrieves pasted content by ID.
func (m *Manager) GetContent(id int) *PastedContent {
	return m.contents[id]
}

// GetAllContents returns all pasted contents.
func (m *Manager) GetAllContents() map[int]*PastedContent {
	result := make(map[int]*PastedContent, len(m.contents))
	for k, v := range m.contents {
		result[k] = v
	}
	return result
}

// GetStoredContents returns contents in StoredPastedContent format for history serialization.
// Small pastes (<=MaxPastedContentLength) are stored inline.
// Large pastes are stored with a hash reference; actual content is on disk.
// Ported from src/history.ts:addToPromptHistory (pastedContents serialization)
func (m *Manager) GetStoredContents() map[int]*StoredPastedContent {
	result := make(map[int]*StoredPastedContent, len(m.contents))
	for id, content := range m.contents {
		if content.Type == "image" {
			continue // Images stored separately
		}
		stored := &StoredPastedContent{
			ID:   content.ID,
			Type: content.Type,
		}
		if len(content.Content) <= MaxPastedContentLength {
			stored.Content = content.Content
		} else {
			// Use hash reference for large pastes
			if hash, ok := m.contentHashes[id]; ok {
				stored.ContentHash = hash
			} else {
				stored.Content = content.Content // fallback
			}
		}
		result[id] = stored
	}
	return result
}

// GetContentHash returns the disk hash for a paste ID, if stored to disk.
func (m *Manager) GetContentHash(id int) (string, bool) {
	hash, ok := m.contentHashes[id]
	return hash, ok
}

// ResolveContent retrieves paste content, loading from disk if stored externally.
// Implements the hash lookup logic from src/history.ts:resolveStoredPastedContent
func (m *Manager) ResolveContent(id int) *PastedContent {
	content := m.contents[id]
	if content != nil {
		return content
	}

	// Try to find by hash in stored contents
	if hash, ok := m.contentHashes[id]; ok {
		if data := utils.RetrievePastedText(hash); data != nil {
			return &PastedContent{
				ID:      id,
				Type:    "text",
				Content: string(data),
			}
		}
	}
	return nil
}

// Clear clears all pasted content.
func (m *Manager) Clear() {
	m.contents = make(map[int]*PastedContent)
	m.nextID = 1
}

// ExpandInput expands all paste references in the input.
func (m *Manager) ExpandInput(input string) string {
	return ExpandPastedTextRefs(input, m.contents)
}

// strToInt converts string to int safely.
func strToInt(s string) int {
	var result int
	negative := false
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return 0
	}
	i := 0
	if s[0] == '-' {
		negative = true
		i = 1
	}
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		result = result*10 + int(s[i]-'0')
		i++
	}
	if negative {
		return -result
	}
	return result
}

// getDiskPasteDir returns the directory path for disk-stored pastes.
func getDiskPasteDir() string {
	return filepath.Join(getConfigDir(), "paste-cache")
}

// getConfigDir returns the Claude Code config directory.
func getConfigDir() string {
	return utils.GetConfigDir()
}

// getDiskPastePath returns the file path for a disk-stored paste.
func getDiskPastePath(dir, hash string) string {
	return filepath.Join(dir, hash+".txt")
}

// contentHash generates a SHA256 hash of content (first 16 hex chars).
func contentHash(content string) string {
	return utils.HashPastedText(content)
}

// mkdirAll creates a directory and its parents.
func mkdirAll(path string) error {
	return os.MkdirAll(path, 0700)
}

// writeFile writes content to a file.
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0600)
}
