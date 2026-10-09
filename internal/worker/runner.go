package worker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"
)

const (
	// DefaultDiagnosticLimit is the maximum buffer size for tool stdout/stderr diagnostic output (64 KiB).
	DefaultDiagnosticLimit int64 = 64 * 1024

	// DefaultGracePeriod is the time between SIGTERM and SIGKILL when canceling child processes.
	DefaultGracePeriod = 500 * time.Millisecond
)

// ProcessGroupMode controls how process groups are configured for child processes.
type ProcessGroupMode int

const (
	// ProcessGroupDefault uses the runner's default configuration.
	ProcessGroupDefault ProcessGroupMode = iota
	// ProcessGroupNew creates a new process group (Setpgid: true).
	ProcessGroupNew
	// ProcessGroupInherit inherits the parent process group.
	ProcessGroupInherit
)

// CommandOptions configures process execution.
type CommandOptions struct {
	Dir            string
	Env            []string
	Stdin          io.Reader
	StdoutWriter   io.Writer
	StderrWriter   io.Writer
	MaxStdoutBytes int64
	MaxStderrBytes int64
	GroupMode      ProcessGroupMode
	GracePeriod    time.Duration
}

// Result contains the output and execution metadata of a process.
type Result struct {
	Tool      string
	Args      []string
	ExitCode  int
	Stdout    []byte
	Stderr    []byte
	Truncated bool
	Duration  time.Duration
}

// ExitError represents a non-zero exit code or execution failure from a process.
type ExitError struct {
	Tool     string
	Args     []string
	ExitCode int
	Stdout   string
	Stderr   string
	Err      error
}

func (e *ExitError) Error() string {
	if len(e.Stderr) > 0 {
		return fmt.Sprintf("%s exit %d: %s", e.Tool, e.ExitCode, e.Stderr)
	}
	if e.Err != nil {
		return fmt.Sprintf("%s exit %d: %v", e.Tool, e.ExitCode, e.Err)
	}
	return fmt.Sprintf("%s exit %d", e.Tool, e.ExitCode)
}

func (e *ExitError) Unwrap() error {
	return e.Err
}

// Runner is an abstraction for executing commands.
type Runner interface {
	Run(ctx context.Context, tool string, args ...string) (*Result, error)
	RunWithOptions(ctx context.Context, opts CommandOptions, tool string, args ...string) (*Result, error)
}

// BoundedBuffer captures output up to a specified limit.
// Writes beyond the limit are discarded, and Truncated is set to true.
type BoundedBuffer struct {
	limit     int64
	buf       bytes.Buffer
	truncated bool
}

// NewBoundedBuffer creates a BoundedBuffer with the given maximum byte limit.
// If limit <= 0, no limit is enforced.
func NewBoundedBuffer(limit int64) *BoundedBuffer {
	return &BoundedBuffer{
		limit: limit,
	}
}

func (b *BoundedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if b.limit <= 0 {
		return b.buf.Write(p)
	}
	remaining := b.limit - int64(b.buf.Len())
	if remaining <= 0 {
		b.truncated = true
		return n, nil
	}
	if int64(len(p)) > remaining {
		b.buf.Write(p[:remaining])
		b.truncated = true
		return n, nil
	}
	b.buf.Write(p)
	return n, nil
}

func (b *BoundedBuffer) Bytes() []byte {
	return b.buf.Bytes()
}

func (b *BoundedBuffer) String() string {
	return b.buf.String()
}

func (b *BoundedBuffer) Truncated() bool {
	return b.truncated
}
