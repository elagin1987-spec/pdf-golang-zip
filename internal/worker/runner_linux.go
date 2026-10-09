//go:build linux

package worker

import (
	"context"
	"os/exec"
	"syscall"
	"time"
)

// LinuxRunner executes commands on Linux with process group controls.
type LinuxRunner struct {
	DefaultNewProcessGroup bool
	DefaultGracePeriod     time.Duration
}

// NewLinuxRunner creates a LinuxRunner.
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
	new_pgrp := r.DefaultNewProcessGroup
	if opts.GroupMode == ProcessGroupNew {
		new_pgrp = true
	}
	if opts.GroupMode == ProcessGroupInherit {
		new_pgrp = false
	}

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

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	cmd := exec.Command(tool, args...)
	cmd.WaitDelay = 2 * grace_period
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

	if new_pgrp {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}
	}

	start_time := time.Now()
	err := cmd.Start()
	if err != nil {
		return nil, &ExitError{
			Tool:     tool,
			Args:     args,
			ExitCode: -1,
			Err:      err,
		}
	}

	done := make(chan struct{})
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		select {
		case <-ctx.Done():
			if new_pgrp && cmd.Process != nil {
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
			} else if cmd.Process != nil {
				_ = cmd.Process.Signal(syscall.SIGTERM)
			}

			select {
			case <-done:
				if new_pgrp {
					_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
				}
				return
			case <-time.After(grace_period):
				if new_pgrp && cmd.Process != nil {
					_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
				} else if cmd.Process != nil {
					_ = cmd.Process.Kill()
				}
			}
		case <-done:
			return
		}
	}()

	wait_err := cmd.Wait()
	close(done)
	<-watcherDone
	// A finished job must not leave descendants behind.
	if new_pgrp {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	duration := time.Since(start_time)

	exit_code := 0
	if wait_err != nil {
		if exit_err, ok := wait_err.(*exec.ExitError); ok {
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

	if wait_err != nil {
		return res, &ExitError{
			Tool:     tool,
			Args:     args,
			ExitCode: exit_code,
			Stdout:   string(stdout_bytes),
			Stderr:   string(stderr_bytes),
			Err:      wait_err,
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
