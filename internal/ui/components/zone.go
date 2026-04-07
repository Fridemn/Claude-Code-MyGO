// Package components provides reusable TUI components
// zone.go provides BubbleZone integration for mouse click support
// Matches Ink's onClick behavior from src/components
package components

import (
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
)

// ZoneManager wraps BubbleZone for mouse click region tracking
// This matches the onClick behavior in Ink components
type ZoneManager struct {
	manager *zone.Manager
}

// ZoneManager creates a new zone manager
func NewZoneManager() *ZoneManager {
	return ZoneManagerFor()
}

func ZoneManagerFor() *ZoneManager {
	return &ZoneManager{
		manager: zone.New(),
	}
}

// Mark marks a string with a zone ID for click detection
// Usage: zm.Mark("my-button", "Click me")
func (zm *ZoneManager) Mark(id string, content string) string {
	return zm.manager.Mark(id, content)
}

// Scan parses the content for zone markers
// Call this on the full rendered output
func (zm *ZoneManager) Scan(content string) string {
	return zm.manager.Scan(content)
}

// Get returns the zone info for a given ID
func (zm *ZoneManager) Get(id string) *zone.ZoneInfo {
	return zm.manager.Get(id)
}

// InBounds checks if the mouse event is within the zone
func (zm *ZoneManager) InBounds(id string, mouse tea.MouseMsg) bool {
	info := zm.manager.Get(id)
	if info == nil || info.IsZero() {
		return false
	}
	return info.InBounds(mouse)
}

// InBoundsXY checks if x,y coordinates are within the zone
func (zm *ZoneManager) InBoundsXY(id string, x, y int) bool {
	info := zm.manager.Get(id)
	if info == nil || info.IsZero() {
		return false
	}
	return x >= info.StartX && x <= info.EndX && y >= info.StartY && y <= info.EndY
}

// ClickableZone represents a clickable region
type ClickableZone struct {
	ID      string
	Handler func()
}

// ZoneRegistry tracks clickable zones and their handlers
type ZoneRegistry struct {
	zones   map[string]func()
	manager *ZoneManager
}

// ZoneRegistry creates a new zone registry with handlers
func NewZoneRegistry() *ZoneRegistry {
	return ZoneRegistryFor()
}

func ZoneRegistryFor() *ZoneRegistry {
	return &ZoneRegistry{
		zones:   make(map[string]func()),
		manager: ZoneManagerFor(),
	}
}

// Register registers a zone with a click handler
func (zr *ZoneRegistry) Register(id string, handler func()) {
	zr.zones[id] = handler
}

// Unregister removes a zone handler
func (zr *ZoneRegistry) Unregister(id string) {
	delete(zr.zones, id)
}

// Mark marks content as a clickable zone
func (zr *ZoneRegistry) Mark(id string, content string) string {
	return zr.manager.Mark(id, content)
}

// Scan processes the content for zone markers
func (zr *ZoneRegistry) Scan(content string) string {
	return zr.manager.Scan(content)
}

// HandleClick checks if a mouse event hits any registered zone
// and calls its handler if so. Returns true if handled.
func (zr *ZoneRegistry) HandleClick(mouse tea.MouseMsg) bool {
	for id, handler := range zr.zones {
		if zr.manager.InBounds(id, mouse) {
			handler()
			return true
		}
	}
	return false
}

// HandleClickXY checks if a click at x,y hits any registered zone
func (zr *ZoneRegistry) HandleClickXY(x, y int) bool {
	for id, handler := range zr.zones {
		if zr.manager.InBoundsXY(id, x, y) {
			handler()
			return true
		}
	}
	return false
}

// Clear clears all registered zones
func (zr *ZoneRegistry) Clear() {
	zr.zones = make(map[string]func())
}

// Helper functions for common click patterns

// MarkButton creates a clickable button-style zone
func (zr *ZoneRegistry) MarkButton(id, label string, handler func()) string {
	zr.Register(id, handler)
	return zr.Mark(id, "[ "+label+" ]")
}

// MarkLink creates a clickable link-style zone
func (zr *ZoneRegistry) MarkLink(id, text string, handler func()) string {
	zr.Register(id, handler)
	return zr.Mark(id, text)
}

// MarkListItem creates a clickable list item zone
func (zr *ZoneRegistry) MarkListItem(id string, content string, handler func()) string {
	zr.Register(id, handler)
	return zr.Mark(id, content)
}

// Global zone manager instance (for simpler usage)
var globalZoneManager = ZoneManagerFor()

// GlobalMark marks content using the global zone manager
func GlobalMark(id, content string) string {
	return globalZoneManager.Mark(id, content)
}

// GlobalScan scans content using the global zone manager
func GlobalScan(content string) string {
	return globalZoneManager.Scan(content)
}

// GlobalInBounds checks bounds using the global zone manager
func GlobalInBounds(id string, mouse tea.MouseMsg) bool {
	return globalZoneManager.InBounds(id, mouse)
}

// GlobalInBoundsXY checks bounds using x,y coordinates
func GlobalInBoundsXY(id string, x, y int) bool {
	return globalZoneManager.InBoundsXY(id, x, y)
}
