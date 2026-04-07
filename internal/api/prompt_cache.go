package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// CacheTTL constants match TypeScript promptCacheBreakDetection.ts:125-126
const (
	CacheTTL5Min  = 5 * time.Minute
	CacheTTL1Hour = 60 * time.Minute
)

// MinCacheMissTokens is the minimum token drop to trigger a cache break warning
const MinCacheMissTokens = 2000

// MaxTrackedSources limits the number of tracked sources
const MaxTrackedSources = 10

// GlobalCacheStrategy represents the caching strategy
type GlobalCacheStrategy string

const (
	CacheStrategyToolBased    GlobalCacheStrategy = "tool_based"
	CacheStrategySystemPrompt GlobalCacheStrategy = "system_prompt"
	CacheStrategyNone         GlobalCacheStrategy = "none"
)

// PendingChanges tracks what changed between API calls
type PendingChanges struct {
	SystemPromptChanged        bool
	ToolSchemasChanged         bool
	ModelChanged               bool
	FastModeChanged            bool
	CacheControlChanged        bool
	GlobalCacheStrategyChanged bool
	BetasChanged               bool
	AddedToolCount             int
	RemovedToolCount           int
	SystemCharDelta            int
	AddedTools                 []string
	RemovedTools               []string
	ChangedToolSchemas         []string
	PreviousModel              string
	NewModel                   string
}

// PreviousState tracks state for cache break detection
type PreviousState struct {
	SystemHash           uint32
	ToolsHash            uint32
	CacheControlHash     uint32
	ToolNames            []string
	PerToolHashes        map[string]uint32
	SystemCharCount      int
	Model                string
	FastMode             bool
	GlobalCacheStrategy  GlobalCacheStrategy
	Betas                []string
	CallCount            int
	PendingChanges       *PendingChanges
	PrevCacheReadTokens  *int
	CacheDeletionsPending bool
	LastCallTime         time.Time
}

// PromptCacheDetector detects prompt cache breaks
type PromptCacheDetector struct {
	states map[string]*PreviousState
	mu     sync.RWMutex
}

// CreatePromptCacheDetector creates a new cache detector
func CreatePromptCacheDetector() *PromptCacheDetector {
	return &PromptCacheDetector{states: make(map[string]*PreviousState)}
}

// PromptStateSnapshot represents the state to track for cache detection
type PromptStateSnapshot struct {
	System              []map[string]any
	ToolSchemas         []map[string]any
	QuerySource         string
	Model               string
	AgentID             string
	FastMode            bool
	GlobalCacheStrategy GlobalCacheStrategy
	Betas               []string
}

// RecordPromptState records the current prompt state for cache break detection
func (d *PromptCacheDetector) RecordPromptState(snapshot PromptStateSnapshot) {
	key := d.getTrackingKey(snapshot.QuerySource, snapshot.AgentID)
	if key == "" {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	systemHash := d.computeHash(stripCacheControl(snapshot.System))
	toolsHash := d.computeHash(stripCacheControl(snapshot.ToolSchemas))
	cacheControlHash := d.computeHash(extractCacheControls(snapshot.System))
	toolNames := extractToolNames(snapshot.ToolSchemas)

	state, exists := d.states[key]
	if !exists {
		if len(d.states) >= MaxTrackedSources {
			d.evictOldest()
		}
		d.states[key] = &PreviousState{
			SystemHash:           systemHash,
			ToolsHash:            toolsHash,
			CacheControlHash:     cacheControlHash,
			ToolNames:            toolNames,
			PerToolHashes:        d.computePerToolHashes(snapshot.ToolSchemas, toolNames),
			SystemCharCount:      d.getSystemCharCount(snapshot.System),
			Model:                snapshot.Model,
			FastMode:             snapshot.FastMode,
			GlobalCacheStrategy:  snapshot.GlobalCacheStrategy,
			Betas:                snapshot.Betas,
			CallCount:            1,
			LastCallTime:         time.Now(),
		}
		return
	}

	state.CallCount++

	changes := &PendingChanges{
		SystemPromptChanged: systemHash != state.SystemHash,
		ToolSchemasChanged:  toolsHash != state.ToolsHash,
		ModelChanged:        snapshot.Model != state.Model,
		FastModeChanged:     snapshot.FastMode != state.FastMode,
		CacheControlChanged: cacheControlHash != state.CacheControlHash,
		GlobalCacheStrategyChanged: snapshot.GlobalCacheStrategy != state.GlobalCacheStrategy,
	}
	changes.BetasChanged = !sliceEqual(state.Betas, snapshot.Betas)

	if changes.ToolSchemasChanged {
		prevToolSet := sliceToSet(state.ToolNames)
		newToolSet := sliceToSet(toolNames)
		changes.AddedTools = sliceDiff(toolNames, prevToolSet)
		changes.RemovedTools = sliceDiff(state.ToolNames, newToolSet)
		changes.AddedToolCount = len(changes.AddedTools)
		changes.RemovedToolCount = len(changes.RemovedTools)
		state.PerToolHashes = d.computePerToolHashes(snapshot.ToolSchemas, toolNames)
	}

	changes.SystemCharDelta = d.getSystemCharCount(snapshot.System) - state.SystemCharCount
	changes.PreviousModel = state.Model
	changes.NewModel = snapshot.Model

	if changes.SystemPromptChanged || changes.ToolSchemasChanged || changes.ModelChanged ||
		changes.FastModeChanged || changes.CacheControlChanged || changes.GlobalCacheStrategyChanged ||
		changes.BetasChanged {
		state.PendingChanges = changes
	} else {
		state.PendingChanges = nil
	}

	state.SystemHash = systemHash
	state.ToolsHash = toolsHash
	state.CacheControlHash = cacheControlHash
	state.ToolNames = toolNames
	state.SystemCharCount = d.getSystemCharCount(snapshot.System)
	state.Model = snapshot.Model
	state.FastMode = snapshot.FastMode
	state.GlobalCacheStrategy = snapshot.GlobalCacheStrategy
	state.Betas = snapshot.Betas
	state.LastCallTime = time.Now()
}

// CacheBreakResult represents the result of a cache break check
type CacheBreakResult struct {
	Detected            bool
	Reason              string
	PrevCacheReadTokens int
	CacheReadTokens     int
	CacheCreationTokens int
	Changes             *PendingChanges
	TimeSinceLastCall   time.Duration
	Over5MinTTL         bool
	Over1HourTTL        bool
}

// CheckResponseForCacheBreak checks if a cache break occurred
func (d *PromptCacheDetector) CheckResponseForCacheBreak(
	querySource, agentID string,
	cacheReadTokens, cacheCreationTokens int,
) *CacheBreakResult {
	key := d.getTrackingKey(querySource, agentID)
	if key == "" {
		return nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	state, exists := d.states[key]
	if !exists || d.isExcludedModel(state.Model) {
		return nil
	}

	prevCacheRead := state.PrevCacheReadTokens
	state.PrevCacheReadTokens = &cacheReadTokens
	timeSinceLastCall := time.Since(state.LastCallTime)

	if prevCacheRead == nil {
		return nil
	}

	if state.CacheDeletionsPending {
		state.CacheDeletionsPending = false
		state.PendingChanges = nil
		return nil
	}

	tokenDrop := *prevCacheRead - cacheReadTokens
	if cacheReadTokens >= *prevCacheRead*95/100 || tokenDrop < MinCacheMissTokens {
		state.PendingChanges = nil
		return nil
	}

	result := &CacheBreakResult{
		Detected:            true,
		PrevCacheReadTokens: *prevCacheRead,
		CacheReadTokens:     cacheReadTokens,
		CacheCreationTokens: cacheCreationTokens,
		Changes:             state.PendingChanges,
		TimeSinceLastCall:   timeSinceLastCall,
		Over5MinTTL:         timeSinceLastCall > CacheTTL5Min,
		Over1HourTTL:        timeSinceLastCall > CacheTTL1Hour,
	}
	result.Reason = d.buildReason(state.PendingChanges, result)
	state.PendingChanges = nil

	return result
}

// NotifyCacheDeletion marks that cache deletions are pending
func (d *PromptCacheDetector) NotifyCacheDeletion(querySource, agentID string) {
	key := d.getTrackingKey(querySource, agentID)
	if key == "" {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if state, exists := d.states[key]; exists {
		state.CacheDeletionsPending = true
	}
}

// NotifyCompaction resets the cache read baseline after compaction
func (d *PromptCacheDetector) NotifyCompaction(querySource, agentID string) {
	key := d.getTrackingKey(querySource, agentID)
	if key == "" {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if state, exists := d.states[key]; exists {
		state.PrevCacheReadTokens = nil
	}
}

// CleanupAgentTracking removes tracking for an agent
func (d *PromptCacheDetector) CleanupAgentTracking(agentID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.states, agentID)
}

// Reset clears all tracking state
func (d *PromptCacheDetector) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.states = make(map[string]*PreviousState)
}

// Helper methods

func (d *PromptCacheDetector) getTrackingKey(querySource, agentID string) string {
	trackedPrefixes := []string{"repl_main_thread", "sdk", "agent:custom", "agent:default", "agent:builtin"}
	if querySource == "compact" {
		return "repl_main_thread"
	}
	for _, prefix := range trackedPrefixes {
		if strings.HasPrefix(querySource, prefix) {
			if agentID != "" {
				return agentID
			}
			return querySource
		}
	}
	return ""
}

func (d *PromptCacheDetector) computeHash(data any) uint32 {
	b, err := json.Marshal(data)
	if err != nil {
		return 0
	}
	h := uint32(2166136261)
	for _, c := range b {
		h ^= uint32(c)
		h *= 16777619
	}
	return h
}

func (d *PromptCacheDetector) computePerToolHashes(tools []map[string]any, names []string) map[string]uint32 {
	hashes := make(map[string]uint32)
	stripped := stripCacheControl(tools)
	for i, name := range names {
		if i < len(stripped) {
			hashes[name] = d.computeHash(stripped[i])
		}
	}
	return hashes
}

func (d *PromptCacheDetector) getSystemCharCount(system []map[string]any) int {
	total := 0
	for _, block := range system {
		if text, ok := block["text"].(string); ok {
			total += len(text)
		}
	}
	return total
}

func (d *PromptCacheDetector) isExcludedModel(model string) bool {
	return strings.Contains(strings.ToLower(model), "haiku")
}

func (d *PromptCacheDetector) buildReason(changes *PendingChanges, result *CacheBreakResult) string {
	parts := []string{}
	if changes != nil {
		if changes.ModelChanged {
			parts = append(parts, fmt.Sprintf("model changed (%s → %s)", changes.PreviousModel, changes.NewModel))
		}
		if changes.SystemPromptChanged {
			parts = append(parts, "system prompt changed")
		}
		if changes.ToolSchemasChanged {
			parts = append(parts, "tools changed")
		}
		if changes.FastModeChanged {
			parts = append(parts, "fast mode toggled")
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, ", ")
	}
	if result.Over1HourTTL {
		return "possible 1h TTL expiry (prompt unchanged)"
	}
	if result.Over5MinTTL {
		return "possible 5min TTL expiry (prompt unchanged)"
	}
	return "likely server-side (prompt unchanged, <5min gap)"
}

func (d *PromptCacheDetector) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	for key, state := range d.states {
		if oldestKey == "" || state.LastCallTime.Before(oldestTime) {
			oldestKey = key
			oldestTime = state.LastCallTime
		}
	}
	if oldestKey != "" {
		delete(d.states, oldestKey)
	}
}

func stripCacheControl(items []map[string]any) []any {
	result := make([]any, len(items))
	for i, item := range items {
		copy := make(map[string]any)
		for k, v := range item {
			if k != "cache_control" {
				copy[k] = v
			}
		}
		result[i] = copy
	}
	return result
}

func extractCacheControls(items []map[string]any) []any {
	result := make([]any, len(items))
	for i, item := range items {
		if cc, ok := item["cache_control"]; ok {
			result[i] = cc
		} else {
			result[i] = nil
		}
	}
	return result
}

func extractToolNames(tools []map[string]any) []string {
	names := make([]string, len(tools))
	for i, tool := range tools {
		if name, ok := tool["name"].(string); ok {
			names[i] = name
		}
	}
	return names
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sliceToSet(s []string) map[string]bool {
	set := make(map[string]bool)
	for _, v := range s {
		set[v] = true
	}
	return set
}

func sliceDiff(s []string, exclude map[string]bool) []string {
	result := []string{}
	for _, v := range s {
		if !exclude[v] {
			result = append(result, v)
		}
	}
	return result
}

// Global cache detector
var globalCacheDetector = CreatePromptCacheDetector()

// RecordPromptStateGlobal records prompt state using the global detector
func RecordPromptStateGlobal(snapshot PromptStateSnapshot) {
	globalCacheDetector.RecordPromptState(snapshot)
}

// CheckResponseForCacheBreakGlobal checks for cache break using the global detector
func CheckResponseForCacheBreakGlobal(querySource, agentID string, cacheReadTokens, cacheCreationTokens int) *CacheBreakResult {
	return globalCacheDetector.CheckResponseForCacheBreak(querySource, agentID, cacheReadTokens, cacheCreationTokens)
}