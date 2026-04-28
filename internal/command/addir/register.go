package addir

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"claude-go/internal/command"

	tea "github.com/charmbracelet/bubbletea"
)

// TS src/commands/add-dir/add-dir.tsx
// TS src/commands/add-dir/validation.ts

func Register(r *command.Registry) {
	registerAddDir(r)
}

// AddDirectoryResult represents the validation result
// TS src/commands/add-dir/validation.ts:AddDirectoryResult
type AddDirectoryResult struct {
	ResultType    string
	DirectoryPath string
	AbsolutePath  string
	WorkingDir    string
}

// validateDirectoryForWorkspace validates a directory path
// TS src/commands/add-dir/validation.ts:validateDirectoryForWorkspace
func validateDirectoryForWorkspace(directoryPath string, existingDirs []string) AddDirectoryResult {
	if directoryPath == "" {
		return AddDirectoryResult{ResultType: "emptyPath"}
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(expandPath(directoryPath))
	if err != nil {
		return AddDirectoryResult{
			ResultType:    "pathNotFound",
			DirectoryPath: directoryPath,
			AbsolutePath:  directoryPath,
		}
	}

	// Check if path exists and is a directory
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) || os.IsPermission(err) {
			return AddDirectoryResult{
				ResultType:    "pathNotFound",
				DirectoryPath: directoryPath,
				AbsolutePath:  absPath,
			}
		}
		return AddDirectoryResult{
			ResultType:    "pathNotFound",
			DirectoryPath: directoryPath,
			AbsolutePath:  absPath,
		}
	}

	if !info.IsDir() {
		return AddDirectoryResult{
			ResultType:    "notADirectory",
			DirectoryPath: directoryPath,
			AbsolutePath:  absPath,
		}
	}

	// Check if already in existing working directories
	for _, existingDir := range existingDirs {
		if pathInWorkingPath(absPath, existingDir) {
			return AddDirectoryResult{
				ResultType:    "alreadyInWorkingDirectory",
				DirectoryPath: directoryPath,
				WorkingDir:    existingDir,
			}
		}
	}

	return AddDirectoryResult{
		ResultType:   "success",
		AbsolutePath: absPath,
	}
}

// pathInWorkingPath checks if path is within workingDir
func pathInWorkingPath(path, workingDir string) bool {
	rel, err := filepath.Rel(workingDir, path)
	if err != nil {
		return false
	}
	// If relative path doesn't start with "..", it's within the working dir
	return !strings.HasPrefix(rel, "..") && rel != ".."
}

// expandPath expands ~ and environment variables
func expandPath(path string) string {
	// Expand ~ to home directory
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[1:])
		}
	}
	return path
}

// addDirHelpMessage generates help message for validation errors
// TS src/commands/add-dir/validation.ts:addDirHelpMessage
func addDirHelpMessage(result AddDirectoryResult) string {
	switch result.ResultType {
	case "emptyPath":
		return "Please provide a directory path."
	case "pathNotFound":
		return fmt.Sprintf("Path %s was not found.", result.AbsolutePath)
	case "notADirectory":
		parentDir := filepath.Dir(result.AbsolutePath)
		return fmt.Sprintf("%s is not a directory. Did you mean to add the parent directory %s?", result.DirectoryPath, parentDir)
	case "alreadyInWorkingDirectory":
		return fmt.Sprintf("%s is already accessible within the existing working directory %s.", result.DirectoryPath, result.WorkingDir)
	case "success":
		return fmt.Sprintf("Added %s as a working directory.", result.AbsolutePath)
	default:
		return "Unknown error."
	}
}

func registerAddDir(r *command.Registry) {
	r.Register(command.LegacyCommand{
		Type:        command.KindLocalJSX,
		Name:        "add-dir",
		Description: "add a working directory for project context",
		Load:        loadAddDirModel,
		Handler: func(ctx context.Context, rt command.Runtime, args []string) (string, error) {
			return handleAddDirCommand(args, rt), nil
		},
	})
}

func handleAddDirCommand(args []string, rt command.Runtime) string {
	directoryPath := ""
	if len(args) > 0 {
		directoryPath = strings.TrimSpace(args[0])
	}

	// Get existing directories
	existingDirs := []string{}
	if rt.State != nil {
		// Include CWD
		cwd := rt.State.Snapshot().CWD
		if cwd != "" {
			existingDirs = append(existingDirs, cwd)
		}
		// Include additional directories
		additional := rt.State.GetAdditionalDirectories()
		existingDirs = append(existingDirs, additional...)
	}

	// Validate the directory
	result := validateDirectoryForWorkspace(directoryPath, existingDirs)

	if result.ResultType != "success" {
		return addDirHelpMessage(result)
	}

	// Add the directory to state
	if rt.State != nil {
		rt.State.AddAdditionalDirectory(result.AbsolutePath)
	}

	return fmt.Sprintf("Added %s as a working directory for this session · /permissions to manage", result.AbsolutePath)
}

type addDirModel struct {
	rt           command.Runtime
	directoryPath string
	result       AddDirectoryResult
	existingDirs []string
	message      string
}

func loadAddDirModel(_ context.Context, rt command.Runtime, args []string) (tea.Model, error) {
	m := addDirModel{rt: rt}

	// Get existing directories
	if rt.State != nil {
		cwd := rt.State.Snapshot().CWD
		if cwd != "" {
			m.existingDirs = append(m.existingDirs, cwd)
		}
		additional := rt.State.GetAdditionalDirectories()
		m.existingDirs = append(m.existingDirs, additional...)
	}

	// Handle path argument
	if len(args) > 0 {
		m.directoryPath = strings.TrimSpace(args[0])
	}

	// If no path, show help
	if m.directoryPath == "" {
		m.message = `Add Working Directory

Usage: /add-dir <directory-path>

This adds a directory to the project context, allowing tools to access
files within it. Use this when working with multiple projects or when
files are outside the current working directory.

Example:
  /add-dir ~/projects/my-lib
  /add-dir ../shared-code

Press Esc to close`
		return m, nil
	}

	// Validate and add
	result := validateDirectoryForWorkspace(m.directoryPath, m.existingDirs)
	m.result = result

	if result.ResultType == "success" {
		if rt.State != nil {
			rt.State.AddAdditionalDirectory(result.AbsolutePath)
		}
		m.message = fmt.Sprintf("Added %s as a working directory for this session", result.AbsolutePath)
	} else {
		m.message = addDirHelpMessage(result)
	}

	return m, nil
}

func (m addDirModel) Init() tea.Cmd { return nil }

func (m addDirModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc, tea.KeyCtrlC, tea.KeyEnter:
			if m.rt.OnExit != nil {
				m.rt.OnExit()
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m addDirModel) View() string {
	return m.message + "\n\nPress Enter or Esc to close"
}