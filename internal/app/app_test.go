package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AI4SciComp/asc-devtools/internal/config"
	"github.com/AI4SciComp/asc-devtools/internal/process"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type appRunner struct {
	calls    []process.Command
	failName string
	failCode int
}

func (r *appRunner) Run(_ context.Context, command process.Command) (process.Result, error) {
	r.calls = append(r.calls, command)
	joined := strings.Join(command.Args, " ")
	switch {
	case strings.Contains(joined, "rev-parse --is-inside-work-tree"):
		return process.Result{Stdout: "true\n"}, nil
	case strings.Contains(joined, "status --porcelain=v2"):
		return process.Result{Stdout: "# branch.head main\n# branch.upstream origin/main\n# branch.ab +1 -2\n"}, nil
	case strings.Contains(joined, "status --porcelain"):
		return process.Result{}, nil
	case strings.Contains(joined, "remote get-url"):
		return process.Result{Stdout: "git@github.com:AI4SciComp/asc-one.git\n"}, nil
	case strings.Contains(joined, "symbolic-ref"):
		return process.Result{Stdout: "main\n"}, nil
	case strings.Contains(joined, "@{upstream}"):
		return process.Result{Stdout: "origin/main\n"}, nil
	case command.Name == "git" && len(command.Args) > 0 && command.Args[0] == "clone":
		return process.Result{}, nil
	case strings.Contains(joined, "--list-presets"):
		return process.Result{Stdout: "  \"dev\"\n"}, nil
	case command.Name == r.failName && command.Stream:
		return process.Result{ExitCode: r.failCode}, &process.CommandError{Command: process.Describe(command), ExitCode: r.failCode}
	case command.Name == "ssh":
		return process.Result{Stderr: "successfully authenticated"}, nil
	case command.Name == "git" || command.Name == "cmake" || command.Name == "ctest":
		return process.Result{Stdout: command.Name + " version test\n"}, nil
	default:
		return process.Result{}, errors.New("unexpected command: " + command.Name + " " + joined)
	}
}

func appLoader(t *testing.T, environment map[string]string) config.Loader {
	t.Helper()
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	return config.Loader{
		LookupEnv: func(name string) (string, bool) {
			value, ok := environment[name]
			return value, ok
		},
		UserHomeDir: func() (string, error) { return home, nil },
		Getwd:       func() (string, error) { return filepath.Dir(home), nil },
	}
}

func runApp(t *testing.T, arguments []string, environment map[string]string, runner process.Runner, client *http.Client) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if runner == nil {
		runner = &appRunner{}
	}
	code := Run(context.Background(), arguments, Dependencies{
		Stdout:      &stdout,
		Stderr:      &stderr,
		Config:      appLoader(t, environment),
		Runner:      runner,
		HTTPClient:  client,
		APIBaseURL:  "https://api.test",
		LookupPath:  func(string) (string, error) { return "", os.ErrNotExist },
		VersionInfo: VersionInfo{Version: "0.1.0", Commit: "abc"},
	})
	return code, stdout.String(), stderr.String()
}

func apiClient(body string, status int) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})}
}

func TestHelpVersionAndInvalidInvocation(t *testing.T) {
	code, stdout, stderr := runApp(t, []string{"--help"}, nil, nil, nil)
	if code != ExitSuccess || !strings.Contains(stdout, "repo clone") || strings.Contains(stdout, "repo save") || strings.Contains(stdout, "\n  update ") || stderr != "" {
		t.Fatalf("help = %d, %q, %q", code, stdout, stderr)
	}
	code, stdout, stderr = runApp(t, []string{"--version"}, nil, nil, nil)
	if code != ExitSuccess || strings.TrimSpace(stdout) != "asc 0.1.0 commit=abc" || stderr != "" {
		t.Fatalf("version = %d, %q, %q", code, stdout, stderr)
	}
	code, stdout, stderr = runApp(t, []string{"unknown"}, nil, nil, nil)
	if code != ExitUsage || stdout != "" || !strings.Contains(stderr, "unknown command") {
		t.Fatalf("invalid = %d, %q, %q", code, stdout, stderr)
	}
	code, _, _ = runApp(t, []string{"repo", "list", "--help"}, nil, nil, nil)
	if code != ExitSuccess {
		t.Fatalf("repo list help code = %d", code)
	}
	for _, arguments := range [][]string{
		{"agent"},
		{"workflow"},
		{"cmake"},
		{"update"},
		{"repo", "save"},
		{"workspace", "init"},
	} {
		code, stdout, stderr = runApp(t, arguments, nil, nil, nil)
		if code != ExitUsage || stdout != "" || !strings.Contains(stderr, "asc: error:") {
			t.Fatalf("excluded command %v = %d, %q, %q", arguments, code, stdout, stderr)
		}
	}
}

func TestWorkspaceHasNoNetworkOrDiagnostics(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace with spaces")
	code, stdout, stderr := runApp(t, []string{"workspace"}, map[string]string{"ASC_WORKSPACE": workspace}, nil, nil)
	if code != ExitSuccess || strings.TrimSpace(stdout) != workspace || stderr != "" {
		t.Fatalf("workspace = %d, %q, %q", code, stdout, stderr)
	}
}

func TestRepositoryListJSON(t *testing.T) {
	body := `[{"name":"asc-z","archived":false,"private":true,"default_branch":"main"},{"name":"other","archived":false},{"name":"asc-a","archived":false}]`
	code, stdout, stderr := runApp(t, []string{"repo", "list", "--json"}, nil, nil, apiClient(body, http.StatusOK))
	if code != ExitSuccess || stderr != "" {
		t.Fatalf("repo list = %d, %q", code, stderr)
	}
	var repositories []map[string]any
	if err := json.Unmarshal([]byte(stdout), &repositories); err != nil {
		t.Fatal(err)
	}
	if len(repositories) != 2 || repositories[0]["name"] != "asc-a" || repositories[1]["name"] != "asc-z" {
		t.Fatalf("JSON repositories = %v", repositories)
	}
}

func TestRepositoryStatusJSONAndCloneRouting(t *testing.T) {
	workspace := t.TempDir()
	if err := os.Mkdir(filepath.Join(workspace, "asc-one"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &appRunner{}
	code, stdout, stderr := runApp(t, []string{"repo", "status", "asc-one", "--json"}, map[string]string{"ASC_WORKSPACE": workspace}, runner, nil)
	if code != ExitSuccess || stderr != "" || !strings.Contains(stdout, `"ahead": 1`) || !strings.Contains(stdout, `"behind": 2`) {
		t.Fatalf("status = %d, %q, %q", code, stdout, stderr)
	}
	body := `[{"name":"asc-two","archived":false,"ssh_url":"git@github.com:AI4SciComp/asc-two.git","clone_url":"https://github.com/AI4SciComp/asc-two.git"}]`
	code, stdout, stderr = runApp(t, []string{"repo", "clone", "asc-two", "--protocol", "https"}, map[string]string{"ASC_WORKSPACE": workspace}, runner, apiClient(body, http.StatusOK))
	if code != ExitSuccess || stderr != "" || !strings.Contains(stdout, "cloned") {
		t.Fatalf("clone = %d, %q, %q", code, stdout, stderr)
	}
	found := false
	for _, call := range runner.calls {
		if len(call.Args) >= 3 && call.Args[0] == "clone" && call.Args[2] == "https://github.com/AI4SciComp/asc-two.git" {
			found = true
		}
	}
	if !found {
		t.Fatalf("clone command not routed: %+v", runner.calls)
	}
}

func TestRepositorySyncDryRunAndCMakeExitPropagation(t *testing.T) {
	workspace := t.TempDir()
	repository := filepath.Join(workspace, "asc-one")
	if err := os.Mkdir(repository, 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &appRunner{}
	code, stdout, stderr := runApp(t, []string{"repo", "sync", "asc-one", "--dry-run"}, map[string]string{"ASC_WORKSPACE": workspace}, runner, nil)
	if code != ExitSuccess || stderr != "" || !strings.Contains(stdout, "planned") {
		t.Fatalf("sync dry run = %d, %q, %q", code, stdout, stderr)
	}
	for _, call := range runner.calls {
		if len(call.Args) > 2 && (call.Args[2] == "fetch" || call.Args[2] == "merge") {
			t.Fatalf("dry run executed mutation: %+v", call)
		}
	}

	if err := os.WriteFile(filepath.Join(repository, "CMakePresets.json"), []byte(`{"version":6}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "CMakeLists.txt"), []byte("cmake_minimum_required(VERSION 3.25)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner = &appRunner{}
	code, stdout, stderr = runApp(t, []string{"configure", "asc-one", "--preset", "dev"}, map[string]string{"ASC_WORKSPACE": workspace}, runner, nil)
	if code != ExitSuccess || stdout != "" || stderr != "" {
		t.Fatalf("configure = %d, %q, %q", code, stdout, stderr)
	}
	last := runner.calls[len(runner.calls)-1]
	if last.Name != "cmake" || strings.Join(last.Args, " ") != "--preset dev" || last.Dir != repository || !last.Stream {
		t.Fatalf("configure command = %+v", last)
	}

	runner = &appRunner{failName: "cmake", failCode: 7}
	code, stdout, stderr = runApp(t, []string{"build", "asc-one", "--preset", "dev"}, map[string]string{"ASC_WORKSPACE": workspace}, runner, nil)
	if code != 7 || stdout != "" || !strings.Contains(stderr, "command failed (7)") {
		t.Fatalf("build failure = %d, %q, %q", code, stdout, stderr)
	}
}

func TestDoctorJSONAndCompletion(t *testing.T) {
	workspace := t.TempDir()
	var stdout, stderr bytes.Buffer
	runner := &appRunner{}
	lookup := func(name string) (string, error) { return "/usr/bin/" + name, nil }
	code := Run(context.Background(), []string{"doctor", "--json"}, Dependencies{
		Stdout:     &stdout,
		Stderr:     &stderr,
		Config:     appLoader(t, map[string]string{"ASC_WORKSPACE": workspace}),
		Runner:     runner,
		HTTPClient: apiClient(`[]`, http.StatusOK),
		APIBaseURL: "https://api.test",
		LookupPath: lookup,
	})
	if code != ExitSuccess || stderr.String() != "" || !strings.Contains(stdout.String(), `"api-authentication"`) || !strings.Contains(stdout.String(), `"github-api"`) {
		t.Fatalf("doctor = %d, %q, %q", code, stdout.String(), stderr.String())
	}
	code, stdoutText, stderrText := runApp(t, []string{"completion", "bash"}, nil, nil, nil)
	if code != ExitSuccess || stderrText != "" || !strings.Contains(stdoutText, "complete -F _asc_completion asc") {
		t.Fatalf("completion = %d, %q, %q", code, stdoutText, stderrText)
	}
}
