package cmake

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/AI4SciComp/asc-devtools/internal/config"
	"github.com/AI4SciComp/asc-devtools/internal/process"
)

type fakeRunner struct {
	calls      []process.Command
	actualFail error
}

func (r *fakeRunner) Run(_ context.Context, command process.Command) (process.Result, error) {
	r.calls = append(r.calls, command)
	joined := strings.Join(command.Args, " ")
	if strings.Contains(joined, "rev-parse --is-inside-work-tree") {
		return process.Result{Stdout: "true\n"}, nil
	}
	if strings.HasPrefix(joined, "--list-presets=") {
		return process.Result{Stdout: `  "dev"\n  "release"\n`}, nil
	}
	if r.actualFail != nil {
		return process.Result{}, r.actualFail
	}
	return process.Result{}, nil
}

func setupManager(t *testing.T) (Manager, *fakeRunner, string) {
	t.Helper()
	workspace := t.TempDir()
	repository := filepath.Join(workspace, "asc-cpp")
	if err := os.Mkdir(repository, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "CMakePresets.json"), []byte(`{"version":6}`), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}
	manager := Manager{Runner: runner, Config: config.Config{Workspace: workspace, RepositoryPrefix: "asc-", IncludeDotGitHub: true}}
	return manager, runner, repository
}

func TestRunCommands(t *testing.T) {
	tests := []struct {
		operation string
		preset    string
		name      string
		args      []string
	}{
		{"configure", "dev", "cmake", []string{"--preset", "dev"}},
		{"build", "release", "cmake", []string{"--build", "--preset", "release"}},
		{"test", "dev", "ctest", []string{"--preset", "dev"}},
	}
	for _, test := range tests {
		t.Run(test.operation, func(t *testing.T) {
			manager, runner, repository := setupManager(t)
			if err := manager.Run(context.Background(), test.operation, "asc-cpp", test.preset); err != nil {
				t.Fatal(err)
			}
			actual := runner.calls[len(runner.calls)-1]
			if actual.Name != test.name || !reflect.DeepEqual(actual.Args, test.args) || actual.Dir != repository || !actual.Stream {
				t.Fatalf("command = %+v", actual)
			}
		})
	}
}

func TestRunValidationAndFailure(t *testing.T) {
	manager, runner, repository := setupManager(t)
	if err := manager.Run(context.Background(), "build", "asc-cpp", ""); err == nil || !strings.Contains(err.Error(), "preset is required") {
		t.Fatalf("missing preset error = %v", err)
	}
	if err := os.Remove(filepath.Join(repository, "CMakePresets.json")); err != nil {
		t.Fatal(err)
	}
	if err := manager.Run(context.Background(), "build", "asc-cpp", "dev"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing file error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repository, "CMakeUserPresets.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	runner.actualFail = &process.CommandError{ExitCode: 7, Command: "cmake"}
	err := manager.Run(context.Background(), "build", "asc-cpp", "dev")
	if err == nil || process.ExitCode(err, 1) != 7 {
		t.Fatalf("process failure = %v", err)
	}
	if !errors.As(err, new(*process.CommandError)) {
		t.Fatalf("error does not wrap CommandError: %v", err)
	}
}
