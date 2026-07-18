// Package app parses commands and coordinates asc services.
package app

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/AI4SciComp/asc-devtools/internal/cmake"
	"github.com/AI4SciComp/asc-devtools/internal/completion"
	"github.com/AI4SciComp/asc-devtools/internal/config"
	"github.com/AI4SciComp/asc-devtools/internal/doctor"
	gitrepo "github.com/AI4SciComp/asc-devtools/internal/git"
	"github.com/AI4SciComp/asc-devtools/internal/github"
	"github.com/AI4SciComp/asc-devtools/internal/process"
)

const (
	// ExitSuccess indicates a completed command.
	ExitSuccess = 0
	// ExitFailure indicates an operational or partial failure.
	ExitFailure = 1
	// ExitUsage indicates an invalid invocation.
	ExitUsage = 2
)

// VersionInfo describes one asc build.
type VersionInfo struct {
	Version   string
	Commit    string
	BuildDate string
}

// String returns deterministic version output.
func (v VersionInfo) String() string {
	version := v.Version
	if version == "" {
		version = "dev"
	}
	parts := []string{"asc", version}
	if v.Commit != "" {
		parts = append(parts, "commit="+v.Commit)
	}
	if v.BuildDate != "" {
		parts = append(parts, "built="+v.BuildDate)
	}
	return strings.Join(parts, " ")
}

// Dependencies contains application boundary implementations.
type Dependencies struct {
	Stdout      io.Writer
	Stderr      io.Writer
	Stdin       io.Reader
	Config      config.Loader
	Runner      process.Runner
	HTTPClient  *http.Client
	APIBaseURL  string
	LookupPath  func(string) (string, error)
	VersionInfo VersionInfo
}

type globalOptions struct {
	configPath   optionalString
	organization optionalString
	workspace    optionalString
	noColor      bool
	help         bool
	version      bool
}

type optionalString struct {
	value string
	set   bool
}

func (o *optionalString) String() string { return o.value }
func (o *optionalString) Set(value string) error {
	o.value, o.set = value, true
	return nil
}

// Run executes asc and returns a documented exit code.
func Run(ctx context.Context, arguments []string, dependencies Dependencies) int {
	dependencies = defaults(dependencies)
	options, remaining, err := parseGlobal(arguments)
	if err != nil {
		return usageError(dependencies.Stderr, err.Error(), globalUsage)
	}
	if options.help {
		fmt.Fprint(dependencies.Stdout, globalUsage)
		return ExitSuccess
	}
	if options.version {
		fmt.Fprintln(dependencies.Stdout, dependencies.VersionInfo.String())
		return ExitSuccess
	}
	if len(remaining) == 0 {
		return usageError(dependencies.Stderr, "a command is required", globalUsage)
	}
	command, commandArguments := remaining[0], remaining[1:]
	if helpRequested(commandArguments) {
		if usage, ok := commandUsage(command, commandArguments); ok {
			fmt.Fprint(dependencies.Stdout, usage)
			return ExitSuccess
		}
	}

	overrides := config.Overrides{NoColor: options.noColor}
	if options.configPath.set {
		overrides.ConfigPath = &options.configPath.value
	}
	if options.organization.set {
		overrides.Organization = &options.organization.value
	}
	if options.workspace.set {
		overrides.Workspace = &options.workspace.value
	}
	cfg, err := dependencies.Config.Load(overrides)
	if err != nil {
		return operationalError(dependencies.Stderr, err)
	}

	switch command {
	case "workspace":
		if len(commandArguments) != 0 {
			return usageError(dependencies.Stderr, "workspace takes no arguments", workspaceUsage)
		}
		fmt.Fprintln(dependencies.Stdout, cfg.Workspace)
		return ExitSuccess
	case "repo":
		return runRepo(ctx, cfg, commandArguments, dependencies)
	case "configure", "build", "test":
		return runCMake(ctx, cfg, command, commandArguments, dependencies)
	case "doctor":
		return runDoctor(ctx, cfg, commandArguments, dependencies)
	case "completion":
		if len(commandArguments) != 1 || commandArguments[0] != "bash" {
			return usageError(dependencies.Stderr, "only Bash completion is supported", completionUsage)
		}
		fmt.Fprint(dependencies.Stdout, completion.Bash)
		return ExitSuccess
	default:
		return usageError(dependencies.Stderr, "unknown command: "+command, globalUsage)
	}
}

func defaults(dependencies Dependencies) Dependencies {
	if dependencies.Stdout == nil {
		dependencies.Stdout = os.Stdout
	}
	if dependencies.Stderr == nil {
		dependencies.Stderr = os.Stderr
	}
	if dependencies.Stdin == nil {
		dependencies.Stdin = os.Stdin
	}
	if dependencies.Config.LookupEnv == nil {
		dependencies.Config = config.NewLoader()
	}
	if dependencies.Runner == nil {
		dependencies.Runner = process.OSRunner{Stdin: dependencies.Stdin, Stdout: dependencies.Stdout, Stderr: dependencies.Stderr}
	}
	if dependencies.LookupPath == nil {
		dependencies.LookupPath = exec.LookPath
	}
	return dependencies
}

func parseGlobal(arguments []string) (globalOptions, []string, error) {
	var options globalOptions
	flags := flag.NewFlagSet("asc", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Var(&options.configPath, "config", "configuration path")
	flags.Var(&options.organization, "organization", "GitHub organization")
	flags.Var(&options.workspace, "workspace", "workspace path")
	flags.BoolVar(&options.noColor, "no-color", false, "disable color")
	flags.BoolVar(&options.help, "help", false, "show help")
	flags.BoolVar(&options.help, "h", false, "show help")
	flags.BoolVar(&options.version, "version", false, "show version")
	if err := flags.Parse(arguments); err != nil {
		return globalOptions{}, nil, err
	}
	return options, flags.Args(), nil
}

func runRepo(ctx context.Context, cfg config.Config, arguments []string, dependencies Dependencies) int {
	if len(arguments) == 0 {
		return usageError(dependencies.Stderr, "a repo command is required", repoUsage)
	}
	subcommand, commandArguments := arguments[0], arguments[1:]
	if helpRequested(commandArguments) {
		var usage string
		switch subcommand {
		case "list":
			usage = repoListUsage
		case "clone":
			usage = repoCloneUsage
		case "status":
			usage = repoStatusUsage
		case "sync":
			usage = repoSyncUsage
		default:
			return usageError(dependencies.Stderr, "unknown repo command: "+subcommand, repoUsage)
		}
		fmt.Fprint(dependencies.Stdout, usage)
		return ExitSuccess
	}
	switch subcommand {
	case "list":
		jsonOutput, names, err := parseSwitches(commandArguments, map[string]*bool{"--json": new(bool)})
		if err != nil || len(names) != 0 {
			if err == nil {
				err = fmt.Errorf("repo list takes no repository names")
			}
			return usageError(dependencies.Stderr, err.Error(), repoListUsage)
		}
		client, cfg := githubClient(ctx, cfg, dependencies)
		repositories, err := client.ListOrganizationRepositories(ctx, cfg.Organization)
		if err != nil {
			return operationalError(dependencies.Stderr, err)
		}
		repositories = github.FilterManaged(repositories, cfg.RepositoryPrefix, cfg.IncludeDotGitHub)
		if jsonOutput["--json"] {
			return writeJSON(dependencies.Stdout, dependencies.Stderr, repositories)
		}
		fmt.Fprintln(dependencies.Stdout, "REPOSITORY\tVISIBILITY\tDEFAULT BRANCH")
		for _, repository := range repositories {
			visibility := "public"
			if repository.Private {
				visibility = "private"
			}
			fmt.Fprintf(dependencies.Stdout, "%s\t%s\t%s\n", repository.Name, visibility, repository.DefaultBranch)
		}
		return ExitSuccess
	case "clone":
		protocol, names, err := parseClone(commandArguments, cfg.CloneProtocol)
		if err != nil {
			return usageError(dependencies.Stderr, err.Error(), repoCloneUsage)
		}
		client, cfg := githubClient(ctx, cfg, dependencies)
		repositories, err := client.ListOrganizationRepositories(ctx, cfg.Organization)
		if err != nil {
			return operationalError(dependencies.Stderr, err)
		}
		repositories = github.FilterManaged(repositories, cfg.RepositoryPrefix, cfg.IncludeDotGitHub)
		operations := (gitrepo.Manager{Runner: dependencies.Runner, Config: cfg}).Clone(ctx, repositories, names, protocol)
		writeOperations(dependencies.Stdout, operations)
		return operationsExit(operations)
	case "status":
		switches, names, err := parseSwitches(commandArguments, map[string]*bool{"--json": new(bool)})
		if err != nil {
			return usageError(dependencies.Stderr, err.Error(), repoStatusUsage)
		}
		statuses := (gitrepo.Manager{Runner: dependencies.Runner, Config: cfg}).Inspect(ctx, names)
		if switches["--json"] {
			if code := writeJSON(dependencies.Stdout, dependencies.Stderr, statuses); code != ExitSuccess {
				return code
			}
		} else {
			writeStatuses(dependencies.Stdout, statuses)
		}
		if gitrepo.StatusFailed(statuses) {
			return ExitFailure
		}
		return ExitSuccess
	case "sync":
		switches, names, err := parseSwitches(commandArguments, map[string]*bool{"--dry-run": new(bool)})
		if err != nil {
			return usageError(dependencies.Stderr, err.Error(), repoSyncUsage)
		}
		operations := (gitrepo.Manager{Runner: dependencies.Runner, Config: cfg}).Sync(ctx, names, switches["--dry-run"])
		writeOperations(dependencies.Stdout, operations)
		return operationsExit(operations)
	default:
		return usageError(dependencies.Stderr, "unknown repo command: "+subcommand, repoUsage)
	}
}

func runCMake(ctx context.Context, cfg config.Config, operation string, arguments []string, dependencies Dependencies) int {
	var repository, preset string
	for index := 0; index < len(arguments); index++ {
		switch arguments[index] {
		case "--preset":
			if index+1 >= len(arguments) {
				return usageError(dependencies.Stderr, "--preset requires a value", cmakeUsage(operation))
			}
			preset = arguments[index+1]
			index++
		default:
			if strings.HasPrefix(arguments[index], "-") {
				return usageError(dependencies.Stderr, "unknown option: "+arguments[index], cmakeUsage(operation))
			}
			if repository != "" {
				return usageError(dependencies.Stderr, operation+" takes one repository", cmakeUsage(operation))
			}
			repository = arguments[index]
		}
	}
	if repository == "" || preset == "" {
		return usageError(dependencies.Stderr, "repository and --preset are required", cmakeUsage(operation))
	}
	err := (cmake.Manager{Runner: dependencies.Runner, Config: cfg}).Run(ctx, operation, repository, preset)
	if err != nil {
		fmt.Fprintf(dependencies.Stderr, "asc: error: %v\n", err)
		code := process.ExitCode(err, ExitFailure)
		if code < 1 || code > 125 {
			return ExitFailure
		}
		return code
	}
	return ExitSuccess
}

func runDoctor(ctx context.Context, cfg config.Config, arguments []string, dependencies Dependencies) int {
	switches, names, err := parseSwitches(arguments, map[string]*bool{"--json": new(bool)})
	if err != nil || len(names) != 0 {
		if err == nil {
			err = fmt.Errorf("doctor takes no positional arguments")
		}
		return usageError(dependencies.Stderr, err.Error(), doctorUsage)
	}
	client, cfg := githubClient(ctx, cfg, dependencies)
	checks := (doctor.Service{Runner: dependencies.Runner, GitHub: client, Config: cfg, LookupPath: dependencies.LookupPath}).Run(ctx)
	if switches["--json"] {
		if code := writeJSON(dependencies.Stdout, dependencies.Stderr, checks); code != ExitSuccess {
			return code
		}
	} else {
		for _, check := range checks {
			fmt.Fprintf(dependencies.Stdout, "%-7s %-20s %s", strings.ToUpper(check.Status), check.Name, check.Detail)
			if check.Remedy != "" {
				fmt.Fprintf(dependencies.Stdout, "; %s", check.Remedy)
			}
			fmt.Fprintln(dependencies.Stdout)
		}
	}
	if doctor.Failed(checks) {
		return ExitFailure
	}
	return ExitSuccess
}

func githubClient(ctx context.Context, cfg config.Config, dependencies Dependencies) (*github.Client, config.Config) {
	if cfg.GitHubToken == "" {
		if _, err := dependencies.LookupPath("gh"); err == nil {
			result, err := dependencies.Runner.Run(ctx, process.Command{Name: "gh", Args: []string{"auth", "token"}})
			if err == nil && strings.TrimSpace(result.Stdout) != "" {
				cfg.GitHubToken = strings.TrimSpace(result.Stdout)
				cfg.GitHubTokenSource = "gh auth token"
			}
		}
	}
	client := github.NewClient(cfg.GitHubToken, dependencies.VersionInfo.Version)
	if dependencies.HTTPClient != nil {
		client.HTTPClient = dependencies.HTTPClient
	}
	if dependencies.APIBaseURL != "" {
		client.BaseURL = dependencies.APIBaseURL
	}
	return client, cfg
}

func parseClone(arguments []string, defaultProtocol string) (string, []string, error) {
	protocol := defaultProtocol
	var names []string
	for index := 0; index < len(arguments); index++ {
		if arguments[index] == "--protocol" {
			if index+1 >= len(arguments) {
				return "", nil, fmt.Errorf("--protocol requires a value")
			}
			protocol = arguments[index+1]
			index++
		} else if strings.HasPrefix(arguments[index], "-") {
			return "", nil, fmt.Errorf("unknown option: %s", arguments[index])
		} else {
			names = append(names, arguments[index])
		}
	}
	if protocol != "ssh" && protocol != "https" {
		return "", nil, fmt.Errorf("protocol must be ssh or https")
	}
	return protocol, names, nil
}

func parseSwitches(arguments []string, allowed map[string]*bool) (map[string]bool, []string, error) {
	values := make(map[string]bool, len(allowed))
	var names []string
	for _, argument := range arguments {
		if _, ok := allowed[argument]; ok {
			values[argument] = true
		} else if strings.HasPrefix(argument, "-") {
			return nil, nil, fmt.Errorf("unknown option: %s", argument)
		} else {
			names = append(names, argument)
		}
	}
	return values, names, nil
}

func writeJSON(stdout, stderr io.Writer, value any) int {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		fmt.Fprintf(stderr, "asc: error: encode JSON: %v\n", err)
		return ExitFailure
	}
	return ExitSuccess
}

func writeOperations(output io.Writer, operations []gitrepo.Operation) {
	for _, operation := range operations {
		fmt.Fprintf(output, "%s: %s", operation.Name, operation.Outcome)
		if operation.Detail != "" {
			fmt.Fprintf(output, ": %s", operation.Detail)
		}
		fmt.Fprintln(output)
		for _, plan := range operation.Plan {
			fmt.Fprintf(output, "  %s\n", plan)
		}
	}
	counts := gitrepo.CountOutcomes(operations)
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	fmt.Fprint(output, "Summary:")
	for _, key := range keys {
		fmt.Fprintf(output, " %s=%d", key, counts[key])
	}
	fmt.Fprintln(output)
}

func writeStatuses(output io.Writer, statuses []gitrepo.Status) {
	fmt.Fprintln(output, "REPOSITORY\tBRANCH\tSTATE\tUPSTREAM\tAHEAD/BEHIND")
	for _, status := range statuses {
		if status.Error != "" {
			fmt.Fprintf(output, "%s\tERROR\t%s\n", status.Name, status.Error)
			continue
		}
		state := "clean"
		if !status.Clean {
			state = fmt.Sprintf("dirty(%d)", status.Changes)
		}
		branch := status.Branch
		if status.Detached {
			branch = "detached HEAD"
		}
		upstream := status.Upstream
		if upstream == "" {
			upstream = "-"
		}
		fmt.Fprintf(output, "%s\t%s\t%s\t%s\t+%d/-%d\n", status.Name, branch, state, upstream, status.Ahead, status.Behind)
	}
}

func operationsExit(operations []gitrepo.Operation) int {
	if gitrepo.Failed(operations) {
		return ExitFailure
	}
	return ExitSuccess
}

func operationalError(output io.Writer, err error) int {
	fmt.Fprintf(output, "asc: error: %v\n", err)
	return ExitFailure
}

func usageError(output io.Writer, message, usage string) int {
	fmt.Fprintf(output, "asc: error: %s\n\n%s", message, usage)
	return ExitUsage
}

func helpRequested(arguments []string) bool {
	return len(arguments) == 1 && (arguments[0] == "--help" || arguments[0] == "-h")
}

func commandUsage(command string, arguments []string) (string, bool) {
	switch command {
	case "workspace":
		return workspaceUsage, true
	case "doctor":
		return doctorUsage, true
	case "configure", "build", "test":
		return cmakeUsage(command), true
	case "completion":
		return completionUsage, true
	case "repo":
		if len(arguments) == 1 {
			return repoUsage, true
		}
		if len(arguments) == 2 && (arguments[1] == "--help" || arguments[1] == "-h") {
			switch arguments[0] {
			case "list":
				return repoListUsage, true
			case "clone":
				return repoCloneUsage, true
			case "status":
				return repoStatusUsage, true
			case "sync":
				return repoSyncUsage, true
			}
		}
	}
	return "", false
}

const globalUsage = `Usage: asc [GLOBAL OPTIONS] COMMAND [ARGS]

Safely manage AI4SciComp repositories in one local workspace.

Commands:
  doctor [--json]                 Diagnose tools and GitHub access
  workspace                       Print the resolved workspace
  repo list [--json]              List organization repositories
  repo clone [NAME...]            Clone missing repositories
  repo status [NAME...] [--json]  Inspect local repositories
  repo sync [NAME...] [--dry-run] Fast-forward clean repositories
  configure NAME --preset PRESET  Configure a CMake preset
  build NAME --preset PRESET      Build a CMake preset
  test NAME --preset PRESET       Run a CTest preset
  completion bash                 Print Bash completion

Global options (must precede COMMAND):
  --config PATH
  --organization NAME
  --workspace PATH
  --no-color
  -h, --help
  --version
`

const workspaceUsage = "Usage: asc workspace\n"
const doctorUsage = "Usage: asc doctor [--json]\n"
const repoUsage = "Usage: asc repo {list|clone|status|sync} [OPTIONS] [REPOSITORY...]\n"
const repoListUsage = "Usage: asc repo list [--json]\n"
const repoCloneUsage = "Usage: asc repo clone [REPOSITORY...] [--protocol ssh|https]\n"
const repoStatusUsage = "Usage: asc repo status [REPOSITORY...] [--json]\n"
const repoSyncUsage = "Usage: asc repo sync [REPOSITORY...] [--dry-run]\n"
const completionUsage = "Usage: asc completion bash\n"

func cmakeUsage(operation string) string {
	return fmt.Sprintf("Usage: asc %s REPOSITORY --preset PRESET\n", operation)
}
