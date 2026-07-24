// Package app parses commands and coordinates asc services.
package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/AI4SciComp/asc-devtools/internal/cmake"
	"github.com/AI4SciComp/asc-devtools/internal/cmakevendor"
	"github.com/AI4SciComp/asc-devtools/internal/completion"
	"github.com/AI4SciComp/asc-devtools/internal/config"
	"github.com/AI4SciComp/asc-devtools/internal/doctor"
	gitrepo "github.com/AI4SciComp/asc-devtools/internal/git"
	"github.com/AI4SciComp/asc-devtools/internal/github"
	"github.com/AI4SciComp/asc-devtools/internal/process"
	"github.com/AI4SciComp/asc-devtools/internal/selfupdate"
	"github.com/AI4SciComp/asc-devtools/internal/workflow"
	"github.com/AI4SciComp/asc-devtools/internal/workspace"
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
	Executable  func() (string, error)
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
		return runWorkspace(cfg, commandArguments, dependencies)
	case "agent":
		return runAgent(cfg, commandArguments, dependencies)
	case "workflow":
		return runWorkflow(cfg, commandArguments, dependencies)
	case "repo":
		return runRepo(ctx, cfg, commandArguments, dependencies)
	case "configure", "build", "test":
		return runCMake(ctx, cfg, command, commandArguments, dependencies)
	case "cmake":
		return runCMakeGroup(ctx, cfg, commandArguments, dependencies)
	case "doctor":
		return runDoctor(ctx, cfg, commandArguments, dependencies)
	case "update":
		return runUpdate(ctx, cfg, commandArguments, dependencies)
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

func runWorkspace(cfg config.Config, arguments []string, dependencies Dependencies) int {
	if len(arguments) == 0 {
		fmt.Fprintln(dependencies.Stdout, cfg.Workspace)
		return ExitSuccess
	}
	subcommand, commandArguments := arguments[0], arguments[1:]
	if len(commandArguments) == 1 && helpRequested(commandArguments) {
		switch subcommand {
		case "init":
			fmt.Fprint(dependencies.Stdout, workspaceInitUsage)
			return ExitSuccess
		case "validate":
			fmt.Fprint(dependencies.Stdout, workspaceValidateUsage)
			return ExitSuccess
		}
	}
	switch subcommand {
	case "init":
		switches, names, err := parseSwitches(commandArguments, map[string]*bool{"--dry-run": new(bool)})
		if err != nil || len(names) != 0 {
			if err == nil {
				err = errors.New("workspace init takes no arguments")
			}
			return usageError(dependencies.Stderr, err.Error(), workspaceInitUsage)
		}
		var actions []workspace.Action
		if switches["--dry-run"] {
			actions, err = workspace.PlanCoordination(cfg.Workspace)
		} else {
			actions, err = workspace.InitializeCoordination(cfg.Workspace)
		}
		if err != nil {
			return operationalError(dependencies.Stderr, err)
		}
		for _, action := range actions {
			fmt.Fprintf(dependencies.Stdout, "%s\t%s\n", action.Action, action.Path)
		}
		return ExitSuccess
	case "validate":
		switches, names, err := parseSwitches(commandArguments, map[string]*bool{"--json": new(bool)})
		if err != nil || len(names) != 0 {
			if err == nil {
				err = errors.New("workspace validate takes no arguments")
			}
			return usageError(dependencies.Stderr, err.Error(), workspaceValidateUsage)
		}
		checks := workspace.ValidateCoordination(cfg.Workspace)
		if switches["--json"] {
			if code := writeJSON(dependencies.Stdout, dependencies.Stderr, checks); code != ExitSuccess {
				return code
			}
		} else {
			for _, check := range checks {
				fmt.Fprintf(dependencies.Stdout, "%s\t%s", check.Status, check.Path)
				if check.Detail != "" {
					fmt.Fprintf(dependencies.Stdout, "\t%s", check.Detail)
				}
				fmt.Fprintln(dependencies.Stdout)
			}
		}
		for _, check := range checks {
			if check.Status == "failure" {
				return ExitFailure
			}
		}
		return ExitSuccess
	default:
		return usageError(dependencies.Stderr, "unknown workspace command: "+subcommand, workspaceUsage)
	}
}

func runAgent(cfg config.Config, arguments []string, dependencies Dependencies) int {
	if len(arguments) == 0 {
		return usageError(dependencies.Stderr, "an agent command is required", agentUsage)
	}
	subcommand, commandArguments := arguments[0], arguments[1:]
	if subcommand != "init" {
		return usageError(dependencies.Stderr, "unknown agent command: "+subcommand, agentUsage)
	}
	if len(commandArguments) == 1 && helpRequested(commandArguments) {
		fmt.Fprint(dependencies.Stdout, agentInitUsage)
		return ExitSuccess
	}
	switches, names, err := parseSwitches(commandArguments, map[string]*bool{"--dry-run": new(bool)})
	if err != nil || len(names) != 1 {
		if err == nil {
			err = errors.New("agent init requires one name")
		}
		return usageError(dependencies.Stderr, err.Error(), agentInitUsage)
	}
	var plan workspace.AgentPlan
	if switches["--dry-run"] {
		plan, err = workspace.PlanAgent(cfg.Workspace, names[0])
	} else {
		plan, err = workspace.InitializeAgent(cfg.Workspace, names[0])
	}
	if err != nil {
		return operationalError(dependencies.Stderr, err)
	}
	action := "created"
	if switches["--dry-run"] {
		action = "create"
	}
	fmt.Fprintf(dependencies.Stdout, "%s\t%s\n", action, plan.Definition)
	return ExitSuccess
}

func runWorkflow(cfg config.Config, arguments []string, dependencies Dependencies) int {
	if len(arguments) == 0 {
		return usageError(dependencies.Stderr, "a workflow command is required", workflowUsage)
	}
	subcommand, commandArguments := arguments[0], arguments[1:]
	if subcommand != "validate" {
		return usageError(dependencies.Stderr, "unknown workflow command: "+subcommand, workflowUsage)
	}
	if len(commandArguments) == 1 && helpRequested(commandArguments) {
		fmt.Fprint(dependencies.Stdout, workflowValidateUsage)
		return ExitSuccess
	}
	switches, names, err := parseSwitches(commandArguments, map[string]*bool{"--json": new(bool)})
	if err != nil {
		return usageError(dependencies.Stderr, err.Error(), workflowValidateUsage)
	}
	results, err := workflow.Validate(cfg.Workspace, names)
	if err != nil {
		return operationalError(dependencies.Stderr, err)
	}
	if switches["--json"] {
		if code := writeJSON(dependencies.Stdout, dependencies.Stderr, results); code != ExitSuccess {
			return code
		}
	} else if len(results) == 0 {
		fmt.Fprintln(dependencies.Stdout, "no workflows found")
	} else {
		for _, result := range results {
			fmt.Fprintf(dependencies.Stdout, "%s\t%s", result.Status, result.Name)
			if result.Detail != "" {
				fmt.Fprintf(dependencies.Stdout, "\t%s", result.Detail)
			}
			fmt.Fprintln(dependencies.Stdout)
		}
	}
	for _, result := range results {
		if result.Status == "failure" {
			return ExitFailure
		}
	}
	return ExitSuccess
}

func runCMakeGroup(ctx context.Context, cfg config.Config, arguments []string, dependencies Dependencies) int {
	if len(arguments) == 0 {
		return usageError(dependencies.Stderr, "a cmake command is required", cmakeGroupUsage)
	}
	subcommand := arguments[0]
	commandArguments := arguments[1:]
	if len(commandArguments) == 1 && helpRequested(commandArguments) {
		fmt.Fprint(dependencies.Stdout, cmakeSubcommandUsage(subcommand))
		return ExitSuccess
	}
	manager := cmake.Manager{Runner: dependencies.Runner, Config: cfg, Reporter: dependencies.Stderr}
	switch subcommand {
	case "configure", "build", "test":
		repository, options, err := parseCMakeOptions(subcommand, commandArguments)
		if err != nil {
			return usageError(dependencies.Stderr, err.Error(), cmakeSubcommandUsage(subcommand))
		}
		return cmakeExit(dependencies.Stderr, manager.RunOperation(ctx, subcommand, repository, options))
	case "workflow":
		repository, options, err := parseWorkflowOptions(commandArguments)
		if err != nil {
			return usageError(dependencies.Stderr, err.Error(), cmakeWorkflowUsage)
		}
		return cmakeExit(dependencies.Stderr, manager.Workflow(ctx, repository, options))
	case "presets":
		switches, names, err := parseSwitches(commandArguments, map[string]*bool{"--json": new(bool)})
		if err != nil || len(names) != 1 {
			if err == nil {
				err = fmt.Errorf("cmake presets requires one repository")
			}
			return usageError(dependencies.Stderr, err.Error(), cmakePresetsUsage)
		}
		presets, err := manager.ListPresets(ctx, names[0])
		if err != nil {
			return cmakeExit(dependencies.Stderr, err)
		}
		if switches["--json"] {
			return writeJSON(dependencies.Stdout, dependencies.Stderr, presets)
		}
		for _, group := range []struct {
			name   string
			values []string
		}{{"configure", presets.Configure}, {"build", presets.Build}, {"test", presets.Test}} {
			for _, preset := range group.values {
				fmt.Fprintf(dependencies.Stdout, "%s\t%s\n", group.name, preset)
			}
		}
		return ExitSuccess
	case "vendor":
		return runVendor(ctx, cfg, commandArguments, dependencies)
	default:
		return usageError(dependencies.Stderr, "unknown cmake command: "+subcommand, cmakeGroupUsage)
	}
}

func parseCMakeOptions(operation string, arguments []string) (string, cmake.Options, error) {
	var repository string
	var options cmake.Options
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		switch argument {
		case "--preset", "--target", "--label":
			if index+1 >= len(arguments) {
				return "", cmake.Options{}, fmt.Errorf("%s requires a value", argument)
			}
			value := arguments[index+1]
			index++
			switch argument {
			case "--preset":
				options.Preset = value
			case "--target":
				if operation != "build" {
					return "", cmake.Options{}, fmt.Errorf("--target is valid only for cmake build")
				}
				options.Targets = append(options.Targets, value)
			case "--label":
				if operation != "test" {
					return "", cmake.Options{}, fmt.Errorf("--label is valid only for cmake test")
				}
				options.Label = value
			}
		case "--output-on-failure":
			if operation != "test" {
				return "", cmake.Options{}, fmt.Errorf("--output-on-failure is valid only for cmake test")
			}
			options.OutputOnFailure = true
		default:
			if strings.HasPrefix(argument, "-") {
				return "", cmake.Options{}, fmt.Errorf("unknown option: %s", argument)
			}
			if repository != "" {
				return "", cmake.Options{}, fmt.Errorf("cmake %s takes one repository", operation)
			}
			repository = argument
		}
	}
	if repository == "" || options.Preset == "" {
		return "", cmake.Options{}, errors.New("repository and --preset are required")
	}
	return repository, options, nil
}

func parseWorkflowOptions(arguments []string) (string, cmake.WorkflowOptions, error) {
	var repository string
	var options cmake.WorkflowOptions
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		if argument == "--configure-preset" || argument == "--build-preset" || argument == "--test-preset" {
			if index+1 >= len(arguments) {
				return "", options, fmt.Errorf("%s requires a value", argument)
			}
			value := arguments[index+1]
			index++
			switch argument {
			case "--configure-preset":
				options.ConfigurePreset = value
			case "--build-preset":
				options.BuildPreset = value
			case "--test-preset":
				options.TestPreset = value
			}
		} else if strings.HasPrefix(argument, "-") {
			return "", options, fmt.Errorf("unknown option: %s", argument)
		} else if repository == "" {
			repository = argument
		} else {
			return "", options, errors.New("cmake workflow takes one repository")
		}
	}
	if repository == "" || options.ConfigurePreset == "" || options.BuildPreset == "" || options.TestPreset == "" {
		return "", options, errors.New("repository and all three workflow presets are required")
	}
	return repository, options, nil
}

func runVendor(ctx context.Context, cfg config.Config, arguments []string, dependencies Dependencies) int {
	if len(arguments) == 0 {
		return usageError(dependencies.Stderr, "a vendor command is required", cmakeVendorUsage)
	}
	subcommand, commandArguments := arguments[0], arguments[1:]
	service := cmakevendor.Service{Runner: dependencies.Runner, Config: cfg}
	if subcommand == "status" {
		switches, names, err := parseSwitches(commandArguments, map[string]*bool{"--json": new(bool)})
		if err != nil || len(names) != 1 {
			if err == nil {
				err = errors.New("vendor status requires one repository")
			}
			return usageError(dependencies.Stderr, err.Error(), cmakeVendorStatusUsage)
		}
		status := service.Status(ctx, names[0])
		if switches["--json"] {
			return writeJSON(dependencies.Stdout, dependencies.Stderr, status)
		}
		fmt.Fprintf(dependencies.Stdout, "%s: %s", status.Repository, status.State)
		if status.Detail != "" {
			fmt.Fprintf(dependencies.Stdout, ": %s", status.Detail)
		}
		fmt.Fprintln(dependencies.Stdout)
		for _, extra := range status.ExtraFiles {
			fmt.Fprintf(dependencies.Stdout, "  extra: %s\n", extra)
		}
		return ExitSuccess
	}
	if subcommand != "plan" && subcommand != "apply" {
		return usageError(dependencies.Stderr, "unknown vendor command: "+subcommand, cmakeVendorUsage)
	}
	repository, source, ref, jsonOutput, yes, err := parseVendorOptions(commandArguments)
	if err != nil || repository == "" {
		if err == nil {
			err = fmt.Errorf("vendor %s requires one repository", subcommand)
		}
		return usageError(dependencies.Stderr, err.Error(), map[bool]string{true: cmakeVendorApplyUsage, false: cmakeVendorPlanUsage}[subcommand == "apply"])
	}
	if subcommand == "plan" && yes {
		return usageError(dependencies.Stderr, "--yes is valid only for vendor apply", cmakeVendorPlanUsage)
	}
	if subcommand == "apply" && jsonOutput {
		return usageError(dependencies.Stderr, "--json is valid only for vendor plan", cmakeVendorApplyUsage)
	}
	plan, err := service.Plan(ctx, repository, source, ref)
	if err != nil {
		return operationalError(dependencies.Stderr, err)
	}
	if subcommand == "plan" {
		if jsonOutput {
			return writeJSON(dependencies.Stdout, dependencies.Stderr, plan)
		}
		writeVendorPlan(dependencies.Stdout, plan)
		return ExitSuccess
	}
	if !yes && !confirmVendor(dependencies.Stdin, dependencies.Stderr, plan) {
		fmt.Fprintln(dependencies.Stderr, "asc: vendor apply cancelled")
		return ExitFailure
	}
	if err := service.Apply(ctx, plan); err != nil {
		return operationalError(dependencies.Stderr, err)
	}
	writeVendorPlan(dependencies.Stdout, plan)
	fmt.Fprintln(dependencies.Stdout, "Applied vendored asc-cmake files. Review with: git diff -- cmake/asc")
	return ExitSuccess
}

func parseVendorOptions(arguments []string) (repository, source, ref string, jsonOutput, yes bool, err error) {
	for index := 0; index < len(arguments); index++ {
		switch arguments[index] {
		case "--source", "--ref":
			if index+1 >= len(arguments) {
				return "", "", "", false, false, fmt.Errorf("%s requires a value", arguments[index])
			}
			if arguments[index] == "--source" {
				source = arguments[index+1]
			} else {
				ref = arguments[index+1]
			}
			index++
		case "--json":
			jsonOutput = true
		case "--yes":
			yes = true
		default:
			if strings.HasPrefix(arguments[index], "-") {
				return "", "", "", false, false, fmt.Errorf("unknown option: %s", arguments[index])
			}
			if repository != "" {
				return "", "", "", false, false, errors.New("vendor command takes one repository")
			}
			repository = arguments[index]
		}
	}
	return repository, source, ref, jsonOutput, yes, nil
}

func confirmVendor(input io.Reader, output io.Writer, plan cmakevendor.Plan) bool {
	writeVendorPlan(output, plan)
	fmt.Fprint(output, "Apply this exact plan? [y/N] ")
	line, err := bufio.NewReader(input).ReadString('\n')
	if err != nil && len(line) == 0 {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}

func writeVendorPlan(output io.Writer, plan cmakevendor.Plan) {
	fmt.Fprintf(output, "asc-cmake %s (%s) -> %s\n", plan.Version, plan.Commit, plan.Repository)
	for _, action := range plan.Actions {
		fmt.Fprintf(output, "  %s\t%s\n", action.Action, action.Path)
	}
}

func cmakeExit(output io.Writer, err error) int {
	if err == nil {
		return ExitSuccess
	}
	fmt.Fprintf(output, "asc: error: %v\n", err)
	code := process.ExitCode(err, ExitFailure)
	if code < 1 || code > 125 {
		return ExitFailure
	}
	return code
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
	if dependencies.Executable == nil {
		dependencies.Executable = os.Executable
	}
	return dependencies
}

func runUpdate(ctx context.Context, cfg config.Config, arguments []string, dependencies Dependencies) int {
	var checkOnly, assumeYes bool
	var prefix string
	for index := 0; index < len(arguments); index++ {
		switch arguments[index] {
		case "--check":
			checkOnly = true
		case "--yes":
			assumeYes = true
		case "--prefix":
			if index+1 >= len(arguments) {
				return usageError(dependencies.Stderr, "--prefix requires a value", updateUsage)
			}
			prefix = arguments[index+1]
			index++
		default:
			return usageError(dependencies.Stderr, "unknown option: "+arguments[index], updateUsage)
		}
	}
	client, resolvedConfig := githubClient(ctx, cfg, dependencies)
	err := selfupdate.Run(ctx, selfupdate.Options{
		CurrentVersion: dependencies.VersionInfo.Version,
		Prefix:         prefix,
		CheckOnly:      checkOnly,
		AssumeYes:      assumeYes,
		Token:          resolvedConfig.GitHubToken,
		BaseURL:        client.BaseURL,
		HTTPClient:     client.HTTPClient,
		Stdin:          dependencies.Stdin,
		Stdout:         dependencies.Stdout,
		Stderr:         dependencies.Stderr,
		Executable:     dependencies.Executable,
	})
	if err != nil {
		return operationalError(dependencies.Stderr, err)
	}
	return ExitSuccess
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
		case "save":
			usage = repoSaveUsage
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
	case "save":
		repository, message, dryRun, yes, err := parseRepoSave(commandArguments)
		if err != nil {
			return usageError(dependencies.Stderr, err.Error(), repoSaveUsage)
		}
		manager := gitrepo.Manager{Runner: dependencies.Runner, Config: cfg}
		plan, err := manager.PlanSave(ctx, repository, message)
		if err != nil {
			return operationalError(dependencies.Stderr, err)
		}
		if dryRun {
			writeSavePlan(dependencies.Stdout, plan)
			return ExitSuccess
		}
		if !yes {
			writeSavePlan(dependencies.Stderr, plan)
			fmt.Fprint(dependencies.Stderr, "Commit all listed working-tree changes and push? [y/N] ")
			line, readErr := bufio.NewReader(dependencies.Stdin).ReadString('\n')
			answer := strings.ToLower(strings.TrimSpace(line))
			if (readErr != nil && line == "") || (answer != "y" && answer != "yes") {
				fmt.Fprintln(dependencies.Stderr, "asc: repo save cancelled")
				return ExitFailure
			}
		}
		operation := manager.ApplySave(ctx, plan)
		writeOperations(dependencies.Stdout, []gitrepo.Operation{operation})
		return operationsExit([]gitrepo.Operation{operation})
	default:
		return usageError(dependencies.Stderr, "unknown repo command: "+subcommand, repoUsage)
	}
}

func parseRepoSave(arguments []string) (repository, message string, dryRun, yes bool, err error) {
	for index := 0; index < len(arguments); index++ {
		switch arguments[index] {
		case "--message":
			if index+1 >= len(arguments) {
				return "", "", false, false, errors.New("--message requires a value")
			}
			message = arguments[index+1]
			index++
		case "--dry-run":
			dryRun = true
		case "--yes":
			yes = true
		default:
			if strings.HasPrefix(arguments[index], "-") {
				return "", "", false, false, fmt.Errorf("unknown option: %s", arguments[index])
			}
			if repository != "" {
				return "", "", false, false, errors.New("repo save takes one repository")
			}
			repository = arguments[index]
		}
	}
	if repository == "" {
		return "", "", false, false, errors.New("repo save requires one repository")
	}
	if message == "" {
		message = "Updated at " + time.Now().Format("2006-01-02 15:04:05")
	}
	return repository, message, dryRun, yes, nil
}

func writeSavePlan(output io.Writer, plan gitrepo.SavePlan) {
	fmt.Fprintf(output, "%s: planned: %s\n", plan.Operation.Name, plan.Operation.Detail)
	for _, command := range plan.Operation.Plan {
		fmt.Fprintf(output, "  %s\n", command)
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

func githubClient(_ context.Context, cfg config.Config, dependencies Dependencies) (*github.Client, config.Config) {
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
	case "agent":
		return agentUsage, true
	case "workflow":
		return workflowUsage, true
	case "doctor":
		return doctorUsage, true
	case "configure", "build", "test":
		return cmakeUsage(command), true
	case "cmake":
		return cmakeGroupUsage, true
	case "completion":
		return completionUsage, true
	case "update":
		return updateUsage, true
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
			case "save":
				return repoSaveUsage, true
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
  workspace init [--dry-run]      Create the coordination directory layout
  workspace validate [--json]     Validate the coordination directory layout
  agent init NAME [--dry-run]     Scaffold one draft agent definition
  workflow validate [NAME...]     Validate strict workflow manifests (--json available)
  repo list [--json]              List organization repositories
  repo clone [NAME...]            Clone missing repositories
  repo status [NAME...] [--json]  Inspect local repositories
  repo sync [NAME...] [--dry-run] Download remote fast-forwards into clean repositories
  repo save NAME [--message TEXT] Commit local changes and push the tracked branch
  configure NAME --preset PRESET  Configure a CMake preset
  build NAME --preset PRESET      Build a CMake preset
  test NAME --preset PRESET       Run a CTest preset
  cmake COMMAND ...               CMake workflows and local vendoring
  update [--check] [--yes]        Update asc to the latest release
  completion bash                 Print Bash completion

Global options (must precede COMMAND):
  --config PATH
  --organization NAME
  --workspace PATH
  --no-color
  -h, --help
  --version
`

const workspaceUsage = `Usage: asc workspace [COMMAND]

Without COMMAND, print the resolved workspace.

Commands:
  init [--dry-run]
  validate [--json]
`
const workspaceInitUsage = "Usage: asc workspace init [--dry-run]\n"
const workspaceValidateUsage = "Usage: asc workspace validate [--json]\n"
const agentUsage = "Usage: asc agent init NAME [--dry-run]\n"
const agentInitUsage = "Usage: asc agent init NAME [--dry-run]\n"
const workflowUsage = "Usage: asc workflow validate [NAME...] [--json]\n"
const workflowValidateUsage = "Usage: asc workflow validate [NAME...] [--json]\n"
const doctorUsage = "Usage: asc doctor [--json]\n"
const repoUsage = "Usage: asc repo {list|clone|status|sync|save} [OPTIONS] [REPOSITORY...]\n"
const repoListUsage = "Usage: asc repo list [--json]\n"
const repoCloneUsage = "Usage: asc repo clone [REPOSITORY...] [--protocol ssh|https]\n"
const repoStatusUsage = "Usage: asc repo status [REPOSITORY...] [--json]\n"
const repoSyncUsage = "Usage: asc repo sync [REPOSITORY...] [--dry-run]\n"
const repoSaveUsage = "Usage: asc repo save REPOSITORY [--message TEXT] [--dry-run] [--yes]\n"
const completionUsage = "Usage: asc completion bash\n"
const updateUsage = "Usage: asc update [--check] [--yes] [--prefix PATH]\n"
const cmakeGroupUsage = `Usage: asc cmake COMMAND [OPTIONS]

Commands:
  configure REPOSITORY --preset PRESET
  build REPOSITORY --preset PRESET [--target TARGET]...
  test REPOSITORY --preset PRESET [--label LABEL] [--output-on-failure]
  workflow REPOSITORY --configure-preset PRESET --build-preset PRESET --test-preset PRESET
  presets REPOSITORY [--json]
  vendor {status|plan|apply} ...
`
const cmakeConfigureUsage = "Usage: asc cmake configure REPOSITORY --preset PRESET\n"
const cmakeBuildUsage = "Usage: asc cmake build REPOSITORY --preset PRESET [--target TARGET]...\n"
const cmakeTestUsage = "Usage: asc cmake test REPOSITORY --preset PRESET [--label LABEL] [--output-on-failure]\n"
const cmakeWorkflowUsage = "Usage: asc cmake workflow REPOSITORY --configure-preset PRESET --build-preset PRESET --test-preset PRESET\n"
const cmakePresetsUsage = "Usage: asc cmake presets REPOSITORY [--json]\n"
const cmakeVendorUsage = "Usage: asc cmake vendor {status|plan|apply} ...\n"
const cmakeVendorStatusUsage = "Usage: asc cmake vendor status REPOSITORY [--json]\n"
const cmakeVendorPlanUsage = "Usage: asc cmake vendor plan REPOSITORY [--source PATH] [--ref REF] [--json]\n"
const cmakeVendorApplyUsage = "Usage: asc cmake vendor apply REPOSITORY [--source PATH] [--ref REF] [--yes]\n"

func cmakeSubcommandUsage(operation string) string {
	switch operation {
	case "configure":
		return cmakeConfigureUsage
	case "build":
		return cmakeBuildUsage
	case "test":
		return cmakeTestUsage
	case "workflow":
		return cmakeWorkflowUsage
	case "presets":
		return cmakePresetsUsage
	case "vendor":
		return cmakeVendorUsage
	default:
		return cmakeGroupUsage
	}
}

func cmakeUsage(operation string) string {
	return fmt.Sprintf("Usage: asc %s REPOSITORY --preset PRESET\n", operation)
}
