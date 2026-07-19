package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testLoader(t *testing.T, environment map[string]string) Loader {
	t.Helper()
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	return Loader{
		LookupEnv: func(name string) (string, bool) {
			value, ok := environment[name]
			return value, ok
		},
		UserHomeDir: func() (string, error) { return home, nil },
		Getwd:       func() (string, error) { return filepath.Dir(home), nil },
	}
}

func TestLoadDefaults(t *testing.T) {
	loader := testLoader(t, map[string]string{})
	config, err := loader.Load(Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if config.Organization != "AI4SciComp" || config.CloneProtocol != "ssh" || !config.IncludeDotGitHub {
		t.Fatalf("unexpected defaults: %+v", config)
	}
	if config.CMake.VendorDirectory != "cmake/asc" || config.CMake.SourceRepository != "asc-cmake" {
		t.Fatalf("unexpected CMake defaults: %+v", config.CMake)
	}
	if !filepath.IsAbs(config.Workspace) || !strings.HasSuffix(config.Workspace, filepath.Join("projects", "AI4SciComp")) {
		t.Fatalf("unexpected workspace: %s", config.Workspace)
	}
}

func TestLoadPrecedence(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "config.json")
	content := `{"organization":"FileOrg","workspace":"~/file","cloneProtocol":"https"}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	loader := testLoader(t, map[string]string{
		"ASC_CONFIG":       path,
		"ASC_ORGANIZATION": "EnvironmentOrg",
		"ASC_WORKSPACE":    filepath.Join(directory, "environment"),
	})
	organization := "CommandOrg"
	workspace := filepath.Join(directory, "command")
	config, err := loader.Load(Overrides{Organization: &organization, Workspace: &workspace})
	if err != nil {
		t.Fatal(err)
	}
	if config.Organization != organization || config.Workspace != workspace || config.CloneProtocol != "https" {
		t.Fatalf("precedence mismatch: %+v", config)
	}
}

func TestLoadRejectsMalformedAndUnknownJSON(t *testing.T) {
	for _, content := range []string{`{"organization":`, `{"unknown":true}`} {
		t.Run(content, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			loader := testLoader(t, map[string]string{"ASC_CONFIG": path})
			_, err := loader.Load(Overrides{})
			if err == nil || !strings.Contains(err.Error(), path) {
				t.Fatalf("Load() error = %v, want file-specific parse error", err)
			}
		})
	}
}

func TestLoadTokenPrecedence(t *testing.T) {
	loader := testLoader(t, map[string]string{
		"ASC_GITHUB_TOKEN": "asc-token",
		"GH_TOKEN":         "gh-token",
		"GITHUB_TOKEN":     "github-token",
	})
	config, err := loader.Load(Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if config.GitHubToken != "asc-token" || config.GitHubTokenSource != "ASC_GITHUB_TOKEN" {
		t.Fatalf("unexpected token source: %s", config.GitHubTokenSource)
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	tests := []map[string]string{
		{"ASC_INCLUDE_DOT_GITHUB": "yes"},
		{"ASC_CLONE_PROTOCOL": "ftp"},
		{"ASC_WORKSPACE": "/"},
		{"ASC_WORKSPACE": "~another/source"},
	}
	for _, environment := range tests {
		loader := testLoader(t, environment)
		if _, err := loader.Load(Overrides{}); err == nil {
			t.Fatalf("Load() succeeded for invalid environment: %v", environment)
		}
	}
}

func TestLoadCMakeVendorConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"cmake":{"vendorDirectory":"vendor/asc","sourceRepository":"asc-cmake-local"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	loader := testLoader(t, map[string]string{"ASC_CONFIG": path})
	loaded, err := loader.Load(Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if loaded.CMake.VendorDirectory != "vendor/asc" || loaded.CMake.SourceRepository != "asc-cmake-local" {
		t.Fatalf("CMake config = %+v", loaded.CMake)
	}
	for _, content := range []string{
		`{"cmake":{"vendorDirectory":"../escape"}}`,
		`{"cmake":{"unknown":true}}`,
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := loader.Load(Overrides{}); err == nil {
			t.Fatalf("Load accepted %s", content)
		}
	}
}

func TestValidateRepositoryName(t *testing.T) {
	config := Config{RepositoryPrefix: "asc-", IncludeDotGitHub: true}
	valid := []string{"asc-cpp", "asc-name_with_parts", ".github"}
	invalid := []string{"", ".", "..", "-option", "../asc-cpp", "asc/cpp", `asc\cpp`, "other", "asc bad", "https://example"}
	for _, name := range valid {
		if err := ValidateRepositoryName(name, config); err != nil {
			t.Errorf("ValidateRepositoryName(%q) unexpected error: %v", name, err)
		}
	}
	for _, name := range invalid {
		if err := ValidateRepositoryName(name, config); err == nil {
			t.Errorf("ValidateRepositoryName(%q) succeeded", name)
		}
	}
}
