//go:build ignore

package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Build configuration
var (
	binaryName = "claude-go"
	version    = "0.1.0-alpha"
	gitCommit  = "unknown"
	buildTime  = ""
	projectEnv = map[string]string{}
)

var buildEnvLdflagVars = map[string]string{
	"CLAUDE_CODE_APP_NAME":       "claude-go/internal/config.builtinAppName",
	"CLAUDE_CODE_SESSION_DIR":    "claude-go/internal/config.builtinSessionDir",
	"CLAUDE_CODE_MCP_CONFIG":     "claude-go/internal/config.builtinMCPConfigPath",
	"CLAUDE_CODE_PLUGINS_CONFIG": "claude-go/internal/config.builtinPluginsConfigPath",
	"CLAUDE_CODE_HOOKS_CONFIG":   "claude-go/internal/config.builtinHooksConfigPath",
	"CLAUDE_CODE_MAX_TURNS":      "claude-go/internal/config.builtinMaxTurns",
	"CLAUDE_CODE_CONTEXT_WINDOW": "claude-go/internal/config.builtinContextWindow",
}

var llmAPIEnvKeys = map[string]bool{
	"CLAUDE_CODE_API_KEY":       true,
	"CLAUDE_CODE_BASE_URL":      true,
	"CLAUDE_CODE_MODEL":         true,
	"CLAUDE_CODE_SUMMARY_MODEL": true,
	"CLAUDE_CODE_FAST_MODEL":    true,
	"CLAUDE_CODE_OAUTH_TOKEN":   true,
	"ANTHROPIC_API_KEY":         true,
	"OPENAI_API_KEY":            true,
}

// Platform represents a build target
type Platform struct {
	OS   string
	Arch string
	Ext  string // File extension (.exe for windows)
}

// String returns platform identifier
func (p Platform) String() string {
	return fmt.Sprintf("%s/%s", p.OS, p.Arch)
}

// OutputName returns the output binary name
func (p Platform) OutputName() string {
	name := fmt.Sprintf("%s_%s_%s", binaryName, p.OS, p.Arch)
	if p.Ext != "" {
		name += p.Ext
	}
	return name
}

// ArchiveName returns the archive name
func (p Platform) ArchiveName() string {
	if p.OS == "windows" {
		return p.OutputName()[:len(p.OutputName())-len(p.Ext)] + ".zip"
	}
	return p.OutputName() + ".tar.gz"
}

// Supported platforms
var platforms = []Platform{
	{OS: "linux", Arch: "amd64"},
	{OS: "linux", Arch: "arm64"},
	{OS: "darwin", Arch: "amd64"},
	{OS: "darwin", Arch: "arm64"},
	{OS: "windows", Arch: "amd64", Ext: ".exe"},
	{OS: "windows", Arch: "arm64", Ext: ".exe"},
}

func main() {
	// Parse flags
	var (
		action       = flag.String("action", "build", "Action: build, build-all, release, clean, test, info")
		verFlag      = flag.String("version", version, "Version string")
		platformOS   = flag.String("os", "", "Target OS (linux, darwin, windows)")
		platformArch = flag.String("arch", "", "Target architecture (amd64, arm64)")
		skipTests    = flag.Bool("skip-tests", false, "Skip running tests")
		help         = flag.Bool("help", false, "Show help")
	)
	flag.Parse()

	if *help {
		printUsage()
		return
	}

	version = *verFlag

	// Initialize build info
	initBuildInfo()

	// Execute action
	switch *action {
	case "clean":
		clean()
	case "test":
		if !*skipTests {
			runTests()
		}
	case "info":
		printInfo()
	case "build":
		if *platformOS != "" && *platformArch != "" {
			buildPlatform(Platform{OS: *platformOS, Arch: *platformArch})
		} else {
			buildCurrent()
		}
	case "build-all":
		buildAll()
	case "release":
		release()
	default:
		fmt.Printf("Unknown action: %s\n", *action)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`Claude Code Go - Build Script

USAGE:
    go run build.go [OPTIONS]

OPTIONS:
    -action VALUE       Action to perform (default: build)
                        build       - Build for current platform
                        build-all   - Build for all platforms
                        release     - Create release archives
                        clean       - Clean build artifacts
                        test        - Run tests
                        info        - Show build environment info
    
    -version VERSION    Set version string (default: %s)
    -os OS              Target OS: linux, darwin, windows
    -arch ARCH          Target arch: amd64, arm64
    -skip-tests         Skip running tests
    -help               Show this help

EXAMPLES:
    go run build.go
    go run build.go -action build-all
    go run build.go -action release -version 1.0.0
    go run build.go -os darwin -arch arm64

`, version)
}

func initBuildInfo() {
	projectEnv = loadProjectEnv(".env")

	// Get git commit
	if cmd, err := exec.LookPath("git"); err == nil {
		out, err := exec.Command(cmd, "rev-parse", "--short", "HEAD").Output()
		if err == nil {
			gitCommit = strings.TrimSpace(string(out))
		}
	}

	// Get build time
	buildTime = time.Now().UTC().Format("2006-01-02_15:04:05")
}

// getLdflags returns linker flags
func getLdflags() string {
	parts := []string{
		ldflagSet("main.Version", version),
		ldflagSet("main.GitCommit", gitCommit),
		ldflagSet("main.BuildTime", buildTime),
		ldflagSet("claude-go/cmd.Version", version),
		ldflagSet("claude-go/internal/constants.Version", version),
	}
	for key, target := range buildEnvLdflagVars {
		value := strings.TrimSpace(projectEnv[key])
		if value == "" {
			continue
		}
		parts = append(parts, ldflagSet(target, value))
	}
	return strings.Join(parts, " ")
}

func ldflagSet(target, value string) string {
	return "-X " + shellQuote(target+"="+value)
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func loadProjectEnv(path string) map[string]string {
	values := map[string]string{}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("Warning: Could not read %s: %v\n", path, err)
		}
		return values
	}
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" || llmAPIEnvKeys[key] {
			continue
		}
		if _, allowed := buildEnvLdflagVars[key]; !allowed {
			continue
		}
		values[key] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return values
}

// buildPlatform builds for a specific platform
func buildPlatform(p Platform) error {
	fmt.Printf("Building for %s...\n", p)

	// Create dist directory
	distDir := "dist"
	if err := os.MkdirAll(distDir, 0755); err != nil {
		return fmt.Errorf("create dist dir: %w", err)
	}

	// Build command
	outputPath := filepath.Join(distDir, p.OutputName())
	ldflags := getLdflags()

	cmd := exec.Command("go", "build", "-ldflags", ldflags, "-o", outputPath, ".")
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("GOOS=%s", p.OS),
		fmt.Sprintf("GOARCH=%s", p.Arch),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	// Make executable on Unix
	if p.OS != "windows" {
		if err := os.Chmod(outputPath, 0755); err != nil {
			return fmt.Errorf("chmod: %w", err)
		}
	}

	fmt.Printf("✓ Built: %s\n", outputPath)
	return nil
}

// buildCurrent builds for current platform
func buildCurrent() error {
	fmt.Printf("Building %s (version: %s)...\n", binaryName, version)

	// Create build directory
	buildDir := "build"
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return fmt.Errorf("create build dir: %w", err)
	}

	// Determine output path
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	outputPath := filepath.Join(buildDir, binaryName+ext)

	// Build
	ldflags := getLdflags()
	cmd := exec.Command("go", "build", "-ldflags", ldflags, "-o", outputPath, ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	fmt.Printf("✓ Build complete: %s\n", outputPath)

	// Show version
	fmt.Printf("\n")
	if err := exec.Command(outputPath, "--version").Run(); err != nil {
		// Ignore error
	}

	return nil
}

// buildAll builds for all platforms
func buildAll() error {
	fmt.Printf("Building for all platforms...\n\n")

	for _, p := range platforms {
		if err := buildPlatform(p); err != nil {
			return err
		}
	}

	fmt.Printf("\n✓ All platform builds complete!\n")
	return nil
}

// release creates release archives
func release() error {
	fmt.Printf("Creating release (version: %s)...\n\n", version)

	// Clean dist directory
	distDir := "dist"
	os.RemoveAll(distDir)
	if err := os.MkdirAll(distDir, 0755); err != nil {
		return fmt.Errorf("create dist dir: %w", err)
	}

	// Build all platforms
	for _, p := range platforms {
		if err := buildPlatform(p); err != nil {
			return err
		}
	}

	fmt.Printf("\nCreating archives...\n")

	// Create archives
	for _, p := range platforms {
		binaryPath := filepath.Join(distDir, p.OutputName())
		archivePath := filepath.Join(distDir, p.ArchiveName())

		if p.OS == "windows" {
			if err := createZipArchive(binaryPath, archivePath); err != nil {
				return fmt.Errorf("create zip: %w", err)
			}
		} else {
			if err := createTarGzArchive(binaryPath, archivePath); err != nil {
				return fmt.Errorf("create tar.gz: %w", err)
			}
		}

		// Remove binary after archiving
		os.Remove(binaryPath)
		fmt.Printf("✓ Created: %s\n", archivePath)
	}

	// Create checksums
	fmt.Printf("\nCreating checksums...\n")
	if err := createChecksums(distDir); err != nil {
		return fmt.Errorf("create checksums: %w", err)
	}

	// List files
	fmt.Printf("\nRelease files:\n")
	files, _ := filepath.Glob(filepath.Join(distDir, "*"))
	for _, f := range files {
		info, _ := os.Stat(f)
		fmt.Printf("  %s (%d bytes)\n", filepath.Base(f), info.Size())
	}

	return nil
}

// createTarGzArchive creates a tar.gz archive
func createTarGzArchive(source, target string) error {
	file, err := os.Create(target)
	if err != nil {
		return err
	}
	defer file.Close()

	gzw := gzip.NewWriter(file)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	// Open source file
	srcFile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Get file info
	info, err := srcFile.Stat()
	if err != nil {
		return err
	}

	// Write header
	header := &tar.Header{
		Name:    filepath.Base(source),
		Size:    info.Size(),
		Mode:    int64(info.Mode()),
		ModTime: info.ModTime(),
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	// Write content
	_, err = io.Copy(tw, srcFile)
	return err
}

// createZipArchive creates a zip archive
func createZipArchive(source, target string) error {
	zipFile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Open source file
	srcFile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Get file info
	info, err := srcFile.Stat()
	if err != nil {
		return err
	}

	// Create zip header
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = filepath.Base(source)
	header.Method = zip.Deflate

	// Write header
	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}

	// Write content
	_, err = io.Copy(writer, srcFile)
	return err
}

// createChecksums creates checksums.txt
func createChecksums(dir string) error {
	// Find all archives
	patterns := []string{"*.tar.gz", "*.zip"}
	var files []string
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(filepath.Join(dir, pattern))
		files = append(files, matches...)
	}

	if len(files) == 0 {
		return nil
	}

	// Create checksums file
	checksumsFile := filepath.Join(dir, "checksums.txt")
	output, err := os.Create(checksumsFile)
	if err != nil {
		return err
	}
	defer output.Close()

	// Calculate and write checksums
	for _, file := range files {
		cmd := exec.Command("sha256sum", filepath.Base(file))
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			// Fallback to shasum on macOS
			cmd = exec.Command("shasum", "-a", "256", filepath.Base(file))
			cmd.Dir = dir
			out, err = cmd.Output()
			if err != nil {
				fmt.Printf("Warning: Could not calculate checksum for %s\n", file)
				continue
			}
		}
		output.Write(out)
	}

	return nil
}

// clean removes build artifacts
func clean() {
	fmt.Println("Cleaning build artifacts...")

	exec.Command("go", "clean").Run()
	os.RemoveAll("build")
	os.RemoveAll("dist")

	os.MkdirAll("build", 0755)
	os.MkdirAll("dist", 0755)

	fmt.Println("✓ Clean complete")
}

// runTests runs all tests
func runTests() error {
	fmt.Println("Running tests...")

	cmd := exec.Command("go", "test", "-v", "./...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tests failed: %w", err)
	}

	fmt.Println("✓ All tests passed")
	return nil
}

// printInfo prints build environment info
func printInfo() {
	fmt.Println("Build Environment Information")
	fmt.Println()
	fmt.Printf("  Go Version:    ")
	exec.Command("go", "version").Run()
	fmt.Println()
	fmt.Printf("  GOOS:          %s\n", runtime.GOOS)
	fmt.Printf("  GOARCH:        %s\n", runtime.GOARCH)
	fmt.Printf("  NumCPU:        %d\n", runtime.NumCPU())
	fmt.Println()
	fmt.Printf("  Binary Name:   %s\n", binaryName)
	fmt.Printf("  Version:       %s\n", version)
	fmt.Printf("  Git Commit:    %s\n", gitCommit)
	fmt.Printf("  Build Time:    %s\n", buildTime)
	fmt.Printf("  Project .env:  %d build-safe value(s), LLM API values excluded\n", len(projectEnv))
	fmt.Println()
	fmt.Println("Supported platforms:")
	for _, p := range platforms {
		fmt.Printf("  - %s\n", p)
	}
}
