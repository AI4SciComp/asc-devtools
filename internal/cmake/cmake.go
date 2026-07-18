// Package cmake runs repository-owned CMake and CTest presets.
package cmake

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AI4SciComp/asc-devtools/internal/config"
	"github.com/AI4SciComp/asc-devtools/internal/process"
	"github.com/AI4SciComp/asc-devtools/internal/workspace"
)

// Manager validates and runs CMake workflow commands.
type Manager struct {
	Runner process.Runner
	Config config.Config
}

// Run executes one configure, build, or test preset operation.
func (m Manager) Run(ctx context.Context, operation, repositoryName, preset string) error {
	if err := config.ValidateRepositoryName(repositoryName, m.Config); err != nil {
		return err
	}
	if preset == "" {
		return errors.New("a preset is required; use --preset NAME")
	}
	if strings.HasPrefix(preset, "-") || strings.ContainsAny(preset, "/\\\\ \t\r\n") {
		return fmt.Errorf("invalid preset name %q", preset)
	}
	repositoryPath, err := workspace.DirectChild(m.Config.Workspace, repositoryName)
	if err != nil {
		return err
	}
	if !workspace.IsGitWorktree(ctx, m.Runner, repositoryPath) {
		return fmt.Errorf("local Git working tree not found: %s", repositoryName)
	}
	if !presetFileExists(repositoryPath) {
		return fmt.Errorf("CMakePresets.json or CMakeUserPresets.json not found in %s", repositoryName)
	}

	presetType := operation
	listCommand := process.Command{Name: "cmake", Args: []string{"--list-presets=" + presetType}, Dir: repositoryPath}
	listed, err := m.Runner.Run(ctx, listCommand)
	if err != nil {
		return fmt.Errorf("list %s presets in %s: %w", presetType, repositoryName, err)
	}
	if !containsPreset(listed.Stdout, preset) {
		return fmt.Errorf("%s preset %q not found in %s", presetType, preset, repositoryName)
	}

	var command process.Command
	switch operation {
	case "configure":
		command = process.Command{Name: "cmake", Args: []string{"--preset", preset}, Dir: repositoryPath, Stream: true}
	case "build":
		command = process.Command{Name: "cmake", Args: []string{"--build", "--preset", preset}, Dir: repositoryPath, Stream: true}
	case "test":
		command = process.Command{Name: "ctest", Args: []string{"--preset", preset}, Dir: repositoryPath, Stream: true}
	default:
		return fmt.Errorf("unsupported CMake operation: %s", operation)
	}
	if _, err := m.Runner.Run(ctx, command); err != nil {
		return fmt.Errorf("%s %s with preset %s: %w", operation, repositoryName, preset, err)
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
	needle := `"` + preset + `"`
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, needle) {
			return true
		}
	}
	return false
}
