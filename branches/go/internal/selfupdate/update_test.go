package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckAndInstallVerifiedRelease(t *testing.T) {
	archive := testArchive(t, map[string]testFile{
		"asc-devtools-go/asc":                {body: "binary", mode: 0o755},
		"asc-devtools-go/scripts/install.sh": {body: "#!/usr/bin/env bash\n", mode: 0o755},
	})
	hash := sha256.Sum256(archive)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/AI4SciComp/asc-devtools/releases/latest":
			fmt.Fprintf(response, `{"tag_name":"v0.2.0","assets":[{"name":"asc-devtools-go-linux-amd64.tar.gz","url":%q},{"name":"SHA256SUMS","url":%q}]}`, server.URL+"/archive", server.URL+"/sums")
		case "/archive":
			_, _ = response.Write(archive)
		case "/sums":
			fmt.Fprintf(response, "%s  asc-devtools-go-linux-amd64.tar.gz\n", hex.EncodeToString(hash[:]))
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	var output bytes.Buffer
	err := Run(context.Background(), Options{CurrentVersion: "0.1.0", CheckOnly: true, BaseURL: server.URL, HTTPClient: server.Client(), Stdout: &output, GOOS: "linux", GOARCH: "amd64"})
	if err != nil || !strings.Contains(output.String(), "0.2.0 is available") {
		t.Fatalf("check = %q, %v", output.String(), err)
	}

	prefix := t.TempDir()
	manifest := filepath.Join(prefix, "share", "asc-devtools", "install-manifest")
	if err := os.MkdirAll(filepath.Dir(manifest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte("managed"), 0o644); err != nil {
		t.Fatal(err)
	}
	called := false
	output.Reset()
	err = Run(context.Background(), Options{
		CurrentVersion: "0.1.0", Prefix: prefix, AssumeYes: true, BaseURL: server.URL,
		HTTPClient: server.Client(), Stdout: &output, GOOS: "linux", GOARCH: "amd64",
		BundleVersion: func(context.Context, string) (string, error) { return "0.2.0", nil },
		RunCommand: func(_ context.Context, name string, arguments ...string) error {
			called = true
			if name != "bash" || len(arguments) != 5 || arguments[1] != "--binary" || arguments[3] != "--prefix" || arguments[4] != prefix {
				t.Fatalf("command = %s %v", name, arguments)
			}
			if info, statErr := os.Stat(arguments[2]); statErr != nil || info.Mode().Perm()&0o111 == 0 {
				t.Fatalf("release binary = %v, %v", info, statErr)
			}
			return nil
		},
	})
	if err != nil || !called || !strings.Contains(output.String(), "Updated asc to 0.2.0") {
		t.Fatalf("update = called %v, output %q, err %v", called, output.String(), err)
	}
}

func TestNoPublishedRelease(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	err := Run(context.Background(), Options{CurrentVersion: "0.1.0", CheckOnly: true, BaseURL: server.URL, HTTPClient: server.Client()})
	if err == nil || !strings.Contains(err.Error(), "no published asc release") {
		t.Fatalf("error = %v", err)
	}
}

func TestRejectsUnsafeArchive(t *testing.T) {
	archive := testArchive(t, map[string]testFile{"../outside": {body: "bad", mode: 0o644}})
	err := extractArchive(archive, filepath.Join(t.TempDir(), "bundle"))
	if err == nil || !strings.Contains(err.Error(), "unsafe archive path") {
		t.Fatalf("error = %v", err)
	}
}

func TestVersionComparison(t *testing.T) {
	if compareVersions("0.1.0", "0.2.0") >= 0 || compareVersions("1.2.0", "1.2") != 0 || compareVersions("2.0.0", "1.9.9") <= 0 || compareVersions("dev", "1.0.0") >= 0 {
		t.Fatal("unexpected version comparison")
	}
}

type testFile struct {
	body string
	mode int64
}

func testArchive(t *testing.T, files map[string]testFile) []byte {
	t.Helper()
	var result bytes.Buffer
	gzipWriter := gzip.NewWriter(&result)
	tarWriter := tar.NewWriter(gzipWriter)
	for name, file := range files {
		if err := tarWriter.WriteHeader(&tar.Header{Name: name, Mode: file.mode, Size: int64(len(file.body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tarWriter.Write([]byte(file.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return result.Bytes()
}
