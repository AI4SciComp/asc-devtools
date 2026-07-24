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
	"time"

	"github.com/AI4SciComp/asc-devtools/internal/config"
	"github.com/AI4SciComp/asc-devtools/internal/process"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type appRunner struct {
	calls []process.Command
}

func (r *appRunner) Run(_ context.Context, command process.Command) (process.Result, error) {
	r.calls = append(r.calls, command)
	joined := strings.Join(command.Args, " ")
	switch {
	case strings.Contains(joined, "rev-parse --is-inside-work-tree"):
		return process.Result{Stdout: "true\n"}, nil
	case strings.Contains(joined, "status --porcelain=v2"):
		return process.Result{Stdout: "# branch.head main\n# branch.upstream origin/main\n# branch.ab +1 -2\n"}, nil
	case command.Name == "git" && len(command.Args) > 0 && command.Args[0] == "clone":
		return process.Result{}, nil
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
	if code != ExitSuccess || !strings.Contains(stdout, "repo clone") || stderr != "" {
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
	code, stdout, stderr = runApp(t, []string{"cmake", "--help"}, nil, nil, nil)
	if code != ExitSuccess || !strings.Contains(stdout, "vendor {status|plan|apply}") || stderr != "" {
		t.Fatalf("cmake help = %d, %q, %q", code, stdout, stderr)
	}
	code, _, stderr = runApp(t, []string{"cmake", "workflow", "asc-cpp", "--configure-preset", "dev"}, nil, nil, nil)
	if code != ExitUsage || !strings.Contains(stderr, "all three workflow presets") {
		t.Fatalf("workflow validation = %d, %q", code, stderr)
	}
	code, stdout, stderr = runApp(t, []string{"update", "--help"}, nil, nil, nil)
	if code != ExitSuccess || !strings.Contains(stdout, "--check") || stderr != "" {
		t.Fatalf("update help = %d, %q, %q", code, stdout, stderr)
	}
	code, stdout, stderr = runApp(t, []string{"update", "--check"}, nil, nil, apiClient("not found", http.StatusNotFound))
	if code != ExitFailure || stdout != "" || !strings.Contains(stderr, "no published asc release") {
		t.Fatalf("update without release = %d, %q, %q", code, stdout, stderr)
	}
	code, stdout, stderr = runApp(t, []string{"repo", "save", "--help"}, nil, nil, nil)
	if code != ExitSuccess || !strings.Contains(stdout, "--message") || stderr != "" {
		t.Fatalf("repo save help = %d, %q, %q", code, stdout, stderr)
	}
	if repository, message, dryRun, yes, err := parseRepoSave([]string{"asc-one", "--message", "Save work", "--dry-run", "--yes"}); err != nil || repository != "asc-one" || message != "Save work" || !dryRun || !yes {
		t.Fatalf("parseRepoSave() = %q, %q, %v, %v, %v", repository, message, dryRun, yes, err)
	}
	if repository, message, dryRun, yes, err := parseRepoSave([]string{"asc-one", "--dry-run"}); err != nil || repository != "asc-one" || !dryRun || yes {
		t.Fatalf("parseRepoSave(default) = %q, %q, %v, %v, %v", repository, message, dryRun, yes, err)
	} else if _, err := time.Parse("Updated at 2006-01-02 15:04:05", message); err != nil {
		t.Fatalf("default save message = %q: %v", message, err)
	}
}

func TestWorkspaceHasNoNetworkOrDiagnostics(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace with spaces")
	code, stdout, stderr := runApp(t, []string{"workspace"}, map[string]string{"ASC_WORKSPACE": workspace}, nil, nil)
	if code != ExitSuccess || strings.TrimSpace(stdout) != workspace || stderr != "" {
		t.Fatalf("workspace = %d, %q, %q", code, stdout, stderr)
	}
}

func TestCoordinationCommands(t *testing.T) {
	root := filepath.Join(t.TempDir(), "AI4SciComp")
	environment := map[string]string{"ASC_WORKSPACE": root}

	code, stdout, stderr := runApp(t, []string{"workspace", "init", "--dry-run"}, environment, nil, nil)
	if code != ExitSuccess || stderr != "" || !strings.Contains(stdout, "create\t"+root) {
		t.Fatalf("workspace init dry-run = %d, %q, %q", code, stdout, stderr)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("workspace init --dry-run changed the filesystem: %v", err)
	}

	code, stdout, stderr = runApp(t, []string{"workspace", "init"}, environment, nil, nil)
	if code != ExitSuccess || stderr != "" || !strings.Contains(stdout, "created\t"+root) {
		t.Fatalf("workspace init = %d, %q, %q", code, stdout, stderr)
	}
	code, stdout, stderr = runApp(t, []string{"workspace", "validate", "--json"}, environment, nil, nil)
	if code != ExitSuccess || stderr != "" || !strings.Contains(stdout, `"status": "pass"`) {
		t.Fatalf("workspace validate = %d, %q, %q", code, stdout, stderr)
	}

	code, stdout, stderr = runApp(t, []string{"agent", "init", "reviewer", "--dry-run"}, environment, nil, nil)
	definition := filepath.Join(root, "workspace", "agents", "reviewer", "AGENT.md")
	if code != ExitSuccess || stderr != "" || !strings.Contains(stdout, definition) {
		t.Fatalf("agent init dry-run = %d, %q, %q", code, stdout, stderr)
	}
	if _, err := os.Stat(definition); !os.IsNotExist(err) {
		t.Fatalf("agent init --dry-run changed the filesystem: %v", err)
	}
	code, stdout, stderr = runApp(t, []string{"agent", "init", "reviewer"}, environment, nil, nil)
	if code != ExitSuccess || stderr != "" || !strings.Contains(stdout, "created\t"+definition) {
		t.Fatalf("agent init = %d, %q, %q", code, stdout, stderr)
	}
	code, _, stderr = runApp(t, []string{"agent", "init", "reviewer"}, environment, nil, nil)
	if code != ExitFailure || !strings.Contains(stderr, "agent already exists") {
		t.Fatalf("duplicate agent init = %d, %q", code, stderr)
	}

	workflowDirectory := filepath.Join(root, "workspace", "workflows", "software-development")
	if err := os.MkdirAll(workflowDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"schemaVersion":1,"name":"software-development","states":["proposed","done"],"initialState":"proposed","terminalStates":["done"],"transitions":[{"from":"proposed","to":"done"}]}`
	if err := os.WriteFile(filepath.Join(workflowDirectory, "workflow.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = runApp(t, []string{"workflow", "validate", "software-development", "--json"}, environment, nil, nil)
	if code != ExitSuccess || stderr != "" || !strings.Contains(stdout, `"status": "pass"`) {
		t.Fatalf("workflow validate = %d, %q, %q", code, stdout, stderr)
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
	body := `[{"name":"asc-two","archived":false,"ssh_url":"ssh-url","clone_url":"https-url"}]`
	code, stdout, stderr = runApp(t, []string{"repo", "clone", "asc-two", "--protocol", "https"}, map[string]string{"ASC_WORKSPACE": workspace}, runner, apiClient(body, http.StatusOK))
	if code != ExitSuccess || stderr != "" || !strings.Contains(stdout, "cloned") {
		t.Fatalf("clone = %d, %q, %q", code, stdout, stderr)
	}
	found := false
	for _, call := range runner.calls {
		if len(call.Args) >= 3 && call.Args[0] == "clone" && call.Args[2] == "https-url" {
			found = true
		}
	}
	if !found {
		t.Fatalf("clone command not routed: %+v", runner.calls)
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
