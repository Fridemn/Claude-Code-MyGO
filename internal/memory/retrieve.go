package memory

import (
	"context"

	"claude-go/internal/types"
)

// RelevantMemoriesConfig contains configuration for memory retrieval.
type RelevantMemoriesConfig struct {
	// MaxSelection is the maximum number of memories to select.
	MaxSelection int
	// MaxSessionBytes is the maximum total bytes per session for recalled memories.
	MaxSessionBytes int
}

// DefaultRelevantMemoriesConfig returns default configuration.
func DefaultRelevantMemoriesConfig() RelevantMemoriesConfig {
	return RelevantMemoriesConfig{
		MaxSelection:    5,
		MaxSessionBytes: 50000,
	}
}

// MemoryRetriever handles finding and selecting relevant memories.
type MemoryRetriever struct {
	config    RelevantMemoriesConfig
	memoryDir string
	selector  *MemorySelector
}

// MemoryRetriever creates a new memory retriever.
func CreateMemoryRetriever(memoryDir string, config RelevantMemoriesConfig) *MemoryRetriever {
	return &MemoryRetriever{
		config:    config,
		memoryDir: memoryDir,
	}
}

// SetSelector sets the memory selector for LLM-based selection.
func (r *MemoryRetriever) SetSelector(selector *MemorySelector) {
	r.selector = selector
}

// FindRelevantMemories finds memory files relevant to a query.
// Returns absolute file paths + mtime of the most relevant memories (up to MaxSelection).
func (r *MemoryRetriever) FindRelevantMemories(ctx context.Context, query string, recentTools []string, alreadySurfaced map[string]bool) ([]types.RelevantMemory, error) {
	// Scan memory files
	memories, err := ScanMemoryFiles(ctx, r.memoryDir)
	if err != nil {
		return nil, err
	}

	// Filter out already surfaced
	var filtered []types.MemoryHeader
	for _, m := range memories {
		if !alreadySurfaced[m.FilePath] {
			filtered = append(filtered, m)
		}
	}

	if len(filtered) == 0 {
		return nil, nil
	}

	// Select relevant memories
	var selectedFilenames []string
	if r.selector != nil {
		selectedFilenames, err = r.selector.SelectRelevantMemories(ctx, query, filtered, recentTools, r.config.MaxSelection)
	} else {
		// Fallback to keyword selection
		selectedFilenames = r.keywordSelection(query, filtered)
	}
	if err != nil {
		return nil, err
	}

	// Map filenames back to memory headers
	byFilename := make(map[string]types.MemoryHeader)
	for _, m := range filtered {
		byFilename[m.Filename] = m
	}

	var result []types.RelevantMemory
	for _, filename := range selectedFilenames {
		if m, ok := byFilename[filename]; ok {
			result = append(result, types.RelevantMemory{
				Path:    m.FilePath,
				MtimeMs: m.MtimeMs,
			})
		}
	}

	return result, nil
}

// keywordSelection performs simple keyword-based memory selection.
func (r *MemoryRetriever) keywordSelection(query string, memories []types.MemoryHeader) []string {
	selector := &MemorySelector{}
	return selector.keywordSelection(query, memories, r.config.MaxSelection)
}

// GetRelevantMemoryAttachments retrieves memory files and formats them as attachments.
func GetRelevantMemoryAttachments(ctx context.Context, memoryDir string, memories []types.RelevantMemory, readFileState map[string]interface{}) (map[string]string, error) {
	return ReadMemoriesForSurfacing(ctx, memoryDir, memories)
}

// FilterDuplicateMemoryAttachments filters out memories that have already been read.
func FilterDuplicateMemoryAttachments(memories []types.RelevantMemory, readFileState map[string]interface{}) []types.RelevantMemory {
	var filtered []types.RelevantMemory
	for _, m := range memories {
		if _, seen := readFileState[m.Path]; !seen {
			filtered = append(filtered, m)
		}
	}
	return filtered
}
