// Package selfupdate installs a verified asc release over a managed installation.
package selfupdate

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL    = "https://api.github.com"
	maxMetadataSize   = 4 << 20
	maxChecksumSize   = 1 << 20
	maxArchiveSize    = 128 << 20
	maxExtractedSize  = 256 << 20
	releaseRepository = "AI4SciComp/asc-devtools"
)

// Options controls one update check or installation.
type Options struct {
	CurrentVersion string
	Prefix         string
	CheckOnly      bool
	AssumeYes      bool
	Token          string
	BaseURL        string
	HTTPClient     *http.Client
	Stdin          io.Reader
	Stdout         io.Writer
	Stderr         io.Writer
	Executable     func() (string, error)
	RunCommand     func(context.Context, string, ...string) error
	BundleVersion  func(context.Context, string) (string, error)
	GOOS           string
	GOARCH         string
}

type release struct {
	TagName string  `json:"tag_name"`
	Assets  []asset `json:"assets"`
}

type asset struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Run checks the latest release and optionally replaces a managed installation.
func Run(ctx context.Context, options Options) error {
	options = defaults(options)
	latest, err := fetchRelease(ctx, options)
	if err != nil {
		return err
	}
	latestVersion, err := releaseVersion(latest.TagName)
	if err != nil {
		return err
	}
	comparison := compareVersions(options.CurrentVersion, latestVersion)
	if comparison == 0 {
		fmt.Fprintf(options.Stdout, "asc %s is up to date.\n", displayVersion(options.CurrentVersion))
		return nil
	}
	if comparison > 0 {
		fmt.Fprintf(options.Stdout, "asc %s is newer than the latest release (%s); no update performed.\n", displayVersion(options.CurrentVersion), latestVersion)
		return nil
	}
	archiveName := fmt.Sprintf("asc-devtools-go-%s-%s.tar.gz", options.GOOS, options.GOARCH)
	archiveAsset, ok := findAsset(latest.Assets, archiveName)
	if !ok {
		return fmt.Errorf("release %s does not contain %s", latest.TagName, archiveName)
	}
	checksumAsset, ok := findAsset(latest.Assets, "SHA256SUMS")
	if !ok {
		return fmt.Errorf("release %s does not contain SHA256SUMS", latest.TagName)
	}
	fmt.Fprintf(options.Stdout, "asc %s is available (current: %s).\n", latestVersion, displayVersion(options.CurrentVersion))
	if options.CheckOnly {
		return nil
	}
	prefix, err := resolvePrefix(options)
	if err != nil {
		return err
	}
	if err := validateManagedInstall(prefix); err != nil {
		return err
	}
	if !options.AssumeYes {
		fmt.Fprintf(options.Stdout, "Update the managed installation in %s? [y/N] ", prefix)
		answer, readErr := bufio.NewReader(options.Stdin).ReadString('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return fmt.Errorf("read confirmation: %w", readErr)
		}
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer != "y" && answer != "yes" {
			fmt.Fprintln(options.Stdout, "Update cancelled.")
			return nil
		}
	}
	temporary, err := os.MkdirTemp("", "asc-update-")
	if err != nil {
		return fmt.Errorf("create update directory: %w", err)
	}
	defer os.RemoveAll(temporary)
	checksums, err := download(ctx, options, checksumAsset.URL, maxChecksumSize)
	if err != nil {
		return fmt.Errorf("download SHA256SUMS: %w", err)
	}
	expected, err := checksumFor(checksums, archiveName)
	if err != nil {
		return err
	}
	archive, err := download(ctx, options, archiveAsset.URL, maxArchiveSize)
	if err != nil {
		return fmt.Errorf("download %s: %w", archiveName, err)
	}
	actual := sha256.Sum256(archive)
	if hex.EncodeToString(actual[:]) != expected {
		return fmt.Errorf("checksum mismatch for %s", archiveName)
	}
	bundle := filepath.Join(temporary, "bundle")
	if err := extractArchive(archive, bundle); err != nil {
		return fmt.Errorf("extract %s: %w", archiveName, err)
	}
	root := filepath.Join(bundle, "asc-devtools-go")
	binary := filepath.Join(root, "asc")
	installer := filepath.Join(root, "scripts", "install.sh")
	if err := requireRegular(binary, true); err != nil {
		return err
	}
	if err := requireRegular(installer, false); err != nil {
		return err
	}
	bundleVersion, err := options.BundleVersion(ctx, binary)
	if err != nil {
		return fmt.Errorf("inspect release binary: %w", err)
	}
	if strings.TrimPrefix(bundleVersion, "v") != latestVersion {
		return fmt.Errorf("release binary version %q does not match release %s", bundleVersion, latestVersion)
	}
	if err := options.RunCommand(ctx, "bash", installer, "--binary", binary, "--prefix", prefix); err != nil {
		return fmt.Errorf("install asc %s: %w", latestVersion, err)
	}
	fmt.Fprintf(options.Stdout, "Updated asc to %s.\n", latestVersion)
	return nil
}

func defaults(options Options) Options {
	if options.BaseURL == "" {
		options.BaseURL = defaultBaseURL
	}
	if options.HTTPClient == nil {
		options.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	if options.Stdin == nil {
		options.Stdin = os.Stdin
	}
	if options.Stdout == nil {
		options.Stdout = os.Stdout
	}
	if options.Stderr == nil {
		options.Stderr = os.Stderr
	}
	if options.Executable == nil {
		options.Executable = os.Executable
	}
	if options.GOOS == "" {
		options.GOOS = runtime.GOOS
	}
	if options.GOARCH == "" {
		options.GOARCH = runtime.GOARCH
	}
	if options.RunCommand == nil {
		options.RunCommand = func(ctx context.Context, name string, arguments ...string) error {
			command := exec.CommandContext(ctx, name, arguments...)
			command.Stdin = options.Stdin
			command.Stdout = options.Stdout
			command.Stderr = options.Stderr
			return command.Run()
		}
	}
	if options.BundleVersion == nil {
		options.BundleVersion = func(ctx context.Context, binary string) (string, error) {
			output, err := exec.CommandContext(ctx, binary, "--version").Output()
			if err != nil {
				return "", err
			}
			fields := strings.Fields(string(output))
			if len(fields) < 2 || fields[0] != "asc" {
				return "", fmt.Errorf("unexpected version output %q", strings.TrimSpace(string(output)))
			}
			return fields[1], nil
		}
	}
	return options
}

func fetchRelease(ctx context.Context, options Options) (release, error) {
	endpoint := strings.TrimRight(options.BaseURL, "/") + "/repos/" + releaseRepository + "/releases/latest"
	body, status, err := request(ctx, options, endpoint, "application/vnd.github+json", maxMetadataSize)
	if err != nil {
		return release{}, fmt.Errorf("check latest release: %w", err)
	}
	if status == http.StatusNotFound {
		return release{}, errors.New("no published asc release is available")
	}
	if status < 200 || status >= 300 {
		return release{}, fmt.Errorf("check latest release: GitHub API returned %d", status)
	}
	var result release
	if err := json.Unmarshal(body, &result); err != nil {
		return release{}, fmt.Errorf("decode latest release: %w", err)
	}
	return result, nil
}

func download(ctx context.Context, options Options, endpoint string, limit int64) ([]byte, error) {
	body, status, err := request(ctx, options, endpoint, "application/octet-stream", limit)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("GitHub returned %d", status)
	}
	return body, nil
}

func request(ctx context.Context, options Options, endpoint, accept string, limit int64) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "asc-devtools/"+displayVersion(options.CurrentVersion))
	if options.Token != "" {
		req.Header.Set("Authorization", "Bearer "+options.Token)
	}
	response, err := options.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, response.StatusCode, err
	}
	if int64(len(body)) > limit {
		return nil, response.StatusCode, fmt.Errorf("response exceeded %d bytes", limit)
	}
	return body, response.StatusCode, nil
}

func findAsset(assets []asset, name string) (asset, bool) {
	for _, candidate := range assets {
		if candidate.Name == name && candidate.URL != "" {
			return candidate, true
		}
	}
	return asset{}, false
}

func checksumFor(body []byte, filename string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != filename {
			continue
		}
		hash := strings.ToLower(fields[0])
		if len(hash) != 64 {
			break
		}
		if _, err := hex.DecodeString(hash); err != nil {
			break
		}
		return hash, nil
	}
	return "", fmt.Errorf("SHA256SUMS has no valid entry for %s", filename)
}

func releaseVersion(tag string) (string, error) {
	version := strings.TrimPrefix(strings.TrimSpace(tag), "v")
	if _, ok := numericVersion(version); !ok {
		return "", fmt.Errorf("latest release has unsupported tag %q", tag)
	}
	return version, nil
}

func compareVersions(current, latest string) int {
	currentParts, currentOK := numericVersion(strings.TrimPrefix(current, "v"))
	latestParts, latestOK := numericVersion(strings.TrimPrefix(latest, "v"))
	if !latestOK || !currentOK {
		return -1
	}
	for index := 0; index < len(currentParts) || index < len(latestParts); index++ {
		var left, right int
		if index < len(currentParts) {
			left = currentParts[index]
		}
		if index < len(latestParts) {
			right = latestParts[index]
		}
		if left < right {
			return -1
		}
		if left > right {
			return 1
		}
	}
	return 0
}

func numericVersion(version string) ([]int, bool) {
	if version == "" || strings.ContainsAny(version, "+-") {
		return nil, false
	}
	parts := strings.Split(version, ".")
	values := make([]int, len(parts))
	for index, part := range parts {
		if part == "" {
			return nil, false
		}
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 {
			return nil, false
		}
		values[index] = value
	}
	return values, true
}

func displayVersion(version string) string {
	if version == "" {
		return "dev"
	}
	return version
}

func resolvePrefix(options Options) (string, error) {
	if options.Prefix != "" {
		return validatePrefix(options.Prefix)
	}
	executable, err := options.Executable()
	if err != nil {
		return "", fmt.Errorf("locate asc executable: %w", err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return "", fmt.Errorf("resolve asc executable: %w", err)
	}
	if filepath.Base(executable) != "asc" || filepath.Base(filepath.Dir(executable)) != "bin" {
		return "", errors.New("cannot infer an installation prefix; run asc update --prefix /absolute/prefix")
	}
	return validatePrefix(filepath.Dir(filepath.Dir(executable)))
}

func validatePrefix(prefix string) (string, error) {
	clean := filepath.Clean(prefix)
	if !filepath.IsAbs(clean) || clean == string(filepath.Separator) || strings.ContainsRune(prefix, '\n') {
		return "", errors.New("update prefix must be an absolute path other than /")
	}
	return clean, nil
}

func validateManagedInstall(prefix string) error {
	manifest := filepath.Join(prefix, "share", "asc-devtools", "install-manifest")
	info, err := os.Lstat(manifest)
	if err != nil {
		return fmt.Errorf("refusing to update an unmanaged installation: %s is missing", manifest)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("refusing non-regular install manifest: %s", manifest)
	}
	return nil
}

func extractArchive(archive []byte, destination string) error {
	if err := os.Mkdir(destination, 0o755); err != nil {
		return err
	}
	gzipReader, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return err
	}
	defer gzipReader.Close()
	reader := tar.NewReader(gzipReader)
	var extracted int64
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		clean := filepath.Clean(filepath.FromSlash(header.Name))
		if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("unsafe archive path %q", header.Name)
		}
		if clean != "asc-devtools-go" && !strings.HasPrefix(clean, "asc-devtools-go"+string(filepath.Separator)) {
			return fmt.Errorf("unexpected archive path %q", header.Name)
		}
		target := filepath.Join(destination, clean)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			extracted += header.Size
			if header.Size < 0 || extracted > maxExtractedSize {
				return errors.New("archive exceeds extracted size limit")
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			mode := os.FileMode(0o644)
			if header.Mode&0o111 != 0 {
				mode = 0o755
			}
			file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
			if err != nil {
				return err
			}
			_, copyErr := io.CopyN(file, reader, header.Size)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		default:
			return fmt.Errorf("unsupported archive entry %q", header.Name)
		}
	}
}

func requireRegular(path string, executable bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("release bundle is missing %s", path)
	}
	if !info.Mode().IsRegular() || (executable && info.Mode().Perm()&0o111 == 0) {
		return fmt.Errorf("release bundle has invalid file %s", path)
	}
	return nil
}
