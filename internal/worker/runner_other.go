//go:build !linux

package worker

import (
	"context"
	"os/exec"
	"time"
)

// LinuxRunner fallback on non-Linux platforms.
type LinuxRunner struct {
	DefaultNewProcessGroup bool
	DefaultGracePeriod     time.Duration
}

// NewLinuxRunner creates a runner on non-Linux platforms.
func NewLinuxRunner(defaultNewProcessGroup bool) *LinuxRunner {
	return &LinuxRunner{
		DefaultNewProcessGroup: defaultNewProcessGroup,
		DefaultGracePeriod:     DefaultGracePeriod,
	}
}

// Run executes a command with default options.
func (r *LinuxRunner) Run(ctx context.Context, tool string, args ...string) (*Result, error) {
	return r.RunWithOptions(ctx, CommandOptions{}, tool, args...)
}

// RunWithOptions executes a command with custom options.
func (r *LinuxRunner) RunWithOptions(
	ctx context.Context,
	opts CommandOptions,
	tool string,
	args ...string,
) (*Result, error) {
	grace_period := opts.GracePeriod
	if grace_period <= 0 {
		grace_period = r.DefaultGracePeriod
	}
	if grace_period <= 0 {
		grace_period = DefaultGracePeriod
	}

	max_stdout := opts.MaxStdoutBytes
	if max_stdout == 0 && opts.StdoutWriter == nil {
		max_stdout = DefaultDiagnosticLimit
	}
	max_stderr := opts.MaxStderrBytes
	if max_stderr == 0 && opts.StderrWriter == nil {
		max_stderr = DefaultDiagnosticLimit
	}

	cmd := exec.CommandContext(ctx, tool, args...)
	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}
	if len(opts.Env) > 0 {
		cmd.Env = opts.Env
	}
	if opts.Stdin != nil {
		cmd.Stdin = opts.Stdin
	}

	var stdout_buf *BoundedBuffer
	if opts.StdoutWriter != nil {
		cmd.Stdout = opts.StdoutWriter
	} else {
		stdout_buf = NewBoundedBuffer(max_stdout)
		cmd.Stdout = stdout_buf
	}

	var stderr_buf *BoundedBuffer
	if opts.StderrWriter != nil {
		cmd.Stderr = opts.StderrWriter
	} else {
		stderr_buf = NewBoundedBuffer(max_stderr)
		cmd.Stderr = stderr_buf
	}

	start_time := time.Now()
	err := cmd.Run()
	duration := time.Since(start_time)

	exit_code := 0
	if err != nil {
		if exit_err, ok := err.(*exec.ExitError); ok {
			exit_code = exit_err.ExitCode()
		} else {
			exit_code = -1
		}
	}

	var stdout_bytes, stderr_bytes []byte
	truncated := false
	if stdout_buf != nil {
		stdout_bytes = stdout_buf.Bytes()
		if stdout_buf.Truncated() {
			truncated = true
		}
	}
	if stderr_buf != nil {
		stderr_bytes = stderr_buf.Bytes()
		if stderr_buf.Truncated() {
			truncated = true
		}
	}

	res := &Result{
		Tool:      tool,
		Args:      args,
		ExitCode:  exit_code,
		Stdout:    stdout_bytes,
		Stderr:    stderr_bytes,
		Truncated: truncated,
		Duration:  duration,
	}

	if ctx.Err() != nil {
		return res, &ExitError{
			Tool:     tool,
			Args:     args,
			ExitCode: exit_code,
			Stdout:   string(stdout_bytes),
			Stderr:   string(stderr_bytes),
			Err:      ctx.Err(),
		}
	}

	if err != nil {
		return res, &ExitError{
			Tool:     tool,
			Args:     args,
			ExitCode: exit_code,
			Stdout:   string(stdout_bytes),
			Stderr:   string(stderr_bytes),
			Err:      err,
		}
	}

	if exit_code != 0 {
		return res, &ExitError{
			Tool:     tool,
			Args:     args,
			ExitCode: exit_code,
			Stdout:   string(stdout_bytes),
			Stderr:   string(stderr_bytes),
		}
	}

	return res, nil
}

// Compile-time check that LinuxRunner implements Runner.
var _ Runner = (*LinuxRunner)(nil)
