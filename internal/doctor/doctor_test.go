package doctor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AI4SciComp/asc-devtools/internal/config"
	"github.com/AI4SciComp/asc-devtools/internal/github"
	"github.com/AI4SciComp/asc-devtools/internal/process"
)

type doctorRunner struct{}

func (doctorRunner) Run(_ context.Context, command process.Command) (process.Result, error) {
	if command.Name == "ssh" {
		return process.Result{Stderr: "successfully authenticated"}, errors.New("GitHub provides no shell")
	}
	return process.Result{Stdout: command.Name + " version\n"}, nil
}

type doctorLister struct {
	err error
}

func (l doctorLister) ListOrganizationRepositories(context.Context, string) ([]github.Repository, error) {
	return []github.Repository{{Name: "asc-one"}}, l.err
}

func TestRunReportsBoundariesAndFailures(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace")
	if err := os.Mkdir(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	service := Service{
		Runner: doctorRunner{},
		GitHub: doctorLister{},
		Config: config.Config{
			ConfigPath:        "/config.json",
			Workspace:         workspace,
			Organization:      "AI4SciComp",
			GitHubToken:       "not-printed",
			GitHubTokenSource: "ASC_GITHUB_TOKEN",
		},
		LookupPath: func(string) (string, error) { return "/bin/tool", nil },
	}
	checks := service.Run(context.Background())
	joined := ""
	for _, check := range checks {
		joined += check.Name + ":" + check.Status + ":" + check.Detail + "\n"
	}
	if Failed(checks) || !strings.Contains(joined, "api-authentication:pass") || !strings.Contains(joined, "ssh:pass") || strings.Contains(joined, "not-printed") {
		t.Fatalf("checks = %s", joined)
	}
	service.GitHub = doctorLister{err: errors.New("offline")}
	if !Failed(service.Run(context.Background())) {
		t.Fatal("Failed() = false for GitHub API failure")
	}
}
