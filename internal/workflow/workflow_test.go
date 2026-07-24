package workflow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateWorkflow(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "workspace", "workflows", "software-development")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{
  "schemaVersion": 1,
  "name": "software-development",
  "states": ["proposed", "approved", "done"],
  "initialState": "proposed",
  "terminalStates": ["done"],
  "transitions": [
    {"from": "proposed", "to": "approved"},
    {"from": "approved", "to": "done"}
  ]
}`
	if err := os.WriteFile(filepath.Join(directory, "workflow.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	results, err := Validate(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != "pass" {
		t.Fatalf("Validate() = %+v", results)
	}
}

func TestValidateWorkflowFailures(t *testing.T) {
	root := t.TempDir()
	workflowRoot := filepath.Join(root, "workspace", "workflows")
	if err := os.MkdirAll(filepath.Join(workflowRoot, "invalid"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"schemaVersion":1,"name":"wrong","states":["start"],"initialState":"start","terminalStates":["missing"],"transitions":[]}`
	if err := os.WriteFile(filepath.Join(workflowRoot, "invalid", "workflow.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	results, err := Validate(root, []string{"invalid"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != "failure" {
		t.Fatalf("Validate() = %+v", results)
	}
	if _, err := Validate(root, []string{"../outside"}); err == nil {
		t.Fatal("Validate accepted traversal")
	}
}

func TestValidateWorkflowRejectsSymlinks(t *testing.T) {
	root := t.TempDir()
	workflowRoot := filepath.Join(root, "workspace", "workflows")
	target := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "workflow.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(workflowRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(workflowRoot, "linked")); err != nil {
		t.Fatal(err)
	}
	results, err := Validate(root, []string{"linked"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != "failure" {
		t.Fatalf("Validate() = %+v", results)
	}
}
