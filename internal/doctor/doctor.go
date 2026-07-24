// Package doctor performs read-only environment diagnostics.
package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AI4SciComp/asc-devtools/internal/config"
	"github.com/AI4SciComp/asc-devtools/internal/github"
	"github.com/AI4SciComp/asc-devtools/internal/process"
)

// RepositoryLister is the GitHub operation used by diagnostics.
type RepositoryLister interface {
	ListOrganizationRepositories(context.Context, string) ([]github.Repository, error)
}

// Check is one structured diagnostic result.
type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	Remedy string `json:"remedy,omitempty"`
}

// Service owns diagnostic dependencies.
type Service struct {
	Runner     process.Runner
	GitHub     RepositoryLister
	Config     config.Config
	LookupPath func(string) (string, error)
}

// Run performs diagnostics without changing user state.
func (s Service) Run(ctx context.Context) []Check {
	lookup := s.LookupPath
	if lookup == nil {
		lookup = exec.LookPath
	}
	checks := []Check{
		{Name: "configuration", Status: "pass", Detail: s.Config.ConfigPath},
		workspaceCheck(s.Config.Workspace),
		toolCheck(ctx, s.Runner, "git", true),
		toolCheck(ctx, s.Runner, "cmake", false),
		toolCheck(ctx, s.Runner, "ctest", false),
	}
	if s.Config.GitHubToken != "" {
		checks = append(checks, Check{Name: "api-authentication", Status: "pass", Detail: "GitHub API token is available from " + s.Config.GitHubTokenSource})
	} else {
		checks = append(checks, Check{Name: "api-authentication", Status: "warning", Detail: "using unauthenticated public GitHub API access", Remedy: "set ASC_GITHUB_TOKEN for private repositories and higher rate limits"})
	}
	if s.GitHub == nil {
		checks = append(checks, Check{Name: "github-api", Status: "failure", Detail: "GitHub API client is unavailable"})
	} else if repositories, err := s.GitHub.ListOrganizationRepositories(ctx, s.Config.Organization); err != nil {
		checks = append(checks, Check{Name: "github-api", Status: "failure", Detail: err.Error(), Remedy: "check connectivity, organization, token permissions, and rate limits"})
	} else {
		checks = append(checks, Check{Name: "github-api", Status: "pass", Detail: fmt.Sprintf("can list %d repositories in %s", len(repositories), s.Config.Organization)})
	}
	checks = append(checks, sshCheck(ctx, s.Runner, lookup))
	if _, err := lookup("asc"); err == nil {
		checks = append(checks, Check{Name: "path", Status: "pass", Detail: "asc is available on PATH"})
	} else {
		checks = append(checks, Check{Name: "path", Status: "warning", Detail: "asc is not available on PATH", Remedy: "install asc under /usr/local/bin"})
	}
	return checks
}

func workspaceCheck(path string) Check {
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return Check{Name: "workspace", Status: "pass", Detail: path + " exists"}
		}
		return Check{Name: "workspace", Status: "failure", Detail: path + " is not a directory"}
	}
	if !os.IsNotExist(err) {
		return Check{Name: "workspace", Status: "failure", Detail: err.Error()}
	}
	parent := path
	for {
		parent = filepath.Dir(parent)
		info, err = os.Stat(parent)
		if err == nil || parent == filepath.Dir(parent) {
			break
		}
	}
	if err == nil && info.IsDir() && info.Mode().Perm()&0o200 != 0 {
		return Check{Name: "workspace", Status: "pass", Detail: path + " can be created"}
	}
	return Check{Name: "workspace", Status: "failure", Detail: path + " cannot be created"}
}

func toolCheck(ctx context.Context, runner process.Runner, name string, required bool) Check {
	result, err := runner.Run(ctx, process.Command{Name: name, Args: []string{"--version"}})
	if err != nil {
		status := "warning"
		if required {
			status = "failure"
		}
		return Check{Name: name, Status: status, Detail: err.Error(), Remedy: "install " + name}
	}
	detail := strings.SplitN(strings.TrimSpace(result.Stdout+result.Stderr), "\n", 2)[0]
	return Check{Name: name, Status: "pass", Detail: detail}
}

func sshCheck(ctx context.Context, runner process.Runner, lookup func(string) (string, error)) Check {
	if _, err := lookup("ssh"); err != nil {
		return Check{Name: "ssh", Status: "warning", Detail: "ssh is not installed", Remedy: "install OpenSSH for SSH clones"}
	}
	result, err := runner.Run(ctx, process.Command{Name: "ssh", Args: []string{"-T", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", "git@github.com"}})
	detail := strings.ToLower(result.Stdout + result.Stderr)
	if err != nil {
		detail += " " + strings.ToLower(err.Error())
	}
	if strings.Contains(detail, "successfully authenticated") {
		return Check{Name: "ssh", Status: "pass", Detail: "GitHub accepted the configured SSH key"}
	}
	return Check{Name: "ssh", Status: "warning", Detail: "GitHub SSH authentication was not confirmed", Remedy: "run ssh -T git@github.com"}
}

// Failed reports whether diagnostics contain a failure.
func Failed(checks []Check) bool {
	for _, check := range checks {
		if check.Status == "failure" {
			return true
		}
	}
	return false
}
