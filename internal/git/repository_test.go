package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/AI4SciComp/asc-devtools/internal/config"
	"github.com/AI4SciComp/asc-devtools/internal/github"
	"github.com/AI4SciComp/asc-devtools/internal/process"
)

type functionRunner struct {
	run   func(process.Command) (process.Result, error)
	calls []process.Command
}

func (r *functionRunner) Run(_ context.Context, command process.Command) (process.Result, error) {
	r.calls = append(r.calls, command)
	return r.run(command)
}

func testConfig(workspace string) config.Config {
	return config.Config{
		Organization:     "AI4SciComp",
		Workspace:        workspace,
		RepositoryPrefix: "asc-",
		IncludeDotGitHub: true,
		CloneProtocol:    "ssh",
		Remote:           "origin",
	}
}

func TestCloneUsesAPIURLAndRefusesConflicts(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "asc-data"), []byte("owned"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &functionRunner{run: func(command process.Command) (process.Result, error) {
		if len(command.Args) > 0 && command.Args[0] == "clone" {
			return process.Result{}, nil
		}
		return process.Result{}, errors.New("unexpected command")
	}}
	repositories := []github.Repository{
		{Name: "asc-one", SSHURL: "git@github.com:AI4SciComp/asc-one.git", CloneURL: "https://github.com/AI4SciComp/asc-one.git"},
		{Name: "asc-data", SSHURL: "git@github.com:AI4SciComp/asc-data.git"},
	}
	operations := (Manager{Runner: runner, Config: testConfig(workspace)}).Clone(context.Background(), repositories, nil, "ssh")
	if len(operations) != 2 || operations[0].Outcome != "failed" || operations[1].Outcome != "cloned" {
		t.Fatalf("Clone() = %+v", operations)
	}
	want := []string{"clone", "--", "git@github.com:AI4SciComp/asc-one.git", filepath.Join(workspace, "asc-one")}
	if len(runner.calls) != 1 || !reflect.DeepEqual(runner.calls[0].Args, want) || !runner.calls[0].Stream {
		t.Fatalf("clone call = %+v", runner.calls)
	}
	content, err := os.ReadFile(filepath.Join(workspace, "asc-data"))
	if err != nil || string(content) != "owned" {
		t.Fatalf("conflict was modified: %q, %v", content, err)
	}
}

func TestCloneExistingRepositoryRequiresExpectedRemote(t *testing.T) {
	workspace := t.TempDir()
	destination := filepath.Join(workspace, "asc-one")
	if err := os.Mkdir(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	repository := github.Repository{Name: "asc-one", SSHURL: "git@github.com:AI4SciComp/asc-one.git"}
	remote := repository.SSHURL
	runner := &functionRunner{run: func(command process.Command) (process.Result, error) {
		joined := strings.Join(command.Args, " ")
		if strings.Contains(joined, "rev-parse --is-inside-work-tree") {
			return process.Result{Stdout: "true\n"}, nil
		}
		if strings.Contains(joined, "remote get-url") {
			return process.Result{Stdout: remote + "\n"}, nil
		}
		return process.Result{}, errors.New("unexpected command")
	}}
	manager := Manager{Runner: runner, Config: testConfig(workspace)}
	operations := manager.Clone(context.Background(), []github.Repository{repository}, nil, "ssh")
	if operations[0].Outcome != "already-present" {
		t.Fatalf("Clone() = %+v", operations)
	}
	remote = "git@github.com:Other/repository.git"
	operations = manager.Clone(context.Background(), []github.Repository{repository}, nil, "ssh")
	if operations[0].Outcome != "failed" || !strings.Contains(operations[0].Detail, "does not match") {
		t.Fatalf("Clone() mismatch = %+v", operations)
	}
}

func TestValidateCloneURL(t *testing.T) {
	tests := []struct {
		protocol string
		url      string
		wantErr  bool
	}{
		{"ssh", "git@github.com:AI4SciComp/asc-one.git", false},
		{"https", "https://github.com/AI4SciComp/asc-one.git", false},
		{"ssh", "ext::sh -c unsafe", true},
		{"https", "file:///tmp/repository", true},
		{"ftp", "https://github.com/AI4SciComp/asc-one.git", true},
	}
	for _, test := range tests {
		err := validateCloneURL(test.url, test.protocol, "AI4SciComp", "asc-one")
		if (err != nil) != test.wantErr {
			t.Errorf("validateCloneURL(%q, %q) error = %v", test.url, test.protocol, err)
		}
	}
}

func TestCloneRefusesDestinationSymlink(t *testing.T) {
	workspace := t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(workspace, "asc-one")); err != nil {
		t.Fatal(err)
	}
	runner := &functionRunner{run: func(process.Command) (process.Result, error) {
		return process.Result{}, errors.New("runner must not be called")
	}}
	repositories := []github.Repository{{
		Name:   "asc-one",
		SSHURL: "git@github.com:AI4SciComp/asc-one.git",
	}}
	operations := (Manager{Runner: runner, Config: testConfig(workspace)}).Clone(context.Background(), repositories, nil, "ssh")
	if len(operations) != 1 || operations[0].Outcome != "failed" || !strings.Contains(operations[0].Detail, "symbolic link") {
		t.Fatalf("Clone() = %+v", operations)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("runner calls = %+v", runner.calls)
	}
}

func TestParseStatus(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		status Status
	}{
		{
			name:   "clean tracking",
			input:  "# branch.oid abc\n# branch.head main\n# branch.upstream origin/main\n# branch.ab +2 -3\n",
			status: Status{Name: "asc-one", Branch: "main", Upstream: "origin/main", Ahead: 2, Behind: 3, Clean: true},
		},
		{
			name:   "detached dirty",
			input:  "# branch.oid abc\n# branch.head (detached)\n1 .M N... 100644 100644 100644 abc abc tracked.txt\n? untracked.txt\n",
			status: Status{Name: "asc-one", Branch: "HEAD", Detached: true, Clean: false, Changes: 2},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, err := ParseStatus("asc-one", test.input)
			if err != nil || !reflect.DeepEqual(status, test.status) {
				t.Fatalf("ParseStatus() = %+v, %v; want %+v", status, err, test.status)
			}
		})
	}
	if _, err := ParseStatus("asc-one", "# branch.ab -1 +2\n"); err == nil {
		t.Fatal("ParseStatus accepted malformed ahead/behind signs")
	}
}

func TestSyncDryRunAndUnsafeStates(t *testing.T) {
	for _, test := range []struct {
		name          string
		status        string
		branchError   bool
		upstreamError bool
		upstream      string
		want          string
	}{
		{name: "dirty", status: "?? file\n", want: "skipped"},
		{name: "detached", branchError: true, want: "skipped"},
		{name: "no upstream", upstreamError: true, want: "skipped"},
		{name: "wrong remote", upstream: "fork/main", want: "skipped"},
		{name: "clean dry run", want: "planned"},
	} {
		t.Run(test.name, func(t *testing.T) {
			workspace := t.TempDir()
			path := filepath.Join(workspace, "asc-one")
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatal(err)
			}
			runner := &functionRunner{run: func(command process.Command) (process.Result, error) {
				joined := strings.Join(command.Args, " ")
				switch {
				case strings.Contains(joined, "rev-parse --is-inside-work-tree"):
					return process.Result{Stdout: "true\n"}, nil
				case strings.Contains(joined, "status --porcelain"):
					return process.Result{Stdout: test.status}, nil
				case strings.Contains(joined, "remote get-url"):
					return process.Result{Stdout: "url\n"}, nil
				case strings.Contains(joined, "symbolic-ref"):
					if test.branchError {
						return process.Result{}, errors.New("detached")
					}
					return process.Result{Stdout: "main\n"}, nil
				case strings.Contains(joined, "@{upstream}"):
					if test.upstreamError {
						return process.Result{}, errors.New("no upstream")
					}
					upstream := test.upstream
					if upstream == "" {
						upstream = "origin/main"
					}
					return process.Result{Stdout: upstream + "\n"}, nil
				default:
					return process.Result{}, errors.New("unexpected command: " + joined)
				}
			}}
			operations := (Manager{Runner: runner, Config: testConfig(workspace)}).Sync(context.Background(), []string{"asc-one"}, true)
			if operations[0].Outcome != test.want {
				t.Fatalf("Sync() = %+v", operations)
			}
			for _, call := range runner.calls {
				if len(call.Args) > 2 && (call.Args[2] == "fetch" || call.Args[2] == "merge") {
					t.Fatalf("dry run executed mutation: %+v", call)
				}
			}
		})
	}
}

func TestSyncRealFastForwardAndDivergence(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	directory := t.TempDir()
	bare := filepath.Join(directory, "remote.git")
	seed := filepath.Join(directory, "seed")
	workspace := filepath.Join(directory, "workspace")
	runGit(t, directory, "init", "-q", "--bare", bare)
	initRepository(t, seed)
	runGit(t, seed, "remote", "add", "origin", bare)
	runGit(t, seed, "push", "-qu", "origin", "HEAD")
	if err := os.Mkdir(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, directory, "clone", "-q", bare, filepath.Join(workspace, "asc-one"))
	clone := filepath.Join(workspace, "asc-one")
	runGit(t, clone, "config", "user.name", "Asc Tests")
	runGit(t, clone, "config", "user.email", "asc-tests@example.invalid")
	if err := os.WriteFile(filepath.Join(seed, "tracked.txt"), []byte("remote\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, seed, "commit", "-qam", "remote")
	runGit(t, seed, "push", "-q")
	manager := Manager{Runner: process.OSRunner{}, Config: testConfig(workspace)}
	operations := manager.Sync(context.Background(), []string{"asc-one"}, false)
	if operations[0].Outcome != "updated" {
		t.Fatalf("fast-forward Sync() = %+v", operations)
	}
	if err := os.WriteFile(filepath.Join(clone, "local.txt"), []byte("local\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, clone, "add", "local.txt")
	runGit(t, clone, "commit", "-qm", "local")
	if err := os.WriteFile(filepath.Join(seed, "other.txt"), []byte("other\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, seed, "add", "other.txt")
	runGit(t, seed, "commit", "-qm", "other")
	runGit(t, seed, "push", "-q")
	operations = manager.Sync(context.Background(), []string{"asc-one"}, false)
	if operations[0].Outcome != "skipped" || !strings.Contains(operations[0].Detail, "fast-forward refused") {
		t.Fatalf("diverged Sync() = %+v", operations)
	}
}

func initRepository(t *testing.T, path string) {
	t.Helper()
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, path, "init", "-q")
	runGit(t, path, "config", "user.name", "Asc Tests")
	runGit(t, path, "config", "user.email", "asc-tests@example.invalid")
	if err := os.WriteFile(filepath.Join(path, "tracked.txt"), []byte("initial\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, path, "add", "tracked.txt")
	runGit(t, path, "commit", "-qm", "initial")
}

func runGit(t *testing.T, directory string, arguments ...string) {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = directory
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", arguments, err, output)
	}
}
