package cmakevendor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/AI4SciComp/asc-devtools/internal/config"
	"github.com/AI4SciComp/asc-devtools/internal/process"
	"github.com/AI4SciComp/asc-devtools/internal/workspace"
)

// Service validates sources and consumer vendoring state.
type Service struct {
	Runner process.Runner
	Config config.Config
}

// Status is the stable result of a read-only status query.
type Status struct {
	Repository string   `json:"repository"`
	State      string   `json:"state"`
	Version    string   `json:"version,omitempty"`
	Commit     string   `json:"commit,omitempty"`
	ExtraFiles []string `json:"extraFiles,omitempty"`
	Detail     string   `json:"detail,omitempty"`
}

// Action is one deterministic plan entry.
type Action struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}

// Plan describes an exact source-to-consumer update.
type Plan struct {
	Repository string   `json:"repository"`
	Source     string   `json:"source"`
	Version    string   `json:"version"`
	Commit     string   `json:"commit"`
	Actions    []Action `json:"actions"`
	Manifest   Manifest `json:"-"`
	files      map[string][]byte
	target     string
	sourceRoot string
}

type source struct {
	root     string
	version  string
	commit   string
	dirty    bool
	files    map[string][]byte
	manifest Manifest
}

// Status reports consumer state without writing or fetching.
func (s Service) Status(ctx context.Context, repositoryName string) Status {
	target, err := s.target(ctx, repositoryName)
	if err != nil {
		return Status{Repository: repositoryName, State: "manifest-invalid", Detail: err.Error()}
	}
	manifest, exists, err := readManifest(target)
	if err != nil {
		return Status{Repository: repositoryName, State: "manifest-invalid", Detail: err.Error()}
	}
	if !exists {
		return Status{Repository: repositoryName, State: "not-vendored"}
	}
	extras, modified, err := inspectTarget(target, manifest)
	if err != nil {
		return Status{Repository: repositoryName, State: "manifest-invalid", Detail: err.Error()}
	}
	if modified {
		return Status{Repository: repositoryName, State: "locally-modified", Version: manifest.Version, Commit: manifest.Commit, ExtraFiles: extras, Detail: "managed files differ from the manifest"}
	}
	source, err := s.loadSource(ctx, "", "", true)
	if err != nil {
		return Status{Repository: repositoryName, State: "source-unavailable", Version: manifest.Version, Commit: manifest.Commit, ExtraFiles: extras, Detail: err.Error()}
	}
	state := "source-newer"
	if sameManifestContent(manifest, source.manifest) {
		state = "current"
	}
	detail := ""
	if source.dirty {
		detail = "source checkout is dirty"
	}
	return Status{Repository: repositoryName, State: state, Version: manifest.Version, Commit: manifest.Commit, ExtraFiles: extras, Detail: detail}
}

// Plan computes a read-only update plan.
func (s Service) Plan(ctx context.Context, repositoryName, sourcePath, ref string) (Plan, error) {
	target, err := s.target(ctx, repositoryName)
	if err != nil {
		return Plan{}, err
	}
	source, err := s.loadSource(ctx, sourcePath, ref, true)
	if err != nil {
		return Plan{}, err
	}
	old, exists, err := readManifest(target)
	if err != nil {
		return Plan{}, err
	}
	if exists {
		_, modified, err := inspectTarget(target, old)
		if err != nil {
			return Plan{}, err
		}
		if modified {
			return Plan{}, errors.New("locally modified managed vendored files must be resolved before planning")
		}
	}
	oldRecords := make(map[string]FileRecord)
	for _, record := range old.Files {
		oldRecords[record.Path] = record
	}
	var actions []Action
	for _, record := range source.manifest.Files {
		action := "add"
		if previous, ok := oldRecords[record.Path]; ok {
			if previous.SHA256 == record.SHA256 {
				action = "preserve"
			} else {
				action = "replace"
			}
			delete(oldRecords, record.Path)
		}
		actions = append(actions, Action{Path: record.Path, Action: action})
	}
	for path := range oldRecords {
		actions = append(actions, Action{Path: path, Action: "remove"})
	}
	sort.Slice(actions, func(i, j int) bool { return actions[i].Path < actions[j].Path })
	return Plan{Repository: repositoryName, Source: source.root, Version: source.version, Commit: source.commit, Actions: actions, Manifest: source.manifest, files: source.files, target: target, sourceRoot: source.root}, nil
}

// Apply revalidates and applies an already approved plan conservatively.
func (s Service) Apply(ctx context.Context, plan Plan) error {
	current, err := s.Plan(ctx, plan.Repository, plan.sourceRoot, plan.Commit)
	if err != nil {
		return fmt.Errorf("revalidate vendor plan: %w", err)
	}
	if !samePlan(plan, current) {
		return errors.New("vendor plan changed during confirmation; run plan again")
	}
	source, err := s.loadSource(ctx, plan.sourceRoot, plan.Commit, false)
	if err != nil {
		return err
	}
	plan.files, plan.Manifest = source.files, source.manifest
	return applyPlan(plan)
}

func (s Service) target(ctx context.Context, repositoryName string) (string, error) {
	if err := config.ValidateRepositoryName(repositoryName, s.Config); err != nil {
		return "", err
	}
	repositoryPath, err := workspace.DirectChild(s.Config.Workspace, repositoryName)
	if err != nil {
		return "", err
	}
	if !workspace.IsGitWorktree(ctx, s.Runner, repositoryPath) {
		return "", fmt.Errorf("local Git working tree not found: %s", repositoryName)
	}
	target := filepath.Join(repositoryPath, filepath.FromSlash(s.Config.CMake.VendorDirectory))
	if err := ensureContained(repositoryPath, target); err != nil {
		return "", err
	}
	if err := rejectSymlinkComponents(repositoryPath, target); err != nil {
		return "", err
	}
	return target, nil
}

func (s Service) loadSource(ctx context.Context, sourcePath, ref string, allowDirty bool) (source, error) {
	root := sourcePath
	if root == "" {
		root = filepath.Join(s.Config.Workspace, s.Config.CMake.SourceRepository)
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return source{}, fmt.Errorf("resolve asc-cmake source: %w", err)
	}
	root = filepath.Clean(absolute)
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return source{}, fmt.Errorf("asc-cmake source is not a regular directory: %s", root)
	}
	runGit := func(arguments ...string) (string, error) {
		result, err := s.Runner.Run(ctx, process.Command{Name: "git", Args: append([]string{"-C", root}, arguments...)})
		return strings.TrimSpace(result.Stdout), err
	}
	inside, err := runGit("rev-parse", "--is-inside-work-tree")
	if err != nil || inside != "true" {
		return source{}, fmt.Errorf("asc-cmake source is not a Git working tree: %s", root)
	}
	commit, err := runGit("rev-parse", "HEAD")
	if err != nil || commit == "" {
		return source{}, fmt.Errorf("resolve asc-cmake source commit: %w", err)
	}
	if ref != "" {
		resolved, err := runGit("rev-parse", "--verify", ref+"^{commit}")
		if err != nil || resolved != commit {
			return source{}, fmt.Errorf("requested ref %q does not match checked-out source commit", ref)
		}
	}
	if origin, err := runGit("remote", "get-url", "origin"); err == nil && origin != "" && !isASCCMakeOrigin(origin) {
		return source{}, fmt.Errorf("asc-cmake source origin is not AI4SciComp/asc-cmake: %s", origin)
	}
	status, err := runGit("status", "--porcelain", "--untracked-files=normal")
	if err != nil {
		return source{}, fmt.Errorf("inspect asc-cmake source: %w", err)
	}
	dirty := status != ""
	if dirty && !allowDirty {
		return source{}, errors.New("refusing to apply from a dirty asc-cmake source")
	}
	files, err := distributionFiles(root)
	if err != nil {
		return source{}, err
	}
	version, err := sourceVersion(ctx, s.Runner, root)
	if err != nil {
		return source{}, err
	}
	manifest := Manifest{SchemaVersion: schemaVersion, SourceRepository: "AI4SciComp/asc-cmake", Version: version, Commit: commit, Files: make([]FileRecord, 0, len(files))}
	for path, data := range files {
		manifest.Files = append(manifest.Files, FileRecord{Path: path, SHA256: hash(data)})
	}
	sort.Slice(manifest.Files, func(i, j int) bool { return manifest.Files[i].Path < manifest.Files[j].Path })
	return source{root: root, version: version, commit: commit, dirty: dirty, files: files, manifest: manifest}, nil
}

type distributionContract struct {
	Files []string `json:"files"`
}

func distributionFiles(root string) (map[string][]byte, error) {
	contractPath := filepath.Join(root, "distribution.json")
	var paths []string
	if data, err := os.ReadFile(contractPath); err == nil {
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		decoder.DisallowUnknownFields()
		var contract distributionContract
		if err := decoder.Decode(&contract); err != nil {
			return nil, fmt.Errorf("parse distribution.json: %w", err)
		}
		paths = append(paths, contract.Files...)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read distribution.json: %w", err)
	} else {
		if info, err := os.Lstat(filepath.Join(root, "LICENSE")); err == nil && info.Mode().IsRegular() {
			paths = append(paths, "LICENSE")
		}
		modules := filepath.Join(root, "modules")
		walkErr := filepath.WalkDir(modules, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("distribution contains symlink: %s", path)
			}
			if entry.Type().IsRegular() && strings.EqualFold(filepath.Ext(path), ".cmake") {
				relative, _ := filepath.Rel(root, path)
				paths = append(paths, filepath.ToSlash(relative))
			}
			return nil
		})
		if walkErr != nil && !errors.Is(walkErr, os.ErrNotExist) {
			return nil, fmt.Errorf("inspect asc-cmake modules: %w", walkErr)
		}
	}
	if len(paths) == 0 {
		return nil, errors.New("asc-cmake distribution contains no managed files")
	}
	files := make(map[string][]byte, len(paths))
	for _, relative := range paths {
		if err := validateManagedPath(relative); err != nil {
			return nil, fmt.Errorf("distribution path %q: %w", relative, err)
		}
		if _, duplicate := files[relative]; duplicate {
			return nil, fmt.Errorf("distribution contains duplicate path %q", relative)
		}
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := ensureContained(root, path); err != nil {
			return nil, err
		}
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("distribution file is not a regular nonsymlink file: %s", relative)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read distribution file %s: %w", relative, err)
		}
		files[relative] = data
	}
	return files, nil
}

var projectVersionPattern = regexp.MustCompile(`(?im)project\s*\([^)]*\bVERSION\s+([0-9]+(?:\.[0-9]+){1,3}(?:[-+][A-Za-z0-9.-]+)?)`)

func sourceVersion(ctx context.Context, runner process.Runner, root string) (string, error) {
	if data, err := os.ReadFile(filepath.Join(root, "VERSION")); err == nil && strings.TrimSpace(string(data)) != "" {
		return strings.TrimSpace(string(data)), nil
	}
	if data, err := os.ReadFile(filepath.Join(root, "CMakeLists.txt")); err == nil {
		if match := projectVersionPattern.FindStringSubmatch(string(data)); len(match) == 2 {
			return match[1], nil
		}
	}
	result, err := runner.Run(ctx, process.Command{Name: "git", Args: []string{"-C", root, "describe", "--tags", "--exact-match", "HEAD"}})
	if err == nil && strings.TrimSpace(result.Stdout) != "" {
		return strings.TrimPrefix(strings.TrimSpace(result.Stdout), "v"), nil
	}
	return "", errors.New("asc-cmake version not found in VERSION, CMake project(), or exact tag")
}

func validateManagedPath(path string) error {
	if path == "" || filepath.IsAbs(path) || filepath.ToSlash(filepath.Clean(filepath.FromSlash(path))) != path || path == "." || path == ".." || strings.HasPrefix(path, "../") || strings.Contains(path, "\\") || path == manifestName {
		return fmt.Errorf("unsafe managed path %q", path)
	}
	return nil
}

func isASCCMakeOrigin(origin string) bool {
	trimmed := strings.TrimSuffix(strings.TrimSpace(origin), ".git")
	return strings.HasSuffix(trimmed, "github.com/AI4SciComp/asc-cmake") || strings.HasSuffix(trimmed, "github.com:AI4SciComp/asc-cmake")
}
