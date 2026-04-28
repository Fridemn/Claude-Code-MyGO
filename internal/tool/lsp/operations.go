package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// OperationResult represents the result of an LSP operation
type OperationResult struct {
	Operation string
	Content   string
	Error     string
}

// GoToDefinition performs textDocument/definition
func GoToDefinition(ctx context.Context, m *Manager, filePath string, line, character uint32) *OperationResult {
	srv, err := m.ServerForFile(filePath)
	if err != nil {
		return &OperationResult{Operation: "goToDefinition", Error: err.Error()}
	}

	// Open file if not already open
	if err := m.OpenFile(ctx, filePath, ""); err != nil {
		// Try reading and opening
		content, err := readFileLimited(filePath)
		if err != nil {
			return &OperationResult{Operation: "goToDefinition", Error: "file not found: " + err.Error()}
		}
		if err := m.OpenFile(ctx, filePath, content); err != nil {
			return &OperationResult{Operation: "goToDefinition", Error: "failed to open file: " + err.Error()}
		}
	}

	params := TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: pathToURI(filePath)},
		Position:    Position{Line: line, Character: character},
	}

	result, err := srv.SendRequest(ctx, "textDocument/definition", params)
	if err != nil {
		return &OperationResult{Operation: "goToDefinition", Error: err.Error()}
	}

	return formatLocationResult(result)
}

// FindReferences performs textDocument/references
func FindReferences(ctx context.Context, m *Manager, filePath string, line, character uint32) *OperationResult {
	srv, err := m.ServerForFile(filePath)
	if err != nil {
		return &OperationResult{Operation: "findReferences", Error: err.Error()}
	}

	// Ensure file is open
	ensureFileOpen(ctx, m, srv, filePath)

	params := map[string]any{
		"textDocument": TextDocumentIdentifier{URI: pathToURI(filePath)},
		"position":     Position{Line: line, Character: character},
		"context":       ReferenceContext{IncludeDeclaration: true},
	}

	result, err := srv.SendRequest(ctx, "textDocument/references", params)
	if err != nil {
		return &OperationResult{Operation: "findReferences", Error: err.Error()}
	}

	return formatReferencesResult(result)
}

// HoverOp performs textDocument/hover
func HoverOp(ctx context.Context, m *Manager, filePath string, line, character uint32) *OperationResult {
	srv, err := m.ServerForFile(filePath)
	if err != nil {
		return &OperationResult{Operation: "hover", Error: err.Error()}
	}

	// Ensure file is open
	ensureFileOpen(ctx, m, srv, filePath)

	params := HoverParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: pathToURI(filePath)},
			Position:    Position{Line: line, Character: character},
		},
	}

	result, err := srv.SendRequest(ctx, "textDocument/hover", params)
	if err != nil {
		return &OperationResult{Operation: "hover", Error: err.Error()}
	}

	return formatHoverResult(result)
}

// DocumentSymbolOp performs textDocument/documentSymbol
func DocumentSymbolOp(ctx context.Context, m *Manager, filePath string) *OperationResult {
	srv, err := m.ServerForFile(filePath)
	if err != nil {
		return &OperationResult{Operation: "documentSymbol", Error: err.Error()}
	}

	// Ensure file is open
	ensureFileOpen(ctx, m, srv, filePath)

	params := map[string]any{
		"textDocument": TextDocumentIdentifier{URI: pathToURI(filePath)},
	}

	result, err := srv.SendRequest(ctx, "textDocument/documentSymbol", params)
	if err != nil {
		return &OperationResult{Operation: "documentSymbol", Error: err.Error()}
	}

	return formatDocumentSymbolResult(result)
}

// WorkspaceSymbol performs workspace/symbol
func WorkspaceSymbol(ctx context.Context, m *Manager, query string) *OperationResult {
	m.mu.RLock()
	var runningServers []*Server
	for _, srv := range m.servers {
		if srv.State() == ServerStateRunning {
			runningServers = append(runningServers, srv)
		}
	}
	m.mu.RUnlock()

	if len(runningServers) == 0 {
		return &OperationResult{Operation: "workspaceSymbol", Error: "no LSP servers running"}
	}

	// Try first available server
	params := WorkspaceSymbolParams{Query: query, Limit: 50}

	var err error
	var result json.RawMessage
	for _, srv := range runningServers {
		result, err = srv.SendRequest(ctx, "workspace/symbol", params)
		if err == nil {
			return formatWorkspaceSymbolResult(result)
		}
	}

	return &OperationResult{Operation: "workspaceSymbol", Error: err.Error()}
}

// GoToImplementation performs textDocument/implementation
func GoToImplementation(ctx context.Context, m *Manager, filePath string, line, character uint32) *OperationResult {
	srv, err := m.ServerForFile(filePath)
	if err != nil {
		return &OperationResult{Operation: "goToImplementation", Error: err.Error()}
	}

	ensureFileOpen(ctx, m, srv, filePath)

	params := TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: pathToURI(filePath)},
		Position:    Position{Line: line, Character: character},
	}

	result, err := srv.SendRequest(ctx, "textDocument/implementation", params)
	if err != nil {
		return &OperationResult{Operation: "goToImplementation", Error: err.Error()}
	}

	return formatLocationResult(result)
}

// PrepareCallHierarchy performs textDocument/prepareCallHierarchy
func PrepareCallHierarchy(ctx context.Context, m *Manager, filePath string, line, character uint32) *OperationResult {
	srv, err := m.ServerForFile(filePath)
	if err != nil {
		return &OperationResult{Operation: "prepareCallHierarchy", Error: err.Error()}
	}

	ensureFileOpen(ctx, m, srv, filePath)

	params := CallHierarchyPrepareParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: pathToURI(filePath)},
			Position:    Position{Line: line, Character: character},
		},
	}

	result, err := srv.SendRequest(ctx, "textDocument/prepareCallHierarchy", params)
	if err != nil {
		return &OperationResult{Operation: "prepareCallHierarchy", Error: err.Error()}
	}

	return formatCallHierarchyItems(result)
}

// IncomingCalls performs callHierarchy/incomingCalls
func IncomingCalls(ctx context.Context, m *Manager, item CallHierarchyItem) *OperationResult {
	srv, err := m.ServerForFile(uriToPath(item.URI))
	if err != nil {
		return &OperationResult{Operation: "incomingCalls", Error: err.Error()}
	}

	params := CallHierarchyIncomingCallsParams{Item: item}
	result, err := srv.SendRequest(ctx, "callHierarchy/incomingCalls", params)
	if err != nil {
		return &OperationResult{Operation: "incomingCalls", Error: err.Error()}
	}

	return formatIncomingCalls(result)
}

// OutgoingCalls performs callHierarchy/outgoingCalls
func OutgoingCalls(ctx context.Context, m *Manager, item CallHierarchyItem) *OperationResult {
	srv, err := m.ServerForFile(uriToPath(item.URI))
	if err != nil {
		return &OperationResult{Operation: "outgoingCalls", Error: err.Error()}
	}

	params := CallHierarchyOutgoingCallsParams{Item: item}
	result, err := srv.SendRequest(ctx, "callHierarchy/outgoingCalls", params)
	if err != nil {
		return &OperationResult{Operation: "outgoingCalls", Error: err.Error()}
	}

	return formatOutgoingCalls(result)
}

// ensureFileOpen ensures a file is opened in the LSP server
func ensureFileOpen(ctx context.Context, m *Manager, srv *Server, filePath string) {
	m.mu.RLock()
	_, ok := m.uriToURI[filePath]
	m.mu.RUnlock()
	if ok {
		return
	}

	content, err := readFileLimited(filePath)
	if err != nil {
		return
	}
	m.OpenFile(ctx, filePath, content)
}

// --- Formatting helpers ---

func formatLocationResult(data json.RawMessage) *OperationResult {
	if data == nil {
		return &OperationResult{Content: "(no definition found)"}
	}

	// Try Location
	var loc Location
	if err := json.Unmarshal(data, &loc); err == nil && loc.URI != "" {
		return &OperationResult{Content: formatLocation(&loc)}
	}

	// Try Location[]
	var locs []Location
	if err := json.Unmarshal(data, &locs); err == nil && len(locs) > 0 {
		var parts []string
		for _, l := range locs {
			parts = append(parts, formatLocation(&l))
		}
		return &OperationResult{Content: strings.Join(parts, "\n")}
	}

	// Try LocationLink[]
	var links []LocationLink
	if err := json.Unmarshal(data, &links); err == nil && len(links) > 0 {
		var parts []string
		for _, l := range links {
			parts = append(parts, formatLocationLink(&l))
		}
		return &OperationResult{Content: strings.Join(parts, "\n")}
	}

	return &OperationResult{Content: string(data)}
}

func formatLocation(loc *Location) string {
	return fmt.Sprintf("%s:%d:%d",
		filepath.Base(uriToPath(loc.URI)),
		loc.Range.Start.Line+1,
		loc.Range.Start.Character+1)
}

func formatLocationLink(link *LocationLink) string {
	return fmt.Sprintf("%s:%d:%d",
		filepath.Base(uriToPath(link.URI)),
		link.Range.Start.Line+1,
		link.Range.Start.Character+1)
}

func formatReferencesResult(data json.RawMessage) *OperationResult {
	if data == nil {
		return &OperationResult{Content: "(no references found)"}
	}

	var refs []Location
	if err := json.Unmarshal(data, &refs); err != nil {
		return &OperationResult{Content: string(data)}
	}

	if len(refs) == 0 {
		return &OperationResult{Content: "(no references found)"}
	}

	// Group by file
	byFile := make(map[string][]Location)
	for _, ref := range refs {
		path := uriToPath(ref.URI)
		byFile[path] = append(byFile[path], ref)
	}

	var parts []string
	for path, locs := range byFile {
		parts = append(parts, fmt.Sprintf("%s:", filepath.Base(path)))
		for _, loc := range locs {
			parts = append(parts, fmt.Sprintf("  %d:%d", loc.Range.Start.Line+1, loc.Range.Start.Character+1))
		}
	}

	return &OperationResult{Content: strings.Join(parts, "\n")}
}

func formatHoverResult(data json.RawMessage) *OperationResult {
	if data == nil {
		return &OperationResult{Content: "(no hover information)"}
	}

	var h Hover
	if err := json.Unmarshal(data, &h); err != nil {
		return &OperationResult{Content: string(data)}
	}

	content := extractHoverContent(h.Contents)
	return &OperationResult{Content: content}
}

func extractHoverContent(contents any) string {
	switch v := contents.(type) {
	case string:
		return v
	case []any:
		var parts []string
		for _, item := range v {
			parts = append(parts, extractHoverContent(item))
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		if val, ok := v["value"].(string); ok {
			return val
		}
	}
	return fmt.Sprintf("%v", contents)
}

func formatDocumentSymbolResult(data json.RawMessage) *OperationResult {
	if data == nil {
		return &OperationResult{Content: "(no symbols found)"}
	}

	// Try DocumentSymbol[]
	var symbols []DocumentSymbol
	if err := json.Unmarshal(data, &symbols); err != nil {
		// Try SymbolInformation[]
		var info []SymbolInformation
		if err2 := json.Unmarshal(data, &info); err2 == nil {
			return formatSymbolInformation(info)
		}
		return &OperationResult{Content: string(data)}
	}

	return formatDocumentSymbols(symbols, 0)
}

func formatDocumentSymbols(symbols []DocumentSymbol, indent int) *OperationResult {
	prefix := strings.Repeat("  ", indent)
	var parts []string
	for _, sym := range symbols {
		kind := symbolKindToString(sym.Kind)
		line := fmt.Sprintf("%s%s %s (line %d)", prefix, kind, sym.Name, sym.Range.Start.Line+1)
		if sym.Detail != "" {
			line += " — " + sym.Detail
		}
		parts = append(parts, line)
		if len(sym.Children) > 0 {
			childResult := formatDocumentSymbols(sym.Children, indent+1)
			if childResult != nil && childResult.Content != "" {
				parts = append(parts, childResult.Content)
			}
		}
	}
	return &OperationResult{Content: strings.Join(parts, "\n")}
}

func formatSymbolInformation(info []SymbolInformation) *OperationResult {
	var parts []string
	for _, sym := range info {
		kind := symbolKindToString(sym.Kind)
		line := fmt.Sprintf("%s %s — %s:%d", kind, sym.Name, filepath.Base(uriToPath(sym.Location.URI)), sym.Location.Range.Start.Line+1)
		parts = append(parts, line)
	}
	return &OperationResult{Content: strings.Join(parts, "\n")}
}

func formatWorkspaceSymbolResult(data json.RawMessage) *OperationResult {
	if data == nil {
		return &OperationResult{Content: "(no symbols found)"}
	}

	var symbols []SymbolInformation
	if err := json.Unmarshal(data, &symbols); err != nil {
		return &OperationResult{Content: string(data)}
	}

	if len(symbols) == 0 {
		return &OperationResult{Content: "(no symbols found)"}
	}

	// Group by file
	byFile := make(map[string][]SymbolInformation)
	for _, sym := range symbols {
		path := uriToPath(sym.Location.URI)
		byFile[path] = append(byFile[path], sym)
	}

	var parts []string
	for path, syms := range byFile {
		parts = append(parts, fmt.Sprintf("%s:", filepath.Base(path)))
		for _, sym := range syms {
			kind := symbolKindToString(sym.Kind)
			parts = append(parts, fmt.Sprintf("  %s %s", kind, sym.Name))
		}
	}

	return &OperationResult{Content: strings.Join(parts, "\n")}
}

func formatCallHierarchyItems(data json.RawMessage) *OperationResult {
	if data == nil {
		return &OperationResult{Content: "(not a callable symbol)"}
	}

	var items []CallHierarchyItem
	if err := json.Unmarshal(data, &items); err != nil {
		return &OperationResult{Content: string(data)}
	}

	var parts []string
	for _, item := range items {
		kind := symbolKindToString(item.Kind)
		parts = append(parts, fmt.Sprintf("%s %s — %s:%d", kind, item.Name, filepath.Base(uriToPath(item.URI)), item.Range.Start.Line+1))
	}

	return &OperationResult{Content: strings.Join(parts, "\n")}
}

func formatIncomingCalls(data json.RawMessage) *OperationResult {
	if data == nil {
		return &OperationResult{Content: "(no incoming calls)"}
	}

	var calls []CallHierarchyIncomingCall
	if err := json.Unmarshal(data, &calls); err != nil {
		return &OperationResult{Content: string(data)}
	}

	if len(calls) == 0 {
		return &OperationResult{Content: "(no incoming calls)"}
	}

	var parts []string
	for _, call := range calls {
		parts = append(parts, fmt.Sprintf("%s %s:", symbolKindToString(call.From.Kind), call.From.Name))
		for _, r := range call.FromRanges {
			parts = append(parts, fmt.Sprintf("  %s:%d", filepath.Base(uriToPath(call.From.URI)), r.Start.Line+1))
		}
	}

	return &OperationResult{Content: strings.Join(parts, "\n")}
}

func formatOutgoingCalls(data json.RawMessage) *OperationResult {
	if data == nil {
		return &OperationResult{Content: "(no outgoing calls)"}
	}

	var calls []CallHierarchyOutgoingCall
	if err := json.Unmarshal(data, &calls); err != nil {
		return &OperationResult{Content: string(data)}
	}

	if len(calls) == 0 {
		return &OperationResult{Content: "(no outgoing calls)"}
	}

	var parts []string
	for _, call := range calls {
		parts = append(parts, fmt.Sprintf("%s %s:", symbolKindToString(call.To.Kind), call.To.Name))
		for _, r := range call.FromRanges {
			parts = append(parts, fmt.Sprintf("  %s:%d", filepath.Base(uriToPath(call.To.URI)), r.Start.Line+1))
		}
	}

	return &OperationResult{Content: strings.Join(parts, "\n")}
}

func symbolKindToString(kind uint32) string {
	switch kind {
	case 1: return "[file]"
	case 2: return "[module]"
	case 3: return "[namespace]"
	case 4: return "[package]"
	case 5: return "[class]"
	case 6: return "[method]"
	case 7: return "[property]"
	case 8: return "[field]"
	case 9: return "[constructor]"
	case 10: return "[enum]"
	case 11: return "[interface]"
	case 12: return "[function]"
	case 13: return "[variable]"
	case 14: return "[constant]"
	case 15: return "[string]"
	case 16: return "[number]"
	case 17: return "[boolean]"
	case 18: return "[array]"
	case 19: return "[object]"
	case 20: return "[key]"
	case 21: return "[null]"
	case 22: return "[enum-member]"
	case 23: return "[struct]"
	case 24: return "[event]"
	case 25: return "[operator]"
	case 26: return "[type-param]"
	default: return "[symbol]"
	}
}
