// Package process provides the external command execution boundary.
package process

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

// Command describes one external command invocation.
type Command struct {
	Name   string
	Args   []string
	Dir    string
	Stream bool
}

// Result contains captured command output and exit status.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Runner executes external commands.
type Runner interface {
	Run(context.Context, Command) (Result, error)
}

// OSRunner runs commands using os/exec.
type OSRunner struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// MissingExecutableError reports an unavailable command.
type MissingExecutableError struct {
	Name string
}

func (e *MissingExecutableError) Error() string {
	return fmt.Sprintf("required executable not found: %s", e.Name)
}

// CommandError reports a nonzero process result.
type CommandError struct {
	Command  string
	ExitCode int
	Detail   string
}

func (e *CommandError) Error() string {
	message := fmt.Sprintf("command failed (%d): %s", e.ExitCode, e.Command)
	if e.Detail != "" {
		message += ": " + e.Detail
	}
	return message
}

// Run executes a command without invoking a shell.
func (r OSRunner) Run(ctx context.Context, command Command) (Result, error) {
	if command.Name == "" {
		return Result{}, errors.New("command name must not be empty")
	}
	if _, err := exec.LookPath(command.Name); err != nil {
		return Result{}, &MissingExecutableError{Name: command.Name}
	}
	cmd := exec.CommandContext(ctx, command.Name, command.Args...)
	cmd.Dir = command.Dir
	cmd.Stdin = r.Stdin
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if command.Stream {
		cmd.Stdout = r.Stdout
		cmd.Stderr = r.Stderr
	} else {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	}
	err := cmd.Run()
	result := Result{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: 0}
	if err == nil {
		return result, nil
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		result.ExitCode = exitError.ExitCode()
		detail := strings.TrimSpace(result.Stderr)
		if detail == "" {
			detail = strings.TrimSpace(result.Stdout)
		}
		return result, &CommandError{Command: Describe(command), ExitCode: result.ExitCode, Detail: detail}
	}
	return result, fmt.Errorf("run %s: %w", Describe(command), err)
}

// Describe renders a command for humans; the returned string is never executed.
func Describe(command Command) string {
	parts := []string{quote(command.Name)}
	for _, argument := range command.Args {
		parts = append(parts, quote(argument))
	}
	return strings.Join(parts, " ")
}

func quote(value string) string {
	if value != "" && !strings.ContainsAny(value, " \t\n\r'\"") {
		return value
	}
	return strconv.Quote(value)
}

// ExitCode returns an external process exit status, or fallback otherwise.
func ExitCode(err error, fallback int) int {
	var commandError *CommandError
	if errors.As(err, &commandError) {
		return commandError.ExitCode
	}
	return fallback
}
