// Package config loads and validates asc configuration.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	defaultOrganization     = "AI4SciComp"
	defaultRepositoryPrefix = "asc-"
	defaultCloneProtocol    = "ssh"
	defaultRemote           = "origin"
)

var (
	organizationPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})$`)
	prefixPattern       = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	namePattern         = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

// Config is the fully resolved application configuration.
type Config struct {
	ConfigPath        string      `json:"configPath"`
	Organization      string      `json:"organization"`
	Workspace         string      `json:"workspace"`
	RepositoryPrefix  string      `json:"repositoryPrefix"`
	IncludeDotGitHub  bool        `json:"includeDotGitHub"`
	CloneProtocol     string      `json:"cloneProtocol"`
	Remote            string      `json:"remote"`
	GitHubToken       string      `json:"-"`
	GitHubTokenSource string      `json:"-"`
	CMake             CMakeConfig `json:"cmake"`
}

// CMakeConfig controls local asc-cmake vendoring. Presets remain repository-owned.
type CMakeConfig struct {
	VendorDirectory  string `json:"vendorDirectory"`
	SourceRepository string `json:"sourceRepository"`
}

type fileConfig struct {
	Organization     *string          `json:"organization"`
	Workspace        *string          `json:"workspace"`
	RepositoryPrefix *string          `json:"repositoryPrefix"`
	IncludeDotGitHub *bool            `json:"includeDotGitHub"`
	CloneProtocol    *string          `json:"cloneProtocol"`
	Remote           *string          `json:"remote"`
	CMake            *fileCMakeConfig `json:"cmake"`
}

type fileCMakeConfig struct {
	VendorDirectory  *string `json:"vendorDirectory"`
	SourceRepository *string `json:"sourceRepository"`
}

// Overrides contains configuration values explicitly supplied by the CLI.
type Overrides struct {
	ConfigPath   *string
	Organization *string
	Workspace    *string
	NoColor      bool
}

// Loader supplies environment and filesystem context for configuration loading.
type Loader struct {
	LookupEnv   func(string) (string, bool)
	UserHomeDir func() (string, error)
	Getwd       func() (string, error)
}

// NewLoader returns a loader backed by the current process environment.
func NewLoader() Loader {
	return Loader{
		LookupEnv:   os.LookupEnv,
		UserHomeDir: os.UserHomeDir,
		Getwd:       os.Getwd,
	}
}

// Load resolves configuration using CLI, environment, file, and default precedence.
func (l Loader) Load(overrides Overrides) (Config, error) {
	home, err := l.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf("resolve home directory: %w", err)
	}
	path := filepath.Join(home, ".config", "asc", "config.json")
	if value, ok := l.LookupEnv("ASC_CONFIG"); ok {
		path = value
	}
	if overrides.ConfigPath != nil {
		path = *overrides.ConfigPath
	}
	path, err = l.absolutePath(path, home)
	if err != nil {
		return Config{}, fmt.Errorf("resolve configuration path: %w", err)
	}

	fileValues, err := readFile(path)
	if err != nil {
		return Config{}, err
	}
	values := Config{
		ConfigPath:       path,
		Organization:     defaultOrganization,
		Workspace:        filepath.Join(home, "AI4SciComp"),
		RepositoryPrefix: defaultRepositoryPrefix,
		IncludeDotGitHub: true,
		CloneProtocol:    defaultCloneProtocol,
		Remote:           defaultRemote,
		CMake: CMakeConfig{
			VendorDirectory:  "cmake/asc",
			SourceRepository: "asc-cmake",
		},
	}
	applyFile(&values, fileValues)
	if err := applyEnvironment(&values, l.LookupEnv); err != nil {
		return Config{}, err
	}
	if overrides.Organization != nil {
		values.Organization = *overrides.Organization
	}
	if overrides.Workspace != nil {
		values.Workspace = *overrides.Workspace
	}
	values.Workspace, err = l.absolutePath(values.Workspace, home)
	if err != nil {
		return Config{}, fmt.Errorf("resolve workspace: %w", err)
	}
	values.GitHubToken, values.GitHubTokenSource = tokenFromEnvironment(l.LookupEnv)
	if err := Validate(values); err != nil {
		return Config{}, err
	}
	return values, nil
}

func (l Loader) absolutePath(path, home string) (string, error) {
	expanded, err := expandHome(path, home)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(expanded) {
		workingDirectory, err := l.Getwd()
		if err != nil {
			return "", err
		}
		expanded = filepath.Join(workingDirectory, expanded)
	}
	return filepath.Clean(expanded), nil
}

func expandHome(path, home string) (string, error) {
	if path == "~" {
		return home, nil
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
	}
	if strings.HasPrefix(path, "~") {
		return "", errors.New("only '~' and '~/' home expansion are supported")
	}
	return path, nil
}

func readFile(path string) (fileConfig, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return fileConfig{}, nil
	}
	if err != nil {
		return fileConfig{}, fmt.Errorf("open configuration %s: %w", path, err)
	}
	defer file.Close()
	if info, err := file.Stat(); err != nil {
		return fileConfig{}, fmt.Errorf("inspect configuration %s: %w", path, err)
	} else if info.Size() > 1<<20 {
		return fileConfig{}, fmt.Errorf("parse configuration %s: file exceeds 1 MiB limit", path)
	}
	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	decoder.DisallowUnknownFields()
	var values fileConfig
	if err := decoder.Decode(&values); err != nil {
		return fileConfig{}, fmt.Errorf("parse configuration %s: %w", path, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return fileConfig{}, fmt.Errorf("parse configuration %s: %w", path, err)
	}
	return values, nil
}

func applyFile(config *Config, values fileConfig) {
	if values.Organization != nil {
		config.Organization = *values.Organization
	}
	if values.Workspace != nil {
		config.Workspace = *values.Workspace
	}
	if values.RepositoryPrefix != nil {
		config.RepositoryPrefix = *values.RepositoryPrefix
	}
	if values.IncludeDotGitHub != nil {
		config.IncludeDotGitHub = *values.IncludeDotGitHub
	}
	if values.CloneProtocol != nil {
		config.CloneProtocol = *values.CloneProtocol
	}
	if values.Remote != nil {
		config.Remote = *values.Remote
	}
	if values.CMake != nil {
		if values.CMake.VendorDirectory != nil {
			config.CMake.VendorDirectory = *values.CMake.VendorDirectory
		}
		if values.CMake.SourceRepository != nil {
			config.CMake.SourceRepository = *values.CMake.SourceRepository
		}
	}
}

func applyEnvironment(config *Config, lookup func(string) (string, bool)) error {
	setString := func(environmentName string, target *string) {
		if value, ok := lookup(environmentName); ok {
			*target = value
		}
	}
	setString("ASC_ORGANIZATION", &config.Organization)
	setString("ASC_WORKSPACE", &config.Workspace)
	setString("ASC_REPOSITORY_PREFIX", &config.RepositoryPrefix)
	setString("ASC_CLONE_PROTOCOL", &config.CloneProtocol)
	setString("ASC_REMOTE", &config.Remote)
	if value, ok := lookup("ASC_INCLUDE_DOT_GITHUB"); ok {
		switch value {
		case "true":
			config.IncludeDotGitHub = true
		case "false":
			config.IncludeDotGitHub = false
		default:
			return errors.New("ASC_INCLUDE_DOT_GITHUB must be 'true' or 'false'")
		}
	}
	return nil
}

func tokenFromEnvironment(lookup func(string) (string, bool)) (string, string) {
	for _, name := range []string{"ASC_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"} {
		if value, ok := lookup(name); ok && value != "" {
			return value, name
		}
	}
	return "", ""
}

// Validate checks configuration values and workspace safety.
func Validate(config Config) error {
	if !organizationPattern.MatchString(config.Organization) {
		return errors.New("organization must be a valid GitHub organization name")
	}
	if config.RepositoryPrefix == "" || !prefixPattern.MatchString(config.RepositoryPrefix) {
		return errors.New("repository prefix contains invalid characters")
	}
	if config.CloneProtocol != "ssh" && config.CloneProtocol != "https" {
		return errors.New("clone protocol must be 'ssh' or 'https'")
	}
	if config.Remote == "" || !namePattern.MatchString(config.Remote) || strings.HasPrefix(config.Remote, "-") {
		return errors.New("remote must be a simple Git remote name")
	}
	if err := validateRelativePath(config.CMake.VendorDirectory, "CMake vendor directory"); err != nil {
		return err
	}
	if config.CMake.SourceRepository == "" || config.CMake.SourceRepository == "." || config.CMake.SourceRepository == ".." || !namePattern.MatchString(config.CMake.SourceRepository) || strings.HasPrefix(config.CMake.SourceRepository, "-") {
		return errors.New("CMake source repository must be a simple repository name")
	}
	volume := filepath.VolumeName(config.Workspace)
	if filepath.Clean(config.Workspace) == volume+string(filepath.Separator) {
		return errors.New("workspace must not be a filesystem root")
	}
	return nil
}

func validateRelativePath(path, label string) error {
	if path == "" || filepath.IsAbs(path) || filepath.Clean(path) != path || path == "." || path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s must be a clean relative path", label)
	}
	return nil
}

// ValidateRepositoryName verifies a repository is a safe managed name.
func ValidateRepositoryName(name string, config Config) error {
	if name == "" || name == "." || name == ".." || strings.HasPrefix(name, "-") || !namePattern.MatchString(name) {
		return fmt.Errorf("invalid repository name %q", name)
	}
	if !strings.HasPrefix(name, config.RepositoryPrefix) && !(config.IncludeDotGitHub && name == ".github") {
		return fmt.Errorf("repository is not managed by asc: %s", name)
	}
	return nil
}
