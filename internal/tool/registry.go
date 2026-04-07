package tool

// Registry stores registered tools
type Registry struct {
	tools map[string]Definition
}

// EmptyRegistry creates a new empty tool registry
func EmptyRegistry() *Registry {
	return &Registry{tools: map[string]Definition{}}
}

// Register adds a tool to the registry
func (r *Registry) Register(t Definition) {
	r.tools[t.Name()] = t
}

// Get retrieves a tool by name
func (r *Registry) Get(name string) (Definition, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// Names returns all registered tool names
func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.tools))
	for name := range r.tools {
		out = append(out, name)
	}
	return out
}

// List returns all registered tools
func (r *Registry) List() []Definition {
	out := make([]Definition, 0, len(r.tools))
	for _, definition := range r.tools {
		out = append(out, definition)
	}
	return out
}

// Global registry instance
var globalRegistry = EmptyRegistry()

// Register adds a tool to the global registry
func Register(t Definition) {
	globalRegistry.Register(t)
}

// Get retrieves a tool from the global registry
func Get(name string) (Definition, bool) {
	return globalRegistry.Get(name)
}

// ListAll returns all registered tools
func ListAll() []Definition {
	return globalRegistry.List()
}