// Package cmake plans and runs repository-owned CMake and CTest presets.
package cmake

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/AI4SciComp/asc-devtools/internal/config"
	"github.com/AI4SciComp/asc-devtools/internal/process"
	"github.com/AI4SciComp/asc-devtools/internal/workspace"
)

// Manager validates, plans, and runs CMake workflow commands.
type Manager struct {
	Runner   process.Runner
	Config   config.Config
	Reporter io.Writer
}

// Options describes one configure, build, or test operation.
type Options struct {
	Preset          string
	Targets         []string
	Label           string
	OutputOnFailure bool
}

// WorkflowOptions describes the three explicit presets in a workflow.
type WorkflowOptions struct {
	ConfigurePreset string
	BuildPreset     string
	TestPreset      string
}

// Presets is the stable result returned by the grouped presets command.
type Presets struct {
	Repository string   `json:"repository"`
	Configure  []string `json:"configure"`
	Build      []string `json:"build"`
	Test       []string `json:"test"`
}

// Run retains the v0.1 API used by the legacy top-level aliases.
func (m Manager) Run(ctx context.Context, operation, repositoryName, preset string) error {
	return m.RunOperation(ctx, operation, repositoryName, Options{Preset: preset})
}

// RunOperation executes one configure, build, or test operation.
func (m Manager) RunOperation(ctx context.Context, operation, repositoryName string, options Options) error {
	repositoryPath, err := m.validateRepository(ctx, repositoryName, operation == "configure")
	if err != nil {
		return err
	}
	if err := validateValue("preset", options.Preset); err != nil {
		return err
	}
	if !presetFileExists(repositoryPath) {
		return fmt.Errorf("CMakePresets.json not found in %s", repositoryName)
	}
	for _, target := range options.Targets {
		if err := validateValue("target", target); err != nil {
			return err
		}
	}
	if options.Label != "" {
		if err := validateValue("label", options.Label); err != nil {
			return err
		}
	}

	listCommand, command, err := commands(operation, repositoryPath, options)
	if err != nil {
		return err
	}
	listed, err := m.Runner.Run(ctx, listCommand)
	if err != nil {
		return fmt.Errorf("list %s presets in %s: %w", operation, repositoryName, err)
	}
	if !containsPreset(listed.Stdout, options.Preset) {
		return fmt.Errorf("%s preset %q not found in %s", operation, options.Preset, repositoryName)
	}
	if _, err := m.Runner.Run(ctx, command); err != nil {
		return fmt.Errorf("%s %s with preset %s: %w", operation, repositoryName, options.Preset, err)
	}
	return nil
}

// Workflow executes configure, build, and test, stopping at the first failure.
func (m Manager) Workflow(ctx context.Context, repositoryName string, options WorkflowOptions) error {
	if _, err := m.validateRepository(ctx, repositoryName, true); err != nil {
		return err
	}
	stages := []struct {
		name   string
		preset string
	}{
		{name: "configure", preset: options.ConfigurePreset},
		{name: "build", preset: options.BuildPreset},
		{name: "test", preset: options.TestPreset},
	}
	for _, stage := range stages {
		repositoryPath, _ := workspace.DirectChild(m.Config.Workspace, repositoryName)
		_, command, err := commands(stage.name, repositoryPath, Options{Preset: stage.preset})
		if err == nil && m.Reporter != nil {
			fmt.Fprintf(m.Reporter, "cmake workflow: %s: %s\n", stage.name, process.Describe(command))
		}
		if err := m.RunOperation(ctx, stage.name, repositoryName, Options{Preset: stage.preset}); err != nil {
			return fmt.Errorf("workflow %s stage: %w", stage.name, err)
		}
	}
	return nil
}

// ListPresets delegates preset interpretation to CMake and CTest.
func (m Manager) ListPresets(ctx context.Context, repositoryName string) (Presets, error) {
	repositoryPath, err := m.validateRepository(ctx, repositoryName, false)
	if err != nil {
		return Presets{}, err
	}
	if !presetFileExists(repositoryPath) {
		return Presets{}, fmt.Errorf("CMakePresets.json not found in %s", repositoryName)
	}
	result := Presets{Repository: repositoryName, Configure: []string{}, Build: []string{}, Test: []string{}}
	queries := []struct {
		command process.Command
		target  *[]string
	}{
		{process.Command{Name: "cmake", Args: []string{"--list-presets"}, Dir: repositoryPath}, &result.Configure},
		{process.Command{Name: "cmake", Args: []string{"--list-presets=build"}, Dir: repositoryPath}, &result.Build},
		{process.Command{Name: "ctest", Args: []string{"--list-presets"}, Dir: repositoryPath}, &result.Test},
	}
	for _, query := range queries {
		listed, err := m.Runner.Run(ctx, query.command)
		if err != nil {
			return Presets{}, fmt.Errorf("list presets with %s: %w", query.command.Name, err)
		}
		*query.target = parsePresetNames(listed.Stdout)
	}
	return result, nil
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

func commands(operation, repositoryPath string, options Options) (process.Command, process.Command, error) {
	if err := validateValue("preset", options.Preset); err != nil {
		return process.Command{}, process.Command{}, err
	}
	var listCommand, command process.Command
	switch operation {
	case "configure":
		listCommand = process.Command{Name: "cmake", Args: []string{"--list-presets"}, Dir: repositoryPath}
		command = process.Command{Name: "cmake", Args: []string{"--preset", options.Preset}, Dir: repositoryPath, Stream: true}
	case "build":
		listCommand = process.Command{Name: "cmake", Args: []string{"--list-presets=build"}, Dir: repositoryPath}
		arguments := []string{"--build", "--preset", options.Preset}
		if len(options.Targets) > 0 {
			arguments = append(arguments, "--target")
			arguments = append(arguments, options.Targets...)
		}
		command = process.Command{Name: "cmake", Args: arguments, Dir: repositoryPath, Stream: true}
	case "test":
		listCommand = process.Command{Name: "ctest", Args: []string{"--list-presets"}, Dir: repositoryPath}
		arguments := []string{"--preset", options.Preset}
		if options.OutputOnFailure {
			arguments = append(arguments, "--output-on-failure")
		}
		if options.Label != "" {
			arguments = append(arguments, "-L", options.Label)
		}
		command = process.Command{Name: "ctest", Args: arguments, Dir: repositoryPath, Stream: true}
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
	for _, name := range parsePresetNames(output) {
		if name == preset {
			return true
		}
	}
	return false
}

func parsePresetNames(output string) []string {
	seen := make(map[string]bool)
	var names []string
	for _, line := range strings.Split(output, "\n") {
		start := strings.IndexByte(line, '"')
		if start < 0 {
			continue
		}
		end := strings.IndexByte(line[start+1:], '"')
		if end < 0 {
			continue
		}
		name := line[start+1 : start+1+end]
		if name != "" && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
