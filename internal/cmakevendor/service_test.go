package cmakevendor

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/AI4SciComp/asc-devtools/internal/config"
	"github.com/AI4SciComp/asc-devtools/internal/process"
)

func TestManifestIsDeterministicAndStrict(t *testing.T) {
	manifest := Manifest{
		SchemaVersion:    1,
		SourceRepository: "AI4SciComp/asc-cmake",
		Version:          "1.2.3",
		Commit:           strings.Repeat("a", 40),
		Files: []FileRecord{
			{Path: "modules/z.cmake", SHA256: strings.Repeat("0", 64)},
			{Path: "LICENSE", SHA256: strings.Repeat("1", 64)},
		},
	}
	encoded, err := EncodeManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Index(encoded, []byte("LICENSE")) > bytes.Index(encoded, []byte("modules/z.cmake")) || !bytes.HasSuffix(encoded, []byte("\n")) {
		t.Fatalf("manifest is not sorted and newline terminated:\n%s", encoded)
	}
	decoded, err := DecodeManifest(bytes.NewReader(encoded))
	if err != nil || decoded.Files[0].Path != "LICENSE" {
		t.Fatalf("DecodeManifest() = %+v, %v", decoded, err)
	}
	for _, invalid := range []string{
		`{"schemaVersion":2,"sourceRepository":"AI4SciComp/asc-cmake","version":"1","commit":"a","files":[]}`,
		`{"schemaVersion":1,"sourceRepository":"AI4SciComp/asc-cmake","version":"1","commit":"a","files":[],"unknown":true}`,
		`{"schemaVersion":1,"sourceRepository":"AI4SciComp/asc-cmake","version":"1","commit":"a","files":[{"path":"../escape","sha256":"` + strings.Repeat("0", 64) + `"}]}`,
	} {
		if _, err := DecodeManifest(strings.NewReader(invalid)); err == nil {
			t.Fatalf("DecodeManifest accepted %s", invalid)
		}
	}
}

func TestPlanApplyAndStatusLifecycle(t *testing.T) {
	service, workspace, sourceRoot, consumer := setupService(t)
	ctx := context.Background()

	status := service.Status(ctx, "asc-cpp")
	if status.State != "not-vendored" {
		t.Fatalf("initial status = %+v", status)
	}
	plan, err := service.Plan(ctx, "asc-cpp", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := actionPairs(plan.Actions); !reflect.DeepEqual(got, []string{"add:LICENSE", "add:modules/ASCWarnings.cmake"}) {
		t.Fatalf("initial actions = %v", got)
	}
	if err := service.Apply(ctx, plan); err != nil {
		t.Fatal(err)
	}
	if status := service.Status(ctx, "asc-cpp"); status.State != "current" {
		t.Fatalf("current status = %+v", status)
	}

	target := filepath.Join(consumer, "cmake", "asc")
	managed := filepath.Join(target, "modules", "ASCWarnings.cmake")
	if err := os.WriteFile(filepath.Join(target, "notes.txt"), []byte("unmanaged"), 0o600); err != nil {
		t.Fatal(err)
	}
	status = service.Status(ctx, "asc-cpp")
	if status.State != "current" || !reflect.DeepEqual(status.ExtraFiles, []string{"notes.txt"}) {
		t.Fatalf("extra-file status = %+v", status)
	}
	if err := os.WriteFile(managed, []byte("local edit"), 0o600); err != nil {
		t.Fatal(err)
	}
	if status := service.Status(ctx, "asc-cpp"); status.State != "locally-modified" {
		t.Fatalf("modified status = %+v", status)
	}
	if _, err := service.Plan(ctx, "asc-cpp", "", ""); err == nil {
		t.Fatal("Plan succeeded over locally modified managed file")
	}
	if err := os.WriteFile(managed, []byte("message(STATUS warnings)\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(sourceRoot, "modules", "ASCWarnings.cmake"), []byte("message(STATUS newer)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, sourceRoot, "add", "modules/ASCWarnings.cmake")
	git(t, sourceRoot, "commit", "-m", "update")
	if status := service.Status(ctx, "asc-cpp"); status.State != "source-newer" {
		t.Fatalf("newer status = %+v", status)
	}
	updated, err := service.Plan(ctx, "asc-cpp", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !containsAction(updated.Actions, "replace", "modules/ASCWarnings.cmake") {
		t.Fatalf("updated actions = %+v", updated.Actions)
	}
	if err := service.Apply(ctx, updated); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "notes.txt")); err != nil {
		t.Fatalf("unmanaged file was not preserved: %v", err)
	}

	if err := os.Remove(filepath.Join(sourceRoot, "modules", "ASCWarnings.cmake")); err != nil {
		t.Fatal(err)
	}
	git(t, sourceRoot, "add", "-A")
	git(t, sourceRoot, "commit", "-m", "remove")
	removal, err := service.Plan(ctx, "asc-cpp", "", "")
	if err != nil || !containsAction(removal.Actions, "remove", "modules/ASCWarnings.cmake") {
		t.Fatalf("removal plan = %+v, %v", removal.Actions, err)
	}
	if err := service.Apply(ctx, removal); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(managed); !os.IsNotExist(err) {
		t.Fatalf("obsolete managed file still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "asc-cpp", "cmake", "asc", "notes.txt")); err != nil {
		t.Fatal(err)
	}
}

func TestApplyRejectsDirtySourceAndDistributionSymlink(t *testing.T) {
	service, _, sourceRoot, _ := setupService(t)
	plan, err := service.Plan(context.Background(), "asc-cpp", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, "VERSION"), []byte("dirty\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := service.Apply(context.Background(), plan); err == nil || !strings.Contains(err.Error(), "plan changed") && !strings.Contains(err.Error(), "dirty") {
		t.Fatalf("dirty apply error = %v", err)
	}

	if err := os.Remove(filepath.Join(sourceRoot, "VERSION")); err != nil {
		t.Fatal(err)
	}
	git(t, sourceRoot, "restore", "VERSION")
	if err := os.Symlink(filepath.Join(sourceRoot, "LICENSE"), filepath.Join(sourceRoot, "modules", "linked.cmake")); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Plan(context.Background(), "asc-cpp", "", ""); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlink plan error = %v", err)
	}
}

func setupService(t *testing.T) (Service, string, string, string) {
	t.Helper()
	workspace := filepath.Join(t.TempDir(), "workspace with spaces")
	sourceRoot := filepath.Join(workspace, "asc-cmake")
	consumer := filepath.Join(workspace, "asc-cpp")
	for _, repository := range []string{sourceRoot, consumer} {
		if err := os.MkdirAll(repository, 0o755); err != nil {
			t.Fatal(err)
		}
		git(t, repository, "init")
		git(t, repository, "config", "user.name", "Test User")
		git(t, repository, "config", "user.email", "test@example.com")
	}
	if err := os.MkdirAll(filepath.Join(sourceRoot, "modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	for path, data := range map[string]string{
		"VERSION":                   "1.2.3\n",
		"LICENSE":                   "Apache-2.0\n",
		"modules/ASCWarnings.cmake": "message(STATUS warnings)\n",
	} {
		if err := os.WriteFile(filepath.Join(sourceRoot, filepath.FromSlash(path)), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	git(t, sourceRoot, "add", ".")
	git(t, sourceRoot, "commit", "-m", "source")
	return Service{
		Runner: process.OSRunner{},
		Config: config.Config{
			Workspace:        workspace,
			RepositoryPrefix: "asc-",
			IncludeDotGitHub: true,
			CMake: config.CMakeConfig{
				VendorDirectory:  "cmake/asc",
				SourceRepository: "asc-cmake",
			},
		},
	}, workspace, sourceRoot, consumer
}

func git(t *testing.T, directory string, arguments ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", directory}, arguments...)...)
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", arguments, err, output)
	}
}

func actionPairs(actions []Action) []string {
	result := make([]string, len(actions))
	for index, action := range actions {
		result[index] = action.Action + ":" + action.Path
	}
	return result
}

func containsAction(actions []Action, action, path string) bool {
	for _, candidate := range actions {
		if candidate.Action == action && candidate.Path == path {
			return true
		}
	}
	return false
}
