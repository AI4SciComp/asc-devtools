package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitializeAndValidateCoordination(t *testing.T) {
	root := filepath.Join(t.TempDir(), "AI4SciComp")
	plan, err := PlanCoordination(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 9 || plan[0].Action != "create" {
		t.Fatalf("PlanCoordination() = %+v", plan)
	}
	actions, err := InitializeCoordination(root)
	if err != nil {
		t.Fatal(err)
	}
	if actions[0].Action != "created" {
		t.Fatalf("InitializeCoordination() = %+v", actions)
	}
	for _, check := range ValidateCoordination(root) {
		if check.Status != "pass" {
			t.Fatalf("ValidateCoordination() = %+v", check)
		}
	}
	plan, err = PlanCoordination(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range plan {
		if action.Action != "keep" {
			t.Fatalf("second plan action = %+v", action)
		}
	}
}

func TestCoordinationRejectsFilesAndSymlinks(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, coordinationDirectory), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := PlanCoordination(root); err == nil {
		t.Fatal("PlanCoordination accepted a file")
	}

	otherRoot := t.TempDir()
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(otherRoot, coordinationDirectory)); err != nil {
		t.Fatal(err)
	}
	if _, err := PlanCoordination(otherRoot); err == nil {
		t.Fatal("PlanCoordination accepted a symlink")
	}
}

func TestInitializeAgent(t *testing.T) {
	root := t.TempDir()
	if _, err := InitializeCoordination(root); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanAgent(root, "researcher")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(plan.Definition); !os.IsNotExist(err) {
		t.Fatalf("PlanAgent changed the filesystem: %v", err)
	}
	if _, err := InitializeAgent(root, "researcher"); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(plan.Definition)
	if err != nil {
		t.Fatal(err)
	}
	if len(content) == 0 {
		t.Fatal("agent definition is empty")
	}
	if _, err := InitializeAgent(root, "researcher"); err == nil {
		t.Fatal("InitializeAgent overwrote an existing agent")
	}
	if _, err := PlanAgent(root, "../outside"); err == nil {
		t.Fatal("PlanAgent accepted traversal")
	}
}
