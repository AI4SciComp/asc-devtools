// Package cmake plans and runs repository-owned CMake and CTest presets.
package cmake

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/AI4SciComp/asc-devtools/internal/config"
	"github.com/AI4SciComp/asc-devtools/internal/process"
	"github.com/AI4SciComp/asc-devtools/internal/workspace"
)

// Manager validates, plans, and runs CMake workflow commands.
type Manager struct {
	Runner process.Runner
	Config config.Config
}

// Run executes one configure, build, or test operation.
func (m Manager) Run(ctx context.Context, operation, repositoryName, preset string) error {
	repositoryPath, err := m.validateRepository(ctx, repositoryName, operation == "configure")
	if err != nil {
		return err
	}
	if err := validateValue("preset", preset); err != nil {
		return err
	}
	if !presetFileExists(repositoryPath) {
		return fmt.Errorf("CMakePresets.json not found in %s", repositoryName)
	}

	listCommand, command, err := commands(operation, repositoryPath, preset)
	if err != nil {
		return err
	}
	listed, err := m.Runner.Run(ctx, listCommand)
	if err != nil {
		return fmt.Errorf("list %s presets in %s: %w", operation, repositoryName, err)
	}
	if !containsPreset(listed.Stdout, preset) {
		return fmt.Errorf("%s preset %q not found in %s", operation, preset, repositoryName)
	}
	if _, err := m.Runner.Run(ctx, command); err != nil {
		return fmt.Errorf("%s %s with preset %s: %w", operation, repositoryName, preset, err)
	}
	return nil
}

func (m Manager) validateRepository(ctx context.Context, repositoryName string, requireProject bool) (string, error) {
	if err := config.ValidateRepositoryName(repositoryName, m.Config); err != nil {
		return "", err
	}
	repositoryPath, err := workspace.DirectChild(m.Config.Workspace, repositoryName)
	if err != nil {
		return "", err
	}
	if !workspace.IsGitWorktree(ctx, m.Runner, repositoryPath) {
		return "", fmt.Errorf("local Git working tree not found: %s", repositoryName)
	}
	if requireProject {
		info, err := os.Stat(filepath.Join(repositoryPath, "CMakeLists.txt"))
		if err != nil || !info.Mode().IsRegular() {
			return "", fmt.Errorf("CMakeLists.txt not found in %s", repositoryName)
		}
	}
	return repositoryPath, nil
}

func commands(operation, repositoryPath, preset string) (process.Command, process.Command, error) {
	if err := validateValue("preset", preset); err != nil {
		return process.Command{}, process.Command{}, err
	}
	var listCommand, command process.Command
	switch operation {
	case "configure":
		listCommand = process.Command{Name: "cmake", Args: []string{"--list-presets"}, Dir: repositoryPath}
		command = process.Command{Name: "cmake", Args: []string{"--preset", preset}, Dir: repositoryPath, Stream: true}
	case "build":
		listCommand = process.Command{Name: "cmake", Args: []string{"--list-presets=build"}, Dir: repositoryPath}
		command = process.Command{Name: "cmake", Args: []string{"--build", "--preset", preset}, Dir: repositoryPath, Stream: true}
	case "test":
		listCommand = process.Command{Name: "ctest", Args: []string{"--list-presets"}, Dir: repositoryPath}
		command = process.Command{Name: "ctest", Args: []string{"--preset", preset}, Dir: repositoryPath, Stream: true}
	default:
		return process.Command{}, process.Command{}, fmt.Errorf("unsupported CMake operation: %s", operation)
	}
	return listCommand, command, nil
}

func validateValue(kind, value string) error {
	if value == "" {
		return fmt.Errorf("a %s is required", kind)
	}
	if strings.HasPrefix(value, "-") || strings.IndexByte(value, 0) >= 0 || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return fmt.Errorf("invalid %s %q", kind, value)
	}
	return nil
}

func presetFileExists(repositoryPath string) bool {
	for _, name := range []string{"CMakePresets.json", "CMakeUserPresets.json"} {
		if info, err := os.Stat(filepath.Join(repositoryPath, name)); err == nil && info.Mode().IsRegular() {
			return true
		}
	}
	return false
}

func containsPreset(output, preset string) bool {
	for _, line := range strings.Split(output, "\n") {
		start := strings.IndexByte(line, '"')
		if start < 0 {
			continue
		}
		end := strings.IndexByte(line[start+1:], '"')
		if end < 0 {
			continue
		}
		if line[start+1:start+1+end] == preset {
			return true
		}
	}
	return false
}
