// Package workspace validates and discovers direct-child repositories.
package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/AI4SciComp/asc-devtools/internal/config"
	"github.com/AI4SciComp/asc-devtools/internal/process"
)

// DirectChild returns a safe direct-child destination for a repository name.
func DirectChild(root, name string) (string, error) {
	destination := filepath.Join(root, name)
	relative, err := filepath.Rel(root, destination)
	if err != nil {
		return "", fmt.Errorf("resolve repository destination: %w", err)
	}
	if relative != name || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("repository destination escapes workspace: %s", name)
	}
	return destination, nil
}

// IsGitWorktree reports whether a path is a Git working tree.
func IsGitWorktree(ctx context.Context, runner process.Runner, path string) bool {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return false
	}
	result, err := runner.Run(ctx, process.Command{Name: "git", Args: []string{"-C", path, "rev-parse", "--is-inside-work-tree"}})
	return err == nil && strings.TrimSpace(result.Stdout) == "true"
}

// Discover returns managed Git worktrees that are direct workspace children.
func Discover(ctx context.Context, runner process.Runner, cfg config.Config) ([]string, error) {
	entries, err := os.ReadDir(cfg.Workspace)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read workspace %s: %w", cfg.Workspace, err)
	}
	var names []string
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		if err := config.ValidateRepositoryName(entry.Name(), cfg); err != nil {
			continue
		}
		path, err := DirectChild(cfg.Workspace, entry.Name())
		if err != nil {
			continue
		}
		if IsGitWorktree(ctx, runner, path) {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}
