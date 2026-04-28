package lsp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Manager manages multiple LSP servers
// TS src/services/lsp/LSPServerManager.ts: extensionMap pattern
type Manager struct {
	mu          sync.RWMutex
	servers     map[string]*Server
	extensionMap map[string][]string // extension → server names (pre-built for fast lookup)
	uriToURI    map[string]URI       // path -> URI
	openedFiles map[string]string    // URI -> server name (track which files are open)
}

// NewManager creates a new LSP manager
func NewManager() *Manager {
	return &Manager{
		servers:     make(map[string]*Server),
		extensionMap: make(map[string][]string),
		uriToURI:    make(map[string]URI),
		openedFiles: make(map[string]string),
	}
}

// RegisterServer adds an LSP server configuration
// TS LSPServerManager.ts:89-117 - builds extensionMap during initialization
func (m *Manager) RegisterServer(name string, config ServerConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.servers[name] = NewServer(name, config)

	// Build extension → server mapping (TS: extensionMap.set(normalized, serverList))
	for _, ext := range config.Extensions {
		normalized := strings.ToLower(ext)
		if m.extensionMap[normalized] == nil {
			m.extensionMap[normalized] = []string{}
		}
		m.extensionMap[normalized] = append(m.extensionMap[normalized], name)
	}
}

// StartAll starts all registered servers
func (m *Manager) StartAll(ctx context.Context) error {
	m.mu.RLock()
	servers := make([]*Server, 0, len(m.servers))
	for _, s := range m.servers {
		servers = append(servers, s)
	}
	m.mu.RUnlock()

	var lastErr error
	for _, srv := range servers {
		if err := srv.Start(ctx); err != nil {
			lastErr = err
			// Don't fail all if one fails - continue starting others
		}
	}
	return lastErr
}

// StopAll stops all servers
func (m *Manager) StopAll() error {
	m.mu.RLock()
	servers := make([]*Server, 0, len(m.servers))
	for _, s := range m.servers {
		servers = append(servers, s)
	}
	m.mu.RUnlock()

	var lastErr error
	for _, srv := range servers {
		if err := srv.Stop(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// ServerForFile finds the server that can handle the given file
// TS LSPServerManager.ts:192-199 - uses extensionMap for fast lookup
func (m *Manager) ServerForFile(filename string) (*Server, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get file extension and normalize (TS: path.extname(filePath).toLowerCase())
	ext := strings.ToLower(filepath.Ext(filename))

	// Fast lookup using pre-built extensionMap (TS: extensionMap.get(ext))
	serverNames := m.extensionMap[ext]
	if len(serverNames) == 0 {
		return nil, fmt.Errorf("no LSP server available for: %s", filename)
	}

	// Use first server (TS: "Use first server (can add priority later)")
	for _, name := range serverNames {
		srv, ok := m.servers[name]
		if ok && srv.State() == ServerStateRunning {
			return srv, nil
		}
	}

	return nil, fmt.Errorf("no running LSP server available for: %s", filename)
}

// ServerInfo returns info about all servers
func (m *Manager) ServerInfo() map[string]ServerInfoEx {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make(map[string]ServerInfoEx, len(m.servers))
	for name, srv := range m.servers {
		out[name] = ServerInfoEx{
			Name:       srv.Name(),
			State:      srv.State(),
			CrashCount: srv.CrashCount(),
			LastError:  srv.LastError(),
		}
	}
	return out
}

// OpenFile opens a file in the appropriate LSP server
// TS LSPServerManager.ts: openFile + openedFiles tracking
func (m *Manager) OpenFile(ctx context.Context, path string, content string) error {
	srv, err := m.ServerForFile(path)
	if err != nil {
		return err
	}

	uri := pathToURI(path)
	langID := detectLanguage(path)

	m.mu.Lock()
	m.uriToURI[path] = uri
	m.openedFiles[string(uri)] = srv.Name() // Track which server has this file open
	m.mu.Unlock()

	return srv.OpenFile(uri, langID, content)
}

// CloseFile closes a file in the appropriate LSP server
// TS LSPServerManager.ts: closeFile + openedFiles cleanup
func (m *Manager) CloseFile(ctx context.Context, path string) error {
	m.mu.RLock()
	uri, ok := m.uriToURI[path]
	m.mu.RUnlock()
	if !ok {
		return nil
	}

	srv, err := m.ServerForFile(path)
	if err != nil {
		return nil
	}

	// Clean up tracking
	m.mu.Lock()
	delete(m.uriToURI, path)
	delete(m.openedFiles, string(uri))
	m.mu.Unlock()

	return srv.CloseFile(uri)
}

// IsFileOpen checks if a file is already open on a compatible LSP server
// TS LSPServerManager.ts:42 - isFileOpen method
func (m *Manager) IsFileOpen(path string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	uri := pathToURI(path)
	_, exists := m.openedFiles[string(uri)]
	return exists
}

// pathToURI converts a file path to a file:// URI
func pathToURI(path string) URI {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	// Convert to absolute path with forward slashes
	absPath = filepath.ToSlash(absPath)
	return URI("file://" + absPath)
}

// uriToPath converts a file:// URI to a path
func uriToPath(uri URI) string {
	u := strings.TrimPrefix(string(uri), "file://")
	// On Windows, strip leading slash if there's a drive letter
	if len(u) >= 3 && u[0] == '/' && u[2] == ':' {
		u = u[1:]
	}
	return u
}

// detectLanguage detects the language ID from file extension
func detectLanguage(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".go":
		return "go"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "typescriptreact"
	case ".js":
		return "javascript"
	case ".jsx":
		return "javascriptreact"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".c":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp", ".hxx":
		return "cpp"
	case ".cs":
		return "csharp"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".swift":
		return "swift"
	case ".kt", ".kts":
		return "kotlin"
	case ".vue":
		return "vue"
	case ".svelte":
		return "svelte"
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".scss":
		return "scss"
	case ".sass":
		return "sass"
	case ".less":
		return "less"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".toml":
		return "toml"
	case ".md":
		return "markdown"
	case ".sql":
		return "sql"
	case ".sh", ".bash":
		return "shell"
	case ".xml":
		return "xml"
	default:
		return "plaintext"
	}
}

// ReadFile reads file content, respecting the 10MB limit
func readFileLimited(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(data) > 10*1024*1024 {
		return "", fmt.Errorf("file too large (>10MB): %s", path)
	}
	return string(data), nil
}
