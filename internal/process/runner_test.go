package process

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestOSRunnerCaptureAndFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix command fixtures")
	}
	runner := OSRunner{}
	result, err := runner.Run(context.Background(), Command{Name: "printf", Args: []string{"hello"}})
	if err != nil || result.Stdout != "hello" {
		t.Fatalf("Run() = %+v, %v", result, err)
	}
	_, err = runner.Run(context.Background(), Command{Name: "false"})
	var commandError *CommandError
	if !errors.As(err, &commandError) || commandError.ExitCode == 0 {
		t.Fatalf("Run(false) error = %v", err)
	}
}

func TestOSRunnerMissingExecutable(t *testing.T) {
	_, err := (OSRunner{}).Run(context.Background(), Command{Name: "asc-command-that-does-not-exist"})
	var missing *MissingExecutableError
	if !errors.As(err, &missing) {
		t.Fatalf("error = %v, want MissingExecutableError", err)
	}
}

func TestOSRunnerCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix command fixture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := (OSRunner{}).Run(ctx, Command{Name: "sleep", Args: []string{"5"}})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want deadline exceeded", err)
	}
}

func TestDescribeQuotesOnlyForDisplay(t *testing.T) {
	description := Describe(Command{Name: "git", Args: []string{"clone", "value with spaces", "plain"}})
	if !strings.Contains(description, `"value with spaces"`) || !strings.HasSuffix(description, " plain") {
		t.Fatalf("Describe() = %q", description)
	}
}
