// Package git implements safe local Git repository operations.
package git

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/AI4SciComp/asc-devtools/internal/config"
	"github.com/AI4SciComp/asc-devtools/internal/github"
	"github.com/AI4SciComp/asc-devtools/internal/process"
	"github.com/AI4SciComp/asc-devtools/internal/workspace"
)

// Manager performs Git operations within one configured workspace.
type Manager struct {
	Runner process.Runner
	Config config.Config
}

// Operation describes one clone or synchronization outcome.
type Operation struct {
	Name    string   `json:"name"`
	Outcome string   `json:"outcome"`
	Detail  string   `json:"detail,omitempty"`
	Plan    []string `json:"plan,omitempty"`
}

// Status is the stable repository status representation.
type Status struct {
	Name     string `json:"name"`
	Branch   string `json:"branch,omitempty"`
	Detached bool   `json:"detached"`
	Upstream string `json:"upstream,omitempty"`
	Ahead    int    `json:"ahead"`
	Behind   int    `json:"behind"`
	Clean    bool   `json:"clean"`
	Changes  int    `json:"changes"`
	Error    string `json:"error,omitempty"`
}

type target struct {
	name string
	path string
	err  error
}

// Clone clones selected API repositories without replacing existing paths.
func (m Manager) Clone(ctx context.Context, repositories []github.Repository, requested []string, protocol string) []Operation {
	available := make(map[string]github.Repository, len(repositories))
	for _, repository := range repositories {
		available[repository.Name] = repository
	}
	var names []string
	if len(requested) == 0 {
		for name := range available {
			names = append(names, name)
		}
		sort.Strings(names)
	} else {
		names = append(names, requested...)
	}
	if err := os.MkdirAll(m.Config.Workspace, 0o755); err != nil {
		return []Operation{{Name: "workspace", Outcome: "failed", Detail: err.Error()}}
	}
	operations := make([]Operation, 0, len(names))
	for _, name := range names {
		if err := config.ValidateRepositoryName(name, m.Config); err != nil {
			operations = append(operations, Operation{Name: name, Outcome: "failed", Detail: err.Error()})
			continue
		}
		repository, ok := available[name]
		if !ok {
			operations = append(operations, Operation{Name: name, Outcome: "failed", Detail: "repository was not returned by the organization API"})
			continue
		}
		destination, err := workspace.DirectChild(m.Config.Workspace, name)
		if err != nil {
			operations = append(operations, Operation{Name: name, Outcome: "failed", Detail: err.Error()})
			continue
		}
		if info, err := os.Lstat(destination); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				operations = append(operations, Operation{Name: name, Outcome: "failed", Detail: "destination is a symbolic link"})
				continue
			}
			if !workspace.IsGitWorktree(ctx, m.Runner, destination) {
				operations = append(operations, Operation{Name: name, Outcome: "failed", Detail: "destination exists and is not a Git working tree"})
				continue
			}
			if !m.expectedRemote(ctx, destination, repository) {
				operations = append(operations, Operation{Name: name, Outcome: "failed", Detail: "existing working tree remote does not match the organization repository"})
				continue
			}
			operations = append(operations, Operation{Name: name, Outcome: "already-present"})
			continue
		} else if !os.IsNotExist(err) {
			operations = append(operations, Operation{Name: name, Outcome: "failed", Detail: err.Error()})
			continue
		}
		cloneURL := repository.SSHURL
		if protocol == "https" {
			cloneURL = repository.CloneURL
		}
		if cloneURL == "" {
			operations = append(operations, Operation{Name: name, Outcome: "failed", Detail: protocol + " clone URL is missing from the API response"})
			continue
		}
		command := process.Command{Name: "git", Args: []string{"clone", "--", cloneURL, destination}, Stream: true}
		if _, err := m.Runner.Run(ctx, command); err != nil {
			operations = append(operations, Operation{Name: name, Outcome: "failed", Detail: err.Error()})
			continue
		}
		operations = append(operations, Operation{Name: name, Outcome: "cloned"})
	}
	return operations
}

func (m Manager) expectedRemote(ctx context.Context, path string, repository github.Repository) bool {
	result, err := m.Runner.Run(ctx, process.Command{Name: "git", Args: []string{"-C", path, "remote", "get-url", "--", m.Config.Remote}})
	if err != nil {
		return false
	}
	actual := normalizeURL(result.Stdout)
	return actual == normalizeURL(repository.SSHURL) || actual == normalizeURL(repository.CloneURL)
}

func normalizeURL(value string) string {
	return strings.TrimSuffix(strings.TrimSpace(value), "/")
}

// Inspect reports status for selected direct-child repositories.
func (m Manager) Inspect(ctx context.Context, requested []string) []Status {
	targets := m.targets(ctx, requested)
	statuses := make([]Status, 0, len(targets))
	for _, repository := range targets {
		if repository.err != nil {
			statuses = append(statuses, Status{Name: repository.name, Error: repository.err.Error()})
			continue
		}
		result, err := m.Runner.Run(ctx, process.Command{Name: "git", Args: []string{"-C", repository.path, "status", "--porcelain=v2", "--branch"}})
		if err != nil {
			statuses = append(statuses, Status{Name: repository.name, Error: err.Error()})
			continue
		}
		status, err := ParseStatus(repository.name, result.Stdout)
		if err != nil {
			status.Error = err.Error()
		}
		statuses = append(statuses, status)
	}
	return statuses
}

// ParseStatus parses Git porcelain-v2 branch and worktree output.
func ParseStatus(name, output string) (Status, error) {
	status := Status{Name: name, Clean: true}
	scanner := bufio.NewScanner(strings.NewReader(output))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "# branch.head "):
			status.Branch = strings.TrimPrefix(line, "# branch.head ")
			if status.Branch == "(detached)" {
				status.Branch = "HEAD"
				status.Detached = true
			}
		case strings.HasPrefix(line, "# branch.upstream "):
			status.Upstream = strings.TrimPrefix(line, "# branch.upstream ")
		case strings.HasPrefix(line, "# branch.ab "):
			fields := strings.Fields(strings.TrimPrefix(line, "# branch.ab "))
			if len(fields) != 2 {
				return status, fmt.Errorf("parse ahead/behind record %q", line)
			}
			var err error
			status.Ahead, err = strconv.Atoi(strings.TrimPrefix(fields[0], "+"))
			if err != nil {
				return status, fmt.Errorf("parse ahead count: %w", err)
			}
			status.Behind, err = strconv.Atoi(strings.TrimPrefix(fields[1], "-"))
			if err != nil {
				return status, fmt.Errorf("parse behind count: %w", err)
			}
		case line != "" && !strings.HasPrefix(line, "# "):
			status.Clean = false
			status.Changes++
		}
	}
	if err := scanner.Err(); err != nil {
		return status, fmt.Errorf("scan Git status: %w", err)
	}
	return status, nil
}

// Sync conservatively fast-forwards selected clean repositories.
func (m Manager) Sync(ctx context.Context, requested []string, dryRun bool) []Operation {
	targets := m.targets(ctx, requested)
	operations := make([]Operation, 0, len(targets))
	for _, repository := range targets {
		if repository.err != nil {
			operations = append(operations, Operation{Name: repository.name, Outcome: "failed", Detail: repository.err.Error()})
			continue
		}
		operations = append(operations, m.syncOne(ctx, repository, dryRun))
	}
	return operations
}

func (m Manager) syncOne(ctx context.Context, repository target, dryRun bool) Operation {
	operation := Operation{Name: repository.name}
	dirty, err := m.run(ctx, repository.path, "status", "--porcelain")
	if err != nil {
		operation.Outcome, operation.Detail = "failed", err.Error()
		return operation
	}
	if strings.TrimSpace(dirty) != "" {
		operation.Outcome, operation.Detail = "skipped", "working tree is dirty"
		return operation
	}
	if _, err := m.run(ctx, repository.path, "remote", "get-url", "--", m.Config.Remote); err != nil {
		operation.Outcome, operation.Detail = "skipped", "configured remote is not available: "+m.Config.Remote
		return operation
	}
	branch, err := m.run(ctx, repository.path, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		operation.Outcome, operation.Detail = "skipped", "detached HEAD cannot be synchronized safely"
		return operation
	}
	upstream, err := m.run(ctx, repository.path, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err != nil {
		operation.Outcome, operation.Detail = "skipped", "branch "+strings.TrimSpace(branch)+" has no upstream"
		return operation
	}
	upstream = strings.TrimSpace(upstream)
	if !strings.HasPrefix(upstream, m.Config.Remote+"/") {
		operation.Outcome, operation.Detail = "skipped", "upstream does not use configured remote: "+upstream
		return operation
	}
	fetch := process.Command{Name: "git", Args: []string{"-C", repository.path, "fetch", "--", m.Config.Remote}, Stream: true}
	merge := process.Command{Name: "git", Args: []string{"-C", repository.path, "merge", "--ff-only", upstream}, Stream: true}
	if dryRun {
		operation.Outcome = "planned"
		operation.Plan = []string{process.Describe(fetch), process.Describe(merge)}
		return operation
	}
	before, err := m.run(ctx, repository.path, "rev-parse", "HEAD")
	if err != nil {
		operation.Outcome, operation.Detail = "failed", err.Error()
		return operation
	}
	if _, err := m.Runner.Run(ctx, fetch); err != nil {
		operation.Outcome, operation.Detail = "failed", "fetch failed: "+err.Error()
		return operation
	}
	if _, err := m.Runner.Run(ctx, merge); err != nil {
		operation.Outcome, operation.Detail = "skipped", "fast-forward refused: "+err.Error()
		return operation
	}
	after, err := m.run(ctx, repository.path, "rev-parse", "HEAD")
	if err != nil {
		operation.Outcome, operation.Detail = "failed", err.Error()
		return operation
	}
	if strings.TrimSpace(before) == strings.TrimSpace(after) {
		operation.Outcome = "unchanged"
	} else {
		operation.Outcome = "updated"
	}
	return operation
}

func (m Manager) run(ctx context.Context, path string, arguments ...string) (string, error) {
	args := append([]string{"-C", path}, arguments...)
	result, err := m.Runner.Run(ctx, process.Command{Name: "git", Args: args})
	return result.Stdout, err
}

func (m Manager) targets(ctx context.Context, requested []string) []target {
	var names []string
	if len(requested) == 0 {
		discovered, err := workspace.Discover(ctx, m.Runner, m.Config)
		if err != nil {
			return []target{{name: "workspace", err: err}}
		}
		names = discovered
	} else {
		names = append(names, requested...)
	}
	targets := make([]target, 0, len(names))
	for _, name := range names {
		if err := config.ValidateRepositoryName(name, m.Config); err != nil {
			targets = append(targets, target{name: name, err: err})
			continue
		}
		path, err := workspace.DirectChild(m.Config.Workspace, name)
		if err == nil && !workspace.IsGitWorktree(ctx, m.Runner, path) {
			err = fmt.Errorf("local Git working tree not found: %s", name)
		}
		targets = append(targets, target{name: name, path: path, err: err})
	}
	return targets
}

// CountOutcomes counts operation outcomes for summaries.
func CountOutcomes(operations []Operation) map[string]int {
	counts := make(map[string]int)
	for _, operation := range operations {
		counts[operation.Outcome]++
	}
	return counts
}

// Failed reports whether an operation set was incomplete.
func Failed(operations []Operation) bool {
	for _, operation := range operations {
		if operation.Outcome == "failed" || operation.Outcome == "skipped" {
			return true
		}
	}
	return false
}

// StatusFailed reports whether any status could not be inspected.
func StatusFailed(statuses []Status) bool {
	for _, status := range statuses {
		if status.Error != "" {
			return true
		}
	}
	return false
}
