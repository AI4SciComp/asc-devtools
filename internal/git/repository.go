// Package git implements safe local Git repository operations.
package git

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// Operation describes one clone, synchronization, or save outcome.
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
	Operation    Operation `json:"operation"`
	Branch       string    `json:"branch"`
	RemoteBranch string    `json:"remoteBranch"`
	Upstream     string    `json:"upstream"`
	Changes      int       `json:"changes"`
	Message      string    `json:"message"`
	Force        bool      `json:"force"`
	path         string
	status       string
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
		if err := validateCloneURL(cloneURL, protocol, m.Config.Organization, name); err != nil {
			operations = append(operations, Operation{Name: name, Outcome: "failed", Detail: err.Error()})
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

func validateCloneURL(cloneURL, protocol, organization, repository string) error {
	if cloneURL == "" {
		return fmt.Errorf("%s clone URL is missing from the API response", protocol)
	}
	var expected string
	switch protocol {
	case "ssh":
		expected = fmt.Sprintf("git@github.com:%s/%s.git", organization, repository)
	case "https":
		expected = fmt.Sprintf("https://github.com/%s/%s.git", organization, repository)
	default:
		return fmt.Errorf("unsupported clone protocol: %s", protocol)
	}
	if cloneURL != expected {
		return fmt.Errorf("unsafe %s clone URL returned by the API for %s", protocol, repository)
	}
	return nil
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
			if len(fields) != 2 || !strings.HasPrefix(fields[0], "+") || !strings.HasPrefix(fields[1], "-") {
				return status, fmt.Errorf("parse ahead/behind record %q", line)
			}
			var err error
			status.Ahead, err = strconv.Atoi(strings.TrimPrefix(fields[0], "+"))
			if err != nil {
				return status, fmt.Errorf("parse ahead count: %w", err)
			}
			if status.Ahead < 0 {
				return status, fmt.Errorf("parse ahead count %q", fields[0])
			}
			status.Behind, err = strconv.Atoi(strings.TrimPrefix(fields[1], "-"))
			if err != nil {
				return status, fmt.Errorf("parse behind count: %w", err)
			}
			if status.Behind < 0 {
				return status, fmt.Errorf("parse behind count %q", fields[1])
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

// CurrentRepository returns the managed direct-child repository containing directory.
func (m Manager) CurrentRepository(ctx context.Context, directory string) (string, error) {
	if directory == "" {
		return "", errors.New("resolve current repository: working directory is empty")
	}
	result, err := m.Runner.Run(ctx, process.Command{
		Name: "git",
		Args: []string{"-C", directory, "rev-parse", "--show-toplevel"},
	})
	if err != nil {
		return "", fmt.Errorf("current directory is not in a Git working tree: %w", err)
	}
	root := filepath.Clean(strings.TrimSpace(result.Stdout))
	if root == "." || !filepath.IsAbs(root) {
		return "", fmt.Errorf("Git returned an invalid working-tree root: %q", strings.TrimSpace(result.Stdout))
	}
	relative, err := filepath.Rel(m.Config.Workspace, root)
	if err != nil {
		return "", fmt.Errorf("resolve current repository relative to workspace: %w", err)
	}
	if relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) ||
		strings.Contains(relative, string(filepath.Separator)) {
		return "", errors.New("current Git working tree is not a direct child of the asc workspace")
	}
	if err := config.ValidateRepositoryName(relative, m.Config); err != nil {
		return "", err
	}
	expected, err := workspace.DirectChild(m.Config.Workspace, relative)
	if err != nil {
		return "", err
	}
	if filepath.Clean(expected) != root || !workspace.IsGitWorktree(ctx, m.Runner, expected) {
		return "", errors.New("current Git working tree is not a safe managed repository")
	}
	return relative, nil
}

// PlanSave prepares a local-only commit-and-push plan for exactly one repository.
func (m Manager) PlanSave(ctx context.Context, name, message string, force bool) (SavePlan, error) {
	return m.PlanSaveBranch(ctx, name, "", message, force)
}

// PlanSaveBranch prepares a save plan with an optional explicit remote destination branch.
func (m Manager) PlanSaveBranch(ctx context.Context, name, branch, message string, force bool) (SavePlan, error) {
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
	remotePrefix := m.Config.Remote + "/"
	remoteBranch := branch
	upstream := status.Upstream
	if remoteBranch == "" {
		if upstream == "" {
			return SavePlan{}, fmt.Errorf("branch %s has no upstream; select a destination with --branch", status.Branch)
		}
		if !strings.HasPrefix(upstream, remotePrefix) || strings.TrimPrefix(upstream, remotePrefix) == "" {
			return SavePlan{}, fmt.Errorf("upstream does not use configured remote: %s", upstream)
		}
		remoteBranch = strings.TrimPrefix(upstream, remotePrefix)
	} else {
		if strings.TrimSpace(remoteBranch) != remoteBranch || strings.HasPrefix(remoteBranch, "-") {
			return SavePlan{}, fmt.Errorf("invalid branch name %q", remoteBranch)
		}
		if _, err := m.run(ctx, repository.path, "check-ref-format", "refs/heads/"+remoteBranch); err != nil {
			return SavePlan{}, fmt.Errorf("invalid branch name %q", remoteBranch)
		}
		upstream = remotePrefix + remoteBranch
	}
	for _, line := range strings.Split(statusOutput, "\n") {
		if strings.HasPrefix(line, "u ") {
			return SavePlan{}, errors.New("repository has unresolved merge conflicts")
		}
	}
	fetchRefspec := "+refs/heads/*:refs/remotes/" + m.Config.Remote + "/*"
	fetch := process.Command{Name: "git", Args: []string{"-C", repository.path, "fetch", "--prune", "--", m.Config.Remote, fetchRefspec}}
	add := process.Command{Name: "git", Args: []string{"-C", repository.path, "add", "--all", "--"}}
	commit := process.Command{Name: "git", Args: []string{"-C", repository.path, "commit", "-m", message}}
	pushArguments := []string{"-C", repository.path, "push"}
	if force {
		pushArguments = append(pushArguments, "--force")
	}
	pushArguments = append(pushArguments, "--", m.Config.Remote, "HEAD:refs/heads/"+remoteBranch)
	push := process.Command{Name: "git", Args: pushArguments}
	commands := []string{process.Describe(fetch)}
	if !status.Clean {
		commands = append(commands, process.Describe(add), process.Describe(commit))
	}
	commands = append(commands, process.Describe(push))
	detail := fmt.Sprintf("%d working-tree changes on %s saving to %s", status.Changes, status.Branch, upstream)
	if force {
		detail += " (remote history will be overwritten)"
	}
	return SavePlan{
		Operation:    Operation{Name: name, Outcome: "planned", Detail: detail, Plan: commands},
		Branch:       status.Branch,
		RemoteBranch: remoteBranch,
		Upstream:     upstream,
		Changes:      status.Changes,
		Message:      message,
		Force:        force,
		path:         repository.path,
		status:       statusOutput,
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
	if _, err := m.Runner.Run(ctx, process.Command{
		Name: "git", Args: []string{
			"-C", plan.path, "fetch", "--prune", "--", m.Config.Remote,
			"+refs/heads/*:refs/remotes/" + m.Config.Remote + "/*",
		}, Stream: true,
	}); err != nil {
		operation.Outcome, operation.Detail = "failed", "fetch failed: "+err.Error()
		return operation
	}
	remoteRef := "refs/remotes/" + m.Config.Remote + "/" + plan.RemoteBranch
	_, remoteRefError := m.Runner.Run(ctx, process.Command{
		Name: "git", Args: []string{"-C", plan.path, "show-ref", "--verify", "--quiet", remoteRef},
	})
	remoteExists := remoteRefError == nil
	if remoteRefError != nil && process.ExitCode(remoteRefError, -1) != 1 {
		operation.Outcome, operation.Detail = "failed", "inspect destination branch: "+remoteRefError.Error()
		return operation
	}
	ahead, behind := 0, 0
	if remoteExists {
		counts, err := m.run(ctx, plan.path, "rev-list", "--left-right", "--count", "HEAD..."+remoteRef)
		if err != nil {
			operation.Outcome, operation.Detail = "failed", "compare destination branch: "+err.Error()
			return operation
		}
		ahead, behind, err = parseAheadBehind(counts)
		if err != nil {
			operation.Outcome, operation.Detail = "failed", err.Error()
			return operation
		}
	}
	if behind > 0 && !plan.Force {
		operation.Outcome = "skipped"
		if ahead > 0 {
			operation.Detail = "local and remote histories diverged; use --force to overwrite the remote or resolve manually"
		} else if plan.Changes > 0 {
			operation.Detail = "remote has new commits while local changes exist; use --force to overwrite the remote or reconcile manually"
		} else {
			operation.Detail = "remote has new commits; run asc repo sync first or use --force to overwrite the remote"
		}
		return operation
	}
	committed := false
	if plan.Changes > 0 {
		if _, err := m.Runner.Run(ctx, process.Command{
			Name: "git", Args: []string{"-C", plan.path, "add", "--all", "--"}, Stream: true,
		}); err != nil {
			operation.Outcome, operation.Detail = "failed", "stage failed: "+err.Error()
			return operation
		}
		_, diffErr := m.Runner.Run(ctx, process.Command{
			Name: "git", Args: []string{"-C", plan.path, "diff", "--cached", "--quiet", "--exit-code"},
		})
		if diffErr != nil && process.ExitCode(diffErr, -1) != 1 {
			operation.Outcome, operation.Detail = "failed", "inspect staged changes: "+diffErr.Error()
			return operation
		}
		if process.ExitCode(diffErr, 0) == 1 {
			if _, err := m.Runner.Run(ctx, process.Command{
				Name: "git", Args: []string{"-C", plan.path, "commit", "-m", plan.Message}, Stream: true,
			}); err != nil {
				operation.Outcome, operation.Detail = "failed", "commit failed: "+err.Error()
				return operation
			}
			committed = true
		}
	}
	if remoteExists && ahead == 0 && behind == 0 && !committed {
		operation.Outcome, operation.Detail = "unchanged", "nothing to commit or push"
		return operation
	}
	pushArguments := []string{"-C", plan.path, "push"}
	if plan.Force {
		pushArguments = append(pushArguments, "--force")
	}
	pushArguments = append(pushArguments, "--", m.Config.Remote, "HEAD:refs/heads/"+plan.RemoteBranch)
	if _, err := m.Runner.Run(ctx, process.Command{
		Name: "git", Args: pushArguments, Stream: true,
	}); err != nil {
		operation.Outcome, operation.Detail = "failed", "push failed; local commit was preserved: "+err.Error()
		return operation
	}
	operation.Outcome = "saved"
	switch {
	case plan.Force && committed:
		operation.Detail = "committed and force-pushed to " + plan.Upstream
	case plan.Force:
		operation.Detail = "force-pushed local history to " + plan.Upstream
	case committed:
		operation.Detail = "committed and pushed to " + plan.Upstream
	default:
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
