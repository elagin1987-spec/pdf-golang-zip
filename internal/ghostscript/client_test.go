package ghostscript

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"pdf-golang-zip/internal/worker"
)

type fakeRunner struct {
	runFunc func(ctx context.Context, opts worker.CommandOptions, tool string, args ...string) (*worker.Result, error)
}

func (f *fakeRunner) Run(ctx context.Context, tool string, args ...string) (*worker.Result, error) {
	return f.RunWithOptions(ctx, worker.CommandOptions{}, tool, args...)
}

func (f *fakeRunner) RunWithOptions(
	ctx context.Context,
	opts worker.CommandOptions,
	tool string,
	args ...string,
) (*worker.Result, error) {
	if f.runFunc != nil {
		return f.runFunc(ctx, opts, tool, args...)
	}
	return &worker.Result{
		Tool:     tool,
		Args:     args,
		ExitCode: 0,
	}, nil
}

func TestNormalizeText(t *testing.T) {
	input := "Line 1   \r\nLine 2\t\t\rLine 3   \n\n\n"
	expected := "Line 1\nLine 2\nLine 3"
	got := NormalizeText(input)
	if got != expected {
		t.Fatalf("expected:\n%q\ngot:\n%q", expected, got)
	}
}

func TestGhostscriptRewrite(t *testing.T) {
	t.Run("baseline balanced args", func(t *testing.T) {
		var captured_args []string
		runner := &fakeRunner{
			runFunc: func(
				_ context.Context,
				_ worker.CommandOptions,
				tool string,
				args ...string,
			) (*worker.Result, error) {
				if tool != "gs" {
					t.Fatalf("expected gs tool, got %s", tool)
				}
				captured_args = args
				return &worker.Result{
					ExitCode: 0,
				}, nil
			},
		}

		client := NewClient("gs", runner)
		diag, err := client.Rewrite(context.Background(), "in.pdf", "out.pdf", RewriteOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if diag.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d", diag.ExitCode)
		}

		expected_baseline := BuildBaselineBalancedArgs("in.pdf", "out.pdf")
		if !reflect.DeepEqual(captured_args, expected_baseline) {
			t.Fatalf("args mismatch:\nexpected: %v\ngot:      %v", expected_baseline, captured_args)
		}
	})

	t.Run("detects invalidfileaccess", func(t *testing.T) {
		runner := &fakeRunner{
			runFunc: func(
				_ context.Context,
				_ worker.CommandOptions,
				_ string,
				_ ...string,
			) (*worker.Result, error) {
				return &worker.Result{
					ExitCode: 1,
					Stderr:   []byte("GPL Ghostscript: Unrecoverable error, exit code 1\n   **** Error: /invalidfileaccess"),
				}, errors.New("exit 1")
			},
		}

		client := NewClient("gs", runner)
		diag, err := client.Rewrite(context.Background(), "in.pdf", "out.pdf", RewriteOptions{})
		if err == nil {
			t.Fatalf("expected error")
		}
		if !errors.Is(err, ErrInvalidFileAccess) {
			t.Fatalf("expected ErrInvalidFileAccess, got %v", err)
		}
		if !diag.HasInvalidFileAccess {
			t.Fatalf("expected HasInvalidFileAccess = true")
		}
	})

	t.Run("detects warnings and font substitution", func(t *testing.T) {
		runner := &fakeRunner{
			runFunc: func(
				_ context.Context,
				_ worker.CommandOptions,
				_ string,
				_ ...string,
			) (*worker.Result, error) {
				return &worker.Result{
					ExitCode: 0,
					Stderr:   []byte("Substituting font Helvetica for Arial\n   **** Warning: font not found"),
				}, nil
			},
		}

		client := NewClient("gs", runner)
		diag, err := client.Rewrite(context.Background(), "in.pdf", "out.pdf", RewriteOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !diag.HasWarning {
			t.Fatalf("expected HasWarning = true")
		}
		if len(diag.FontSubstitutions) != 1 {
			t.Errorf("expected 1 font substitution, got %d", len(diag.FontSubstitutions))
		}
	})

	t.Run("compact profile rewrite invocation", func(t *testing.T) {
		var captured_args []string
		runner := &fakeRunner{
			runFunc: func(
				_ context.Context,
				_ worker.CommandOptions,
				_ string,
				args ...string,
			) (*worker.Result, error) {
				captured_args = args
				return &worker.Result{ExitCode: 0}, nil
			},
		}

		client := NewClient("gs", runner)
		_, err := client.Rewrite(context.Background(), "in.pdf", "out.pdf", RewriteOptions{
			Profile: ProfileCompact,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected, _ := BuildArgs(ProfileCompact, "in.pdf", "out.pdf")
		if !reflect.DeepEqual(captured_args, expected) {
			t.Fatalf("args mismatch:\nexpected: %v\ngot:      %v", expected, captured_args)
		}
	})

	t.Run("lossless profile refuses rewrite", func(t *testing.T) {
		runner := &fakeRunner{}
		client := NewClient("gs", runner)
		_, err := client.Rewrite(context.Background(), "in.pdf", "out.pdf", RewriteOptions{
			Profile: ProfileLossless,
		})
		if !errors.Is(err, ErrLosslessNoRewriteArgs) {
			t.Fatalf("expected ErrLosslessNoRewriteArgs, got %v", err)
		}
	})
}

func TestGhostscriptExtractText(t *testing.T) {
	tmp_dir := t.TempDir()
	out_txt := filepath.Join(tmp_dir, "output.txt")

	t.Run("successful text extraction", func(t *testing.T) {
		runner := &fakeRunner{
			runFunc: func(
				_ context.Context,
				_ worker.CommandOptions,
				_ string,
				args ...string,
			) (*worker.Result, error) {
				// Simulate gs writing the file
				write_err := os.WriteFile(out_txt, []byte("Hello World \r\nSecond Line\n\n"), 0600)
				if write_err != nil {
					t.Fatalf("failed to write test file: %v", write_err)
				}
				expected_prefix := []string{
					"-q",
					"-dBATCH",
					"-dNOPAUSE",
					"-dSAFER",
					"-sDEVICE=txtwrite",
					"-dTextFormat=3",
					"--permit-file-read=in.pdf",
					"--permit-file-write=" + out_txt,
					"-sOutputFile=" + out_txt,
					"-f",
					"in.pdf",
				}
				if !reflect.DeepEqual(args, expected_prefix) {
					t.Fatalf("unexpected txtwrite args:\nexpected: %v\ngot:      %v", expected_prefix, args)
				}
				return &worker.Result{ExitCode: 0}, nil
			},
		}

		client := NewClient("gs", runner)
		text, err := client.ExtractText(context.Background(), "in.pdf", out_txt)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected_normalized := "Hello World\nSecond Line"
		if text != expected_normalized {
			t.Fatalf("expected %q, got %q", expected_normalized, text)
		}
	})

	t.Run("text exceeds 32 MiB cap", func(t *testing.T) {
		runner := &fakeRunner{
			runFunc: func(
				_ context.Context,
				_ worker.CommandOptions,
				_ string,
				_ ...string,
			) (*worker.Result, error) {
				f, err := os.Create(out_txt)
				if err != nil {
					t.Fatalf("failed to create file: %v", err)
				}
				defer f.Close()
				// Truncate to 33 MiB
				if err := f.Truncate(DefaultTextLimit + 1024); err != nil {
					t.Fatalf("failed to truncate: %v", err)
				}
				return &worker.Result{ExitCode: 0}, nil
			},
		}

		client := NewClient("gs", runner)
		_, err := client.ExtractText(context.Background(), "in.pdf", out_txt)
		if err == nil {
			t.Fatalf("expected ErrTextLimitExceeded")
		}
		if !errors.Is(err, ErrTextLimitExceeded) {
			t.Fatalf("expected ErrTextLimitExceeded, got %v", err)
		}
	})

	t.Run("safer denied error", func(t *testing.T) {
		runner := &fakeRunner{
			runFunc: func(
				_ context.Context,
				_ worker.CommandOptions,
				_ string,
				_ ...string,
			) (*worker.Result, error) {
				return &worker.Result{
					ExitCode: 1,
					Stderr:   []byte("Error: /invalidfileaccess"),
				}, errors.New("exit 1")
			},
		}

		client := NewClient("gs", runner)
		_, err := client.ExtractText(context.Background(), "in.pdf", out_txt)
		if err == nil {
			t.Fatalf("expected error")
		}
		if !errors.Is(err, ErrInvalidFileAccess) {
			t.Fatalf("expected ErrInvalidFileAccess, got %v", err)
		}
	})
}
