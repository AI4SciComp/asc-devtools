// Package workflow validates versioned local workflow manifests.
package workflow

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/AI4SciComp/asc-devtools/internal/workspace"
)

const maxManifestSize = 1 << 20

// Transition describes one allowed workflow state transition.
type Transition struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Manifest is the strict schema-v1 workflow definition.
type Manifest struct {
	SchemaVersion  int          `json:"schemaVersion"`
	Name           string       `json:"name"`
	States         []string     `json:"states"`
	InitialState   string       `json:"initialState"`
	TerminalStates []string     `json:"terminalStates"`
	Transitions    []Transition `json:"transitions"`
}

// Result describes validation of one workflow manifest.
type Result struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// Validate checks named workflows, or every direct-child workflow when names
// is empty. It never follows workflow directory symlinks.
func Validate(root string, names []string) ([]Result, error) {
	workflowRoot := filepath.Join(root, "workspace", "workflows")
	info, err := os.Lstat(workflowRoot)
	if err != nil {
		return nil, fmt.Errorf("inspect workflows directory %s: %w", workflowRoot, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("workflows path is not a real directory: %s", workflowRoot)
	}
	if len(names) == 0 {
		entries, err := os.ReadDir(workflowRoot)
		if err != nil {
			return nil, fmt.Errorf("read workflows directory: %w", err)
		}
		for _, entry := range entries {
			if entry.IsDir() && entry.Type()&os.ModeSymlink == 0 {
				names = append(names, entry.Name())
			}
		}
	}
	sort.Strings(names)
	results := make([]Result, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		if err := workspace.ValidateResourceName(name); err != nil {
			return nil, err
		}
		if _, ok := seen[name]; ok {
			return nil, fmt.Errorf("duplicate workflow name: %s", name)
		}
		seen[name] = struct{}{}
		directory := filepath.Join(workflowRoot, name)
		path := filepath.Join(directory, "workflow.json")
		result := Result{Name: name, Path: path, Status: "pass"}
		if err := validateDirectory(directory); err != nil {
			result.Status, result.Detail = "failure", err.Error()
		} else if err := validateFile(path, name); err != nil {
			result.Status, result.Detail = "failure", err.Error()
		}
		results = append(results, result)
	}
	return results, nil
}

func validateDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("workflow directory must not be a symlink")
	}
	if !info.IsDir() {
		return errors.New("workflow path is not a directory")
	}
	return nil
}

func validateFile(path, expectedName string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("workflow manifest must not be a symlink")
	}
	if !info.Mode().IsRegular() {
		return errors.New("workflow manifest is not a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxManifestSize+1))
	if err != nil {
		return err
	}
	if len(data) > maxManifestSize {
		return errors.New("manifest exceeds 1 MiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return fmt.Errorf("decode strict JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("manifest must contain exactly one JSON object")
	}
	return validateManifest(manifest, expectedName)
}

func validateManifest(manifest Manifest, expectedName string) error {
	if manifest.SchemaVersion != 1 {
		return fmt.Errorf("schemaVersion must be 1")
	}
	if manifest.Name != expectedName {
		return fmt.Errorf("manifest name %q does not match directory %q", manifest.Name, expectedName)
	}
	if len(manifest.States) == 0 {
		return errors.New("states must not be empty")
	}
	states := make(map[string]struct{}, len(manifest.States))
	for _, state := range manifest.States {
		if err := workspace.ValidateResourceName(state); err != nil {
			return fmt.Errorf("invalid state: %w", err)
		}
		if _, exists := states[state]; exists {
			return fmt.Errorf("duplicate state: %s", state)
		}
		states[state] = struct{}{}
	}
	if _, ok := states[manifest.InitialState]; !ok {
		return fmt.Errorf("initialState is not in states: %s", manifest.InitialState)
	}
	if len(manifest.TerminalStates) == 0 {
		return errors.New("terminalStates must not be empty")
	}
	terminals := make(map[string]struct{}, len(manifest.TerminalStates))
	for _, state := range manifest.TerminalStates {
		if _, ok := states[state]; !ok {
			return fmt.Errorf("terminal state is not in states: %s", state)
		}
		if _, exists := terminals[state]; exists {
			return fmt.Errorf("duplicate terminal state: %s", state)
		}
		terminals[state] = struct{}{}
	}
	transitions := make(map[Transition]struct{}, len(manifest.Transitions))
	for _, transition := range manifest.Transitions {
		if _, ok := states[transition.From]; !ok {
			return fmt.Errorf("transition source is not in states: %s", transition.From)
		}
		if _, ok := states[transition.To]; !ok {
			return fmt.Errorf("transition destination is not in states: %s", transition.To)
		}
		if _, exists := transitions[transition]; exists {
			return fmt.Errorf("duplicate transition: %s -> %s", transition.From, transition.To)
		}
		transitions[transition] = struct{}{}
	}
	return nil
}
