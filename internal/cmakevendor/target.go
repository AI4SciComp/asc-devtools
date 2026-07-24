package cmakevendor

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

func readManifest(target string) (Manifest, bool, error) {
	file, err := os.Open(filepath.Join(target, manifestName))
	if errors.Is(err, os.ErrNotExist) {
		return Manifest{}, false, nil
	}
	if err != nil {
		return Manifest{}, false, fmt.Errorf("open vendor manifest: %w", err)
	}
	defer file.Close()
	manifest, err := DecodeManifest(file)
	if err != nil {
		return Manifest{}, false, err
	}
	return manifest, true, nil
}

func inspectTarget(target string, manifest Manifest) ([]string, bool, error) {
	managed := make(map[string]bool, len(manifest.Files))
	modified := false
	for _, record := range manifest.Files {
		managed[record.Path] = true
		path := filepath.Join(target, filepath.FromSlash(record.Path))
		if err := ensureContained(target, path); err != nil {
			return nil, false, err
		}
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			modified = true
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, false, fmt.Errorf("read managed vendor file %s: %w", record.Path, err)
		}
		if hash(data) != record.SHA256 {
			modified = true
		}
	}
	var extras []string
	if _, err := os.Stat(target); errors.Is(err, os.ErrNotExist) {
		return nil, modified, nil
	}
	err := filepath.WalkDir(target, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == target || entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(target, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if relative != manifestName && !managed[relative] {
			extras = append(extras, relative)
		}
		return nil
	})
	if err != nil {
		return nil, false, fmt.Errorf("inspect vendor directory: %w", err)
	}
	sort.Strings(extras)
	return extras, modified, nil
}

func samePlan(left, right Plan) bool {
	if left.Repository != right.Repository || left.Commit != right.Commit || left.Version != right.Version || len(left.Actions) != len(right.Actions) {
		return false
	}
	for index := range left.Actions {
		if left.Actions[index] != right.Actions[index] {
			return false
		}
	}
	return true
}

func applyPlan(plan Plan) (returnErr error) {
	if err := rejectSymlinkComponents(filepath.Dir(filepath.Dir(plan.target)), plan.target); err != nil {
		return err
	}
	parent := filepath.Dir(plan.target)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create vendor parent: %w", err)
	}
	stage, err := os.MkdirTemp(parent, ".asc-vendor-stage-")
	if err != nil {
		return fmt.Errorf("create vendor staging directory: %w", err)
	}
	defer os.RemoveAll(stage)
	backup, err := os.MkdirTemp(parent, ".asc-vendor-backup-")
	if err != nil {
		return fmt.Errorf("create vendor rollback directory: %w", err)
	}
	defer os.RemoveAll(backup)
	for path, data := range plan.files {
		staged := filepath.Join(stage, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(staged), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(staged, data, 0o644); err != nil {
			return fmt.Errorf("stage vendor file %s: %w", path, err)
		}
	}
	manifestData, err := EncodeManifest(plan.Manifest)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(stage, manifestName), manifestData, 0o644); err != nil {
		return fmt.Errorf("stage vendor manifest: %w", err)
	}
	if err := os.MkdirAll(plan.target, 0o755); err != nil {
		return fmt.Errorf("create vendor directory: %w", err)
	}
	type moved struct {
		destination, backup string
		installed           bool
	}
	var changes []moved
	rollback := func() {
		for index := len(changes) - 1; index >= 0; index-- {
			change := changes[index]
			if change.installed {
				_ = os.Remove(change.destination)
			}
			if change.backup != "" {
				_ = os.MkdirAll(filepath.Dir(change.destination), 0o755)
				_ = os.Rename(change.backup, change.destination)
			}
		}
	}
	defer func() {
		if returnErr != nil {
			rollback()
		}
	}()
	for _, action := range plan.Actions {
		if action.Action == "preserve" {
			continue
		}
		destination := filepath.Join(plan.target, filepath.FromSlash(action.Path))
		if err := ensureContained(plan.target, destination); err != nil {
			return err
		}
		change := moved{destination: destination}
		if _, err := os.Lstat(destination); err == nil {
			change.backup = filepath.Join(backup, filepath.FromSlash(action.Path))
			if err := os.MkdirAll(filepath.Dir(change.backup), 0o755); err != nil {
				return err
			}
			if err := os.Rename(destination, change.backup); err != nil {
				return fmt.Errorf("back up vendor file %s: %w", action.Path, err)
			}
		}
		changes = append(changes, change)
		if action.Action == "remove" {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		if err := os.Rename(filepath.Join(stage, filepath.FromSlash(action.Path)), destination); err != nil {
			return fmt.Errorf("install vendor file %s: %w", action.Path, err)
		}
		changes[len(changes)-1].installed = true
	}
	manifestDestination := filepath.Join(plan.target, manifestName)
	manifestChange := moved{destination: manifestDestination}
	if _, err := os.Lstat(manifestDestination); err == nil {
		manifestChange.backup = filepath.Join(backup, manifestName)
		if err := os.Rename(manifestDestination, manifestChange.backup); err != nil {
			return fmt.Errorf("back up vendor manifest: %w", err)
		}
	}
	changes = append(changes, manifestChange)
	if err := os.Rename(filepath.Join(stage, manifestName), manifestDestination); err != nil {
		return fmt.Errorf("install vendor manifest: %w", err)
	}
	changes[len(changes)-1].installed = true
	return nil
}

func ensureContained(root, path string) error {
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || filepath.IsAbs(relative) || (len(relative) > 3 && relative[:3] == ".."+string(filepath.Separator)) {
		return fmt.Errorf("path escapes allowed root: %s", path)
	}
	return nil
}

func rejectSymlinkComponents(root, path string) error {
	if err := ensureContained(root, path); err != nil {
		return err
	}
	current := root
	relative, _ := filepath.Rel(root, path)
	for _, part := range splitPath(relative) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symlinked vendor path: %s", current)
		}
	}
	return nil
}

func splitPath(path string) []string {
	var parts []string
	for path != "." && path != "" {
		directory, file := filepath.Split(path)
		if file != "" {
			parts = append([]string{file}, parts...)
		}
		path = filepath.Clean(directory)
	}
	return parts
}
