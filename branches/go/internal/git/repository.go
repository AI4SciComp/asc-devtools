// Package git implements safe local Git repository operations.
package git

import (
	"bufio"
	"context"
	"errors"
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
	Branch   string `json:"branch"`
	Detached bool   `json:"detached"`
	Upstream string `json:"upstream"`
	Ahead    int    `json:"ahead"`
	Behind   int    `json:"behind"`
	Clean    bool   `json:"clean"`
	Changes  int    `json:"changes"`
	Error    string `json:"error,omitempty"`
}

// SavePlan is an immutable, reviewable plan for committing and pushing one repository.
type SavePlan struct {
	Operation Operation `json:"operation"`
	Branch    string    `json:"branch"`
	Upstream  string    `json:"upstream"`
	Changes   int       `json:"changes"`
	Message   string    `json:"message"`
	path      string
	status    string
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
		operation.Outcome, operation.Detail = "unchanged", "already matches upstream"
	} else {
		operation.Outcome, operation.Detail = "updated", "downloaded a remote fast-forward"
	}
	return operation
}

// PlanSave prepares a local-only commit-and-push plan for exactly one repository.
func (m Manager) PlanSave(ctx context.Context, name, message string) (SavePlan, error) {
	message = strings.TrimSpace(message)
	if message == "" || len(message) > 500 || strings.ContainsAny(message, "\x00\r\n") {
		return SavePlan{}, errors.New("commit message must be 1-500 characters on one line")
	}
	targets := m.targets(ctx, []string{name})
	if len(targets) != 1 || targets[0].err != nil {
		if len(targets) == 1 {
			return SavePlan{}, targets[0].err
		}
		return SavePlan{}, errors.New("save requires one repository")
	}
	repository := targets[0]
	statusOutput, err := m.run(ctx, repository.path, "status", "--porcelain=v2", "--branch")
	if err != nil {
		return SavePlan{}, err
	}
	status, err := ParseStatus(name, statusOutput)
	if err != nil {
		return SavePlan{}, err
	}
	if status.Detached || status.Branch == "" {
		return SavePlan{}, errors.New("detached HEAD cannot be saved safely")
	}
	if status.Upstream == "" {
		return SavePlan{}, fmt.Errorf("branch %s has no upstream", status.Branch)
	}
	remotePrefix := m.Config.Remote + "/"
	if !strings.HasPrefix(status.Upstream, remotePrefix) || strings.TrimPrefix(status.Upstream, remotePrefix) == "" {
		return SavePlan{}, fmt.Errorf("upstream does not use configured remote: %s", status.Upstream)
	}
	for _, line := range strings.Split(statusOutput, "\n") {
		if strings.HasPrefix(line, "u ") {
			return SavePlan{}, errors.New("repository has unresolved merge conflicts")
		}
	}
	fetch := process.Command{Name: "git", Args: []string{"-C", repository.path, "fetch", "--", m.Config.Remote}}
	add := process.Command{Name: "git", Args: []string{"-C", repository.path, "add", "--all", "--"}}
	commit := process.Command{Name: "git", Args: []string{"-C", repository.path, "commit", "-m", message}}
	remoteBranch := strings.TrimPrefix(status.Upstream, remotePrefix)
	push := process.Command{Name: "git", Args: []string{"-C", repository.path, "push", "--", m.Config.Remote, "HEAD:refs/heads/" + remoteBranch}}
	commands := []string{process.Describe(fetch)}
	if !status.Clean {
		commands = append(commands, process.Describe(add), process.Describe(commit))
	}
	commands = append(commands, process.Describe(push))
	detail := fmt.Sprintf("%d working-tree changes on %s tracking %s", status.Changes, status.Branch, status.Upstream)
	return SavePlan{
		Operation: Operation{Name: name, Outcome: "planned", Detail: detail, Plan: commands},
		Branch:    status.Branch, Upstream: status.Upstream, Changes: status.Changes, Message: message,
		path: repository.path, status: statusOutput,
	}, nil
}

// ApplySave verifies and executes a previously reviewed save plan.
func (m Manager) ApplySave(ctx context.Context, plan SavePlan) Operation {
	operation := Operation{Name: plan.Operation.Name}
	currentStatus, err := m.run(ctx, plan.path, "status", "--porcelain=v2", "--branch")
	if err != nil {
		operation.Outcome, operation.Detail = "failed", err.Error()
		return operation
	}
	if currentStatus != plan.status {
		operation.Outcome, operation.Detail = "failed", "repository changed after the save plan was reviewed"
		return operation
	}
	if _, err := m.Runner.Run(ctx, process.Command{Name: "git", Args: []string{"-C", plan.path, "fetch", "--", m.Config.Remote}, Stream: true}); err != nil {
		operation.Outcome, operation.Detail = "failed", "fetch failed: "+err.Error()
		return operation
	}
	counts, err := m.run(ctx, plan.path, "rev-list", "--left-right", "--count", "HEAD..."+plan.Upstream)
	if err != nil {
		operation.Outcome, operation.Detail = "failed", "compare upstream: "+err.Error()
		return operation
	}
	ahead, behind, err := parseAheadBehind(counts)
	if err != nil {
		operation.Outcome, operation.Detail = "failed", err.Error()
		return operation
	}
	if behind > 0 {
		operation.Outcome = "skipped"
		if ahead > 0 {
			operation.Detail = "local and remote histories diverged; resolve manually before saving"
		} else if plan.Changes > 0 {
			operation.Detail = "remote has new commits while local changes exist; preserve and reconcile them manually before saving"
		} else {
			operation.Detail = "remote has new commits; run asc repo sync first"
		}
		return operation
	}
	committed := false
	if plan.Changes > 0 {
		if _, err := m.Runner.Run(ctx, process.Command{Name: "git", Args: []string{"-C", plan.path, "add", "--all", "--"}, Stream: true}); err != nil {
			operation.Outcome, operation.Detail = "failed", "stage failed: "+err.Error()
			return operation
		}
		_, diffErr := m.Runner.Run(ctx, process.Command{Name: "git", Args: []string{"-C", plan.path, "diff", "--cached", "--quiet", "--exit-code"}})
		if diffErr != nil && process.ExitCode(diffErr, -1) != 1 {
			operation.Outcome, operation.Detail = "failed", "inspect staged changes: "+diffErr.Error()
			return operation
		}
		if process.ExitCode(diffErr, 0) == 1 {
			if _, err := m.Runner.Run(ctx, process.Command{Name: "git", Args: []string{"-C", plan.path, "commit", "-m", plan.Message}, Stream: true}); err != nil {
				operation.Outcome, operation.Detail = "failed", "commit failed: "+err.Error()
				return operation
			}
			committed = true
		}
	}
	if ahead == 0 && !committed {
		operation.Outcome, operation.Detail = "unchanged", "nothing to commit or push"
		return operation
	}
	remoteBranch := strings.TrimPrefix(plan.Upstream, m.Config.Remote+"/")
	if _, err := m.Runner.Run(ctx, process.Command{Name: "git", Args: []string{"-C", plan.path, "push", "--", m.Config.Remote, "HEAD:refs/heads/" + remoteBranch}, Stream: true}); err != nil {
		operation.Outcome, operation.Detail = "failed", "push failed; local commit was preserved: "+err.Error()
		return operation
	}
	operation.Outcome = "saved"
	if committed {
		operation.Detail = "committed and pushed to " + plan.Upstream
	} else {
		operation.Detail = "pushed existing local commits to " + plan.Upstream
	}
	return operation
}

func parseAheadBehind(output string) (int, int, error) {
	fields := strings.Fields(output)
	if len(fields) != 2 {
		return 0, 0, fmt.Errorf("parse upstream comparison %q", strings.TrimSpace(output))
	}
	ahead, err := strconv.Atoi(fields[0])
	if err != nil || ahead < 0 {
		return 0, 0, fmt.Errorf("parse local ahead count %q", fields[0])
	}
	behind, err := strconv.Atoi(fields[1])
	if err != nil || behind < 0 {
		return 0, 0, fmt.Errorf("parse remote ahead count %q", fields[1])
	}
	return ahead, behind, nil
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
