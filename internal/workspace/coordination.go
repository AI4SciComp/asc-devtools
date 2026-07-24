package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

const coordinationDirectory = "workspace"

var (
	coordinationChildren = []string{
		"agents",
		"workflows",
		"memory",
		"tools",
		"config",
		"projects",
		"docs",
	}
	resourceNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)
)

// Action describes one directory retained or created by workspace init.
type Action struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}

// Check describes one workspace validation result.
type Check struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// AgentPlan describes the files created for one agent definition.
type AgentPlan struct {
	Name       string `json:"name"`
	Directory  string `json:"directory"`
	Definition string `json:"definition"`
}

// ValidateResourceName validates an agent or workflow path segment.
func ValidateResourceName(name string) error {
	if !resourceNamePattern.MatchString(name) {
		return fmt.Errorf("name %q must match %s", name, resourceNamePattern)
	}
	return nil
}

// PlanCoordination reports exactly which workspace directories already exist
// and which would be created.
func PlanCoordination(root string) ([]Action, error) {
	paths := coordinationPaths(root)
	actions := make([]Action, 0, len(paths))
	for _, path := range paths {
		info, err := os.Lstat(path)
		switch {
		case errors.Is(err, os.ErrNotExist):
			actions = append(actions, Action{Path: path, Action: "create"})
		case err != nil:
			return nil, fmt.Errorf("inspect %s: %w", path, err)
		case info.Mode()&os.ModeSymlink != 0:
			return nil, fmt.Errorf("workspace path is a symlink: %s", path)
		case !info.IsDir():
			return nil, fmt.Errorf("workspace path is not a directory: %s", path)
		default:
			actions = append(actions, Action{Path: path, Action: "keep"})
		}
	}
	return actions, nil
}

// InitializeCoordination creates only missing approved directories and rolls
// back directories created by this call if an operation fails.
func InitializeCoordination(root string) ([]Action, error) {
	actions, err := PlanCoordination(root)
	if err != nil {
		return nil, err
	}
	var created []string
	for _, action := range actions {
		if action.Action != "create" {
			continue
		}
		if err := os.Mkdir(action.Path, 0o755); err != nil {
			for index := len(created) - 1; index >= 0; index-- {
				_ = os.Remove(created[index])
			}
			return nil, fmt.Errorf("create %s: %w", action.Path, err)
		}
		created = append(created, action.Path)
	}
	for index := range actions {
		if actions[index].Action == "create" {
			actions[index].Action = "created"
		}
	}
	return actions, nil
}

// ValidateCoordination checks the canonical workspace directory set without
// changing the filesystem.
func ValidateCoordination(root string) []Check {
	paths := coordinationPaths(root)
	checks := make([]Check, 0, len(paths))
	for _, path := range paths {
		check := Check{Path: path, Status: "pass"}
		info, err := os.Lstat(path)
		switch {
		case errors.Is(err, os.ErrNotExist):
			check.Status, check.Detail = "failure", "missing"
		case err != nil:
			check.Status, check.Detail = "failure", err.Error()
		case info.Mode()&os.ModeSymlink != 0:
			check.Status, check.Detail = "failure", "symlink is not allowed"
		case !info.IsDir():
			check.Status, check.Detail = "failure", "not a directory"
		}
		checks = append(checks, check)
	}
	return checks
}

// PlanAgent validates one new agent definition target without changing it.
func PlanAgent(root, name string) (AgentPlan, error) {
	if err := ValidateResourceName(name); err != nil {
		return AgentPlan{}, err
	}
	parent := filepath.Join(root, coordinationDirectory, "agents")
	info, err := os.Lstat(parent)
	if err != nil {
		return AgentPlan{}, fmt.Errorf("inspect agents directory %s: %w", parent, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return AgentPlan{}, fmt.Errorf("agents path is not a real directory: %s", parent)
	}
	directory := filepath.Join(parent, name)
	if _, err := os.Lstat(directory); err == nil {
		return AgentPlan{}, fmt.Errorf("agent already exists: %s", directory)
	} else if !errors.Is(err, os.ErrNotExist) {
		return AgentPlan{}, fmt.Errorf("inspect agent %s: %w", directory, err)
	}
	return AgentPlan{
		Name:       name,
		Directory:  directory,
		Definition: filepath.Join(directory, "AGENT.md"),
	}, nil
}

// InitializeAgent creates one new agent definition without overwriting an
// existing path. It removes its new directory if writing the definition fails.
func InitializeAgent(root, name string) (AgentPlan, error) {
	plan, err := PlanAgent(root, name)
	if err != nil {
		return AgentPlan{}, err
	}
	if err := os.Mkdir(plan.Directory, 0o755); err != nil {
		return AgentPlan{}, fmt.Errorf("create agent directory: %w", err)
	}
	content := agentDefinition(name)
	if err := os.WriteFile(plan.Definition, []byte(content), 0o644); err != nil {
		_ = os.Remove(plan.Definition)
		_ = os.Remove(plan.Directory)
		return AgentPlan{}, fmt.Errorf("write agent definition: %w", err)
	}
	return plan, nil
}

func coordinationPaths(root string) []string {
	paths := []string{root, filepath.Join(root, coordinationDirectory)}
	for _, name := range coordinationChildren {
		paths = append(paths, filepath.Join(root, coordinationDirectory, name))
	}
	return paths
}

func agentDefinition(name string) string {
	return fmt.Sprintf(`# Agent: %s

Status: draft

## Role and responsibility

Define the agent's bounded role before activation.

## Allowed inputs

List the reviewed inputs this agent may consume.

## Expected outputs

List the artifacts and evidence this agent must produce.

## Tools and permissions

Grant only the tools and permissions required by the role.

## Stop conditions

List ambiguity, safety, provenance, and verification conditions that require a stop.

## Evidence requirements

Record source revisions, commands, tests, and unresolved risks.

## Handoff format

Define the structured handoff expected by the next owner.

## Verification owner

Assign an independent verifier before changing status from draft.
`, name)
}
