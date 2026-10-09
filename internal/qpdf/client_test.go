package qpdf

import (
	"context"
	"errors"
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

func TestClientIsEncrypted(t *testing.T) {
	t.Run("exit 0 means encrypted", func(t *testing.T) {
		runner := &fakeRunner{
			runFunc: func(
				_ context.Context,
				_ worker.CommandOptions,
				tool string,
				args ...string,
			) (*worker.Result, error) {
				if tool != "qpdf" || args[0] != "--is-encrypted" {
					t.Fatalf("unexpected call: %s %v", tool, args)
				}
				return &worker.Result{
					Tool:     tool,
					Args:     args,
					ExitCode: 0,
				}, nil
			},
		}

		client := NewClient("qpdf", runner)
		encrypted, err := client.IsEncrypted(context.Background(), "test.pdf")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !encrypted {
			t.Fatalf("expected encrypted = true")
		}
	})

	t.Run("exit 2 means not encrypted", func(t *testing.T) {
		runner := &fakeRunner{
			runFunc: func(
				_ context.Context,
				_ worker.CommandOptions,
				tool string,
				args ...string,
			) (*worker.Result, error) {
				return &worker.Result{
					Tool:     tool,
					Args:     args,
					ExitCode: 2,
				}, nil
			},
		}

		client := NewClient("qpdf", runner)
		encrypted, err := client.IsEncrypted(context.Background(), "test.pdf")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if encrypted {
			t.Fatalf("expected encrypted = false")
		}
	})

	t.Run("other exit code returns error", func(t *testing.T) {
		runner := &fakeRunner{
			runFunc: func(
				_ context.Context,
				_ worker.CommandOptions,
				tool string,
				args ...string,
			) (*worker.Result, error) {
				return &worker.Result{
					Tool:     tool,
					Args:     args,
					ExitCode: 1,
				}, errors.New("command failed")
			},
		}

		client := NewClient("qpdf", runner)
		_, err := client.IsEncrypted(context.Background(), "test.pdf")
		if err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestClientCheck(t *testing.T) {
	t.Run("exit 0 success", func(t *testing.T) {
		runner := &fakeRunner{
			runFunc: func(
				_ context.Context,
				_ worker.CommandOptions,
				_ string,
				args ...string,
			) (*worker.Result, error) {
				if args[0] != "--check" {
					t.Fatalf("expected --check, got %v", args)
				}
				return &worker.Result{
					ExitCode: 0,
				}, nil
			},
		}

		client := NewClient("qpdf", runner)
		err := client.Check(context.Background(), "test.pdf")
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})

	t.Run("exit 2 or 3 failure", func(t *testing.T) {
		runner := &fakeRunner{
			runFunc: func(
				_ context.Context,
				_ worker.CommandOptions,
				_ string,
				_ ...string,
			) (*worker.Result, error) {
				return &worker.Result{
					ExitCode: 2,
					Stderr:   []byte("damaged syntax"),
				}, errors.New("exit 2")
			},
		}

		client := NewClient("qpdf", runner)
		err := client.Check(context.Background(), "test.pdf")
		if err == nil {
			t.Fatalf("expected error")
		}
		if !errors.Is(err, ErrCheckFailed) {
			t.Fatalf("expected ErrCheckFailed, got %v", err)
		}
	})
}

func TestClientAnalyze(t *testing.T) {
	sample_json := `{
		"version": 2,
		"pages": [
			{"pageposfrom1": 1, "object": "1 0 R", "images": []}
		],
		"qpdf": [
			{"obj:1 0 R": {"value": {"/Type": "/Page"}}}
		]
	}`

	runner := &fakeRunner{
		runFunc: func(
			_ context.Context,
			opts worker.CommandOptions,
			_ string,
			args ...string,
		) (*worker.Result, error) {
			if opts.MaxStdoutBytes != DefaultJSONLimit {
				t.Fatalf("expected %d MaxStdoutBytes, got %d", DefaultJSONLimit, opts.MaxStdoutBytes)
			}
			if args[0] != "--json=2" || args[1] != "--json-stream-data=none" {
				t.Fatalf("unexpected args: %v", args)
			}
			return &worker.Result{
				ExitCode: 0,
				Stdout:   []byte(sample_json),
			}, nil
		},
	}

	client := NewClient("qpdf", runner)
	out, err := client.Analyze(context.Background(), "test.pdf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(out.Pages))
	}
}

func TestClientReadStream(t *testing.T) {
	t.Run("successful stream read", func(t *testing.T) {
		runner := &fakeRunner{
			runFunc: func(
				_ context.Context,
				opts worker.CommandOptions,
				_ string,
				args ...string,
			) (*worker.Result, error) {
				expected_args := []string{
					"--show-object=4,0",
					"--filtered-stream-data",
					"test.pdf",
				}
				if !reflect.DeepEqual(args, expected_args) {
					t.Fatalf("expected %v, got %v", expected_args, args)
				}
				if opts.MaxStdoutBytes != DefaultXMPLimit {
					t.Fatalf("expected max stdout %d, got %d", DefaultXMPLimit, opts.MaxStdoutBytes)
				}
				return &worker.Result{
					ExitCode: 0,
					Stdout:   []byte("<xmpmeta>test</xmpmeta>"),
				}, nil
			},
		}

		client := NewClient("qpdf", runner)
		data, err := client.ReadStream(context.Background(), "test.pdf", "obj:4 0 R", 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(data) != "<xmpmeta>test</xmpmeta>" {
			t.Fatalf("unexpected data: %s", string(data))
		}
	})

	t.Run("stream truncated", func(t *testing.T) {
		runner := &fakeRunner{
			runFunc: func(
				_ context.Context,
				_ worker.CommandOptions,
				_ string,
				_ ...string,
			) (*worker.Result, error) {
				return &worker.Result{
					ExitCode:  0,
					Stdout:    []byte("partial"),
					Truncated: true,
				}, nil
			},
		}

		client := NewClient("qpdf", runner)
		_, err := client.ReadStream(context.Background(), "test.pdf", "4", 100)
		if err == nil {
			t.Fatalf("expected error on truncated stream")
		}
		if !errors.Is(err, ErrStreamExceedsLimit) {
			t.Fatalf("expected ErrStreamExceedsLimit, got %v", err)
		}
	})

	t.Run("invalid object id", func(t *testing.T) {
		client := NewClient("qpdf", &fakeRunner{})
		_, err := client.ReadStream(context.Background(), "test.pdf", "invalid_id", 0)
		if err == nil {
			t.Fatalf("expected error for invalid id")
		}
		if !errors.Is(err, ErrInvalidObjectID) {
			t.Fatalf("expected ErrInvalidObjectID, got %v", err)
		}
	})
}

func TestClientWriteLossless(t *testing.T) {
	runner := &fakeRunner{
		runFunc: func(
			_ context.Context,
			_ worker.CommandOptions,
			_ string,
			args ...string,
		) (*worker.Result, error) {
			expected_args := []string{
				"--compress-streams=y",
				"--decode-level=generalized",
				"--recompress-flate",
				"--compression-level=9",
				"--object-streams=generate",
				"input.pdf",
				"output.pdf",
			}
			if !reflect.DeepEqual(args, expected_args) {
				t.Fatalf("expected %v, got %v", expected_args, args)
			}
			return &worker.Result{
				ExitCode: 0,
			}, nil
		},
	}

	client := NewClient("qpdf", runner)
	err := client.WriteLossless(context.Background(), "input.pdf", "output.pdf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
