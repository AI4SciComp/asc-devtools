// Package cmakevendor manages pinned, local-only asc-cmake distributions.
package cmakevendor

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

const (
	manifestName  = "ASC_CMAKE_MANIFEST.json"
	schemaVersion = 1
)

// FileRecord identifies one managed file and its exact content hash.
type FileRecord struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// Manifest records a deterministic asc-cmake distribution.
type Manifest struct {
	SchemaVersion    int          `json:"schemaVersion"`
	SourceRepository string       `json:"sourceRepository"`
	Version          string       `json:"version"`
	Commit           string       `json:"commit"`
	Files            []FileRecord `json:"files"`
}

// EncodeManifest returns stable, indented JSON. A timestamp is intentionally omitted.
func EncodeManifest(manifest Manifest) ([]byte, error) {
	manifest.Files = append([]FileRecord(nil), manifest.Files...)
	sort.Slice(manifest.Files, func(i, j int) bool { return manifest.Files[i].Path < manifest.Files[j].Path })
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode vendor manifest: %w", err)
	}
	return append(data, '\n'), nil
}

// DecodeManifest strictly validates a schema-v1 manifest.
func DecodeManifest(reader io.Reader) (Manifest, error) {
	decoder := json.NewDecoder(io.LimitReader(reader, 4<<20))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode vendor manifest: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Manifest{}, fmt.Errorf("decode vendor manifest: trailing JSON data")
	}
	if manifest.SchemaVersion != schemaVersion {
		return Manifest{}, fmt.Errorf("unsupported vendor manifest schema %d", manifest.SchemaVersion)
	}
	if manifest.SourceRepository != "AI4SciComp/asc-cmake" || manifest.Version == "" || manifest.Commit == "" {
		return Manifest{}, fmt.Errorf("vendor manifest source, version, and commit are required")
	}
	seen := make(map[string]bool)
	for index, file := range manifest.Files {
		if err := validateManagedPath(file.Path); err != nil {
			return Manifest{}, fmt.Errorf("vendor manifest file %d: %w", index, err)
		}
		if seen[file.Path] {
			return Manifest{}, fmt.Errorf("vendor manifest contains duplicate path %q", file.Path)
		}
		seen[file.Path] = true
		decoded, err := hex.DecodeString(file.SHA256)
		if err != nil || len(decoded) != sha256.Size {
			return Manifest{}, fmt.Errorf("vendor manifest has invalid SHA-256 for %s", file.Path)
		}
	}
	return manifest, nil
}

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func sameManifestContent(left, right Manifest) bool {
	leftData, leftErr := EncodeManifest(left)
	rightData, rightErr := EncodeManifest(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftData, rightData)
}
