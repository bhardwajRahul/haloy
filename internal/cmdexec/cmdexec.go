package cmdexec

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// RunShellCommand - for shell commands with pipes, variables, etc.
func RunCommand(ctx context.Context, command, workDir string) error {
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("empty command")
	}

	shell, flag := findShell()
	cmd := exec.CommandContext(ctx, shell, flag, command)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	return cmd.Run()
}

// RunShellCommandWithOutput - shell command that returns output
func RunCommandWithOutput(ctx context.Context, command, workDir string) (string, error) {
	if strings.TrimSpace(command) == "" {
		return "", fmt.Errorf("empty command")
	}

	shell, flag := findShell()
	cmd := exec.CommandContext(ctx, shell, flag, command)
	cmd.Dir = workDir
	cmd.Env = os.Environ()

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("shell command failed: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// RunCLICommandInDir executes a CLI command directly with streamed output and no shell parsing.
func RunCLICommandInDir(ctx context.Context, workDir, name string, args ...string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("empty command")
	}

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.Error); ok && ee.Err == exec.ErrNotFound {
			return fmt.Errorf("command not found: '%s'. Is it installed and in your PATH?", name)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("command '%s' failed with exit code %d", name, exitErr.ExitCode())
		}
		return fmt.Errorf("failed to execute '%s': %w", name, err)
	}

	return nil
}

// RunCLICommand - for direct CLI tool execution (no shell interpretation)
func RunCLICommand(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = os.Environ()

	output, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.Error); ok && ee.Err == exec.ErrNotFound {
			return "", fmt.Errorf("command not found: '%s'. Is it installed and in your PATH?", name)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("command '%s' failed: %s", name, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to execute '%s': %w", name, err)
	}
	return strings.TrimSpace(string(output)), nil
}

func findShell() (string, string) {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell, "-c"
	}

	if bashPath, err := exec.LookPath("bash"); err == nil {
		return bashPath, "-c"
	}

	if comspec := os.Getenv("COMSPEC"); comspec != "" {
		return comspec, "/C"
	}

	if pwsh, err := exec.LookPath("powershell"); err == nil {
		return pwsh, "-Command"
	}

	if cmd, err := exec.LookPath("cmd"); err == nil {
		return cmd, "/C"
	}

	return "sh", "-c"
}
