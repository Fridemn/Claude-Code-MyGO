package command

// RegisterBuiltins registers commands that are still in the command package.
// Commands that have been migrated to subdirectories are registered separately.
func RegisterBuiltins(r *Registry) {
	// All builtin commands have been migrated to subdirectories
}

// RegisterAll registers all commands including those from subdirectories.
// This function should be called by the services container after importing all command packages.
func RegisterAll(r *Registry) {
	RegisterBuiltins(r)
	// Subdirectory commands are registered by their respective packages
}
