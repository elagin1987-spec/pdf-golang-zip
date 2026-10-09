package worker

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"
)

func TestBoundedBuffer(t *testing.T) {
	t.Run("within limit", func(t *testing.T) {
		buf := NewBoundedBuffer(10)
		n, err := buf.Write([]byte("hello"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 5 {
			t.Fatalf("expected 5 bytes written, got %d", n)
		}
		if buf.Truncated() {
			t.Fatalf("expected not truncated")
		}
		if buf.String() != "hello" {
			t.Fatalf("expected hello, got %s", buf.String())
		}
	})

	t.Run("exceeding limit", func(t *testing.T) {
		buf := NewBoundedBuffer(5)
		n, err := buf.Write([]byte("hello world"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 11 {
			t.Fatalf("expected 11 bytes reported written, got %d", n)
		}
		if !buf.Truncated() {
			t.Fatalf("expected truncated")
		}
		if buf.String() != "hello" {
			t.Fatalf("expected hello, got %s", buf.String())
		}
	})

	t.Run("unlimited", func(t *testing.T) {
		buf := NewBoundedBuffer(0)
		_, err := buf.Write([]byte("unlimited payload"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if buf.Truncated() {
			t.Fatalf("expected not truncated")
		}
		if buf.String() != "unlimited payload" {
			t.Fatalf("expected match, got %s", buf.String())
		}
	})
}

func TestExitErrorFormatting(t *testing.T) {
	err := &ExitError{
		Tool:     "test-tool",
		Args:     []string{"--arg"},
		ExitCode: 2,
		Stderr:   "something went wrong",
	}

	if err.Error() != "test-tool exit 2: something went wrong" {
		t.Fatalf("unexpected error string: %s", err.Error())
	}
}

func TestRunnerExecution(t *testing.T) {
	runner := NewLinuxRunner(false)

	t.Run("successful echo", func(t *testing.T) {
		res, err := runner.Run(context.Background(), "echo", "test output")
		if err != nil {
			t.Fatalf("expected success, got error: %v", err)
		}
		if res.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", res.ExitCode)
		}
		if !bytes.Contains(res.Stdout, []byte("test output")) {
			t.Fatalf("expected stdout to contain 'test output', got %s", string(res.Stdout))
		}
	})

	t.Run("non-zero exit", func(t *testing.T) {
		res, err := runner.Run(context.Background(), "sh", "-c", "exit 42")
		if err == nil {
			t.Fatalf("expected error for non-zero exit")
		}
		var exitErr *ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("expected ExitError, got %T: %v", err, err)
		}
		if exitErr.ExitCode != 42 {
			t.Fatalf("expected exit code 42, got %d", exitErr.ExitCode)
		}
		if res.ExitCode != 42 {
			t.Fatalf("expected res.ExitCode 42, got %d", res.ExitCode)
		}
	})

	t.Run("bounded stdout", func(t *testing.T) {
		opts := CommandOptions{
			MaxStdoutBytes: 4,
		}
		res, err := runner.RunWithOptions(context.Background(), opts, "echo", "123456789")
		if err != nil {
			t.Fatalf("expected success, got error: %v", err)
		}
		if !res.Truncated {
			t.Fatalf("expected truncated result")
		}
		if len(res.Stdout) != 4 {
			t.Fatalf("expected exactly 4 bytes in stdout, got %d", len(res.Stdout))
		}
	})

	t.Run("context cancellation kills process", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err := runner.Run(ctx, "sleep", "10")
		if err == nil {
			t.Fatalf("expected cancellation error")
		}
		var exitErr *ExitError
		if errors.As(err, &exitErr) {
			if !errors.Is(exitErr.Err, context.DeadlineExceeded) &&
				!errors.Is(exitErr.Err, context.Canceled) {
				t.Fatalf("expected DeadlineExceeded/Canceled, got %v", exitErr.Err)
			}
		}
	})

	t.Run("non-existent binary", func(t *testing.T) {
		_, err := runner.Run(context.Background(), "non-existent-binary-12345")
		if err == nil {
			t.Fatalf("expected error for non-existent binary")
		}
		var exitErr *ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("expected ExitError, got %T", err)
		}
		if !errors.Is(exitErr.Err, exec.ErrNotFound) && exitErr.ExitCode != -1 {
			t.Fatalf("expected not found, got %v", exitErr)
		}
	})
}
