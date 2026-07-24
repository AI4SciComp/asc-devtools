package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/AI4SciComp/asc-devtools/internal/config"
	"github.com/AI4SciComp/asc-devtools/internal/process"
)

type fakeRunner struct{}

func (fakeRunner) Run(_ context.Context, command process.Command) (process.Result, error) {
	if _, err := os.Stat(filepath.Join(command.Args[1], ".git")); err == nil {
		return process.Result{Stdout: "true\n"}, nil
	}
	return process.Result{}, os.ErrNotExist
}

func TestDirectChild(t *testing.T) {
	root := t.TempDir()
	path, err := DirectChild(root, "asc-cpp")
	if err != nil || path != filepath.Join(root, "asc-cpp") {
		t.Fatalf("DirectChild() = %q, %v", path, err)
	}
	if _, err := DirectChild(root, "../outside"); err == nil {
		t.Fatal("DirectChild traversal succeeded")
	}
}

func TestDiscoverDirectChildrenAndSkipSymlinks(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"asc-one", ".github", "other"} {
		if err := os.MkdirAll(filepath.Join(root, name, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	outside := t.TempDir()
	if err := os.Mkdir(filepath.Join(outside, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "asc-link")); err != nil {
		t.Fatal(err)
	}
	config := config.Config{Workspace: root, RepositoryPrefix: "asc-", IncludeDotGitHub: true}
	names, err := Discover(context.Background(), fakeRunner{}, config)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != ".github" || names[1] != "asc-one" {
		t.Fatalf("Discover() = %v", names)
	}
}
