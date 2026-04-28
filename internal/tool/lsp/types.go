package lsp

import "time"

// URI represents a text document identifier (file:// URI)
type URI string

// Position in a text document
type Position struct {
	Line      uint32 `json:"line"`
	Character uint32 `json:"character"`
}

// Range in a text document
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location in a text document
type Location struct {
	URI   URI   `json:"uri"`
	Range Range `json:"range"`
}

// LocationLink represents a location with additional context
type LocationLink struct {
	OriginSelectionRange *Range  `json:"originSelectionRange,omitempty"`
	URI                  URI    `json:"uri"`
	Range                Range  `json:"range"`
	TargetSelectionRange Range  `json:"targetSelectionRange"`
}

// TextDocumentIdentifier identifies a text document
type TextDocumentIdentifier struct {
	URI URI `json:"uri"`
}

// TextDocumentPositionParams identifies a position in a text document
type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position    Position              `json:"position"`
}

// ReferenceContext provides additional context for references
type ReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

// DocumentSymbol represents a symbol in a document
type DocumentSymbol struct {
	Name           string             `json:"name"`
	Detail        string             `json:"detail,omitempty"`
	Kind           uint32             `json:"kind"`
	Deprecated     bool               `json:"deprecated,omitempty"`
	Range          Range              `json:"range"`
	SelectionRange Range              `json:"selectionRange"`
	Children       []DocumentSymbol   `json:"children,omitempty"`
}

// SymbolInformation represents information about a symbol
type SymbolInformation struct {
	Name          string           `json:"name"`
	Kind          uint32           `json:"kind"`
	Deprecated    bool             `json:"deprecated,omitempty"`
	Location      Location         `json:"location"`
	ContainerName string           `json:"containerName,omitempty"`
}

// WorkspaceSymbolParams for workspace symbol search
type WorkspaceSymbolParams struct {
	Query    string `json:"query"`
	Limit    uint32 `json:"limit,omitempty"`
	WorkDone *struct {
		Progress bool `json:"progress,omitempty"`
	} `json:"workDoneToken,omitempty"`
}

// HoverParams for hover requests
type HoverParams struct {
	TextDocumentPositionParams
	WorkDone *struct {
		Progress bool `json:"progress,omitempty"`
	} `json:"workDoneToken,omitempty"`
}

// Hover response
type Hover struct {
	Contents any    `json:"contents"` // MarkedString | MarkedString[] | MarkupContent
	Range    *Range `json:"range,omitempty"`
}

// MarkedString is either a plain string or a language-tagged string
type MarkedString = string

// MarkupContent represents content that can be rendered in multiple formats
type MarkupContent struct {
	Kind   string `json:"kind"`   // "plaintext" | "markdown"
	Value  string `json:"value"`
}

// CallHierarchyItem represents an item in a call hierarchy
type CallHierarchyItem struct {
	Name           string           `json:"name"`
	Kind           uint32           `json:"kind"`
	Tags           []uint32         `json:"tags,omitempty"`
	Detail         string           `json:"detail,omitempty"`
	URI            URI              `json:"uri"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Data           any              `json:"data,omitempty"`
}

// CallHierarchyIncomingCall represents an incoming call to a item
type CallHierarchyIncomingCall struct {
	From       CallHierarchyItem `json:"from"`
	FromRanges []Range          `json:"fromRanges"`
}

// CallHierarchyOutgoingCall represents an outgoing call from a item
type CallHierarchyOutgoingCall struct {
	To         CallHierarchyItem `json:"to"`
	FromRanges []Range           `json:"fromRanges"`
}

// CallHierarchyPrepareParams for preparing call hierarchy
type CallHierarchyPrepareParams struct {
	TextDocumentPositionParams
	WorkDone *struct {
		Progress bool `json:"progress,omitempty"`
	} `json:"workDoneToken,omitempty"`
}

// CallHierarchyIncomingCallsParams for incoming calls
type CallHierarchyIncomingCallsParams struct {
	Item         CallHierarchyItem `json:"item"`
	WorkDone    *struct {
		Progress bool `json:"progress,omitempty"`
	} `json:"workDoneToken,omitempty"`
	PartialResult *struct {
		Progress bool `json:"progress,omitempty"`
	} `json:"partialResultToken,omitempty"`
}

// CallHierarchyOutgoingCallsParams for outgoing calls
type CallHierarchyOutgoingCallsParams struct {
	Item         CallHierarchyItem `json:"item"`
	WorkDone    *struct {
		Progress bool `json:"progress,omitempty"`
	} `json:"workDoneToken,omitempty"`
	PartialResult *struct {
		Progress bool `json:"progress,omitempty"`
	} `json:"partialResultToken,omitempty"`
}

// ServerCapabilities represents what a server can do
type ServerCapabilities struct {
	TextDocumentSync       any                  `json:"textDocumentSync,omitempty"`
	HoverProvider          bool                 `json:"hoverProvider,omitempty"`
	DefinitionProvider     bool                 `json:"definitionProvider,omitempty"`
	ReferencesProvider     bool                 `json:"referencesProvider,omitempty"`
	DocumentSymbolProvider bool                 `json:"documentSymbolProvider,omitempty"`
	WorkspaceSymbolProvider bool               `json:"workspaceSymbolProvider,omitempty"`
	ImplementationProvider bool                 `json:"implementationProvider,omitempty"`
	CallHierarchyProvider  bool                 `json:"callHierarchyProvider,omitempty"`
}

// InitializeResult from the server
type InitializeResult struct {
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo     *ServerInfo      `json:"serverInfo,omitempty"`
}

// ServerInfo about the server
type ServerInfo struct {
	Name    string            `json:"name"`
	Version string           `json:"version,omitempty"`
}

// InitializedParams sent after initialize
type InitializedParams struct{}

// PublishDiagnosticsParams for diagnostic notifications
type PublishDiagnosticsParams struct {
	URI         URI            `json:"uri"`
	Diagnostics []Diagnostic  `json:"diagnostics"`
}

// Diagnostic represents a problem
type Diagnostic struct {
	Range    Range    `json:"range"`
	Severity uint32   `json:"severity,omitempty"`
	Code     any      `json:"code,omitempty"`
	Source   string   `json:"source,omitempty"`
	Message  string   `json:"message"`
	Tags     []uint32 `json:"tags,omitempty"`
}

// Diagnostic severity constants
const (
	DiagnosticSeverityError   = 1
	DiagnosticSeverityWarning = 2
	DiagnosticSeverityInfo    = 3
	DiagnosticSeverityHint    = 4
)

// Symbol kind constants (from LSP)
const (
	SymbolKindFile           = 1
	SymbolKindModule         = 2
	SymbolKindNamespace      = 3
	SymbolKindPackage        = 4
	SymbolKindClass          = 5
	SymbolKindMethod         = 6
	SymbolKindProperty       = 7
	SymbolKindField          = 8
	SymbolKindConstructor    = 9
	SymbolKindEnum           = 10
	SymbolKindInterface      = 11
	SymbolKindFunction       = 12
	SymbolKindVariable      = 13
	SymbolKindConstant       = 14
	SymbolKindString         = 15
	SymbolKindNumber         = 16
	SymbolKindBoolean        = 17
	SymbolKindArray          = 18
	SymbolKindObject         = 19
	SymbolKindKey            = 20
	SymbolKindNull           = 21
	SymbolKindEnumMember     = 22
	SymbolKindStruct         = 23
	SymbolKindEvent          = 24
	SymbolKindOperator       = 25
	SymbolKindTypeParameter  = 26
)

// ServerConfig for LSP server configuration
type ServerConfig struct {
	// Command to run the LSP server
	Command string `json:"command"`
	// Arguments for the command
	Args []string `json:"args,omitempty"`
	// Environment variables
	Env map[string]string `json:"env,omitempty"`
	// File extensions this server handles (e.g., ".go", ".ts")
	Extensions []string `json:"extensions,omitempty"`
	// Language IDs this server handles (e.g., "go", "typescript")
	Languages []string `json:"languages,omitempty"`
	// Initialization options passed to the server
	InitializationOptions any `json:"initializationOptions,omitempty"`
	// Workspace folder for the server
	WorkspaceFolder string `json:"workspaceFolder,omitempty"`
	// Startup timeout in seconds
	StartupTimeout int `json:"startupTimeout,omitempty"`
	// Max restarts on crash
	MaxRestarts int `json:"maxRestarts,omitempty"`
}

// ServerState represents the state of an LSP server
type ServerState string

const (
	ServerStateStopped  ServerState = "stopped"
	ServerStateStarting ServerState = "starting"
	ServerStateRunning  ServerState = "running"
	ServerStateStopping ServerState = "stopping"
	ServerStateError    ServerState = "error"
)

// ServerInfoEx holds extended info about a running server
type ServerInfoEx struct {
	Name        string       `json:"name"`
	State       ServerState  `json:"state"`
	Capabilities *ServerCapabilities `json:"capabilities,omitempty"`
	CrashCount  int          `json:"crashCount"`
	LastError   string       `json:"lastError,omitempty"`
	StartedAt   time.Time    `json:"startedAt,omitempty"`
}
