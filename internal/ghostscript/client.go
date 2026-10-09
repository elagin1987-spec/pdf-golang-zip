package ghostscript

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"pdf-golang-zip/internal/worker"
)

const (
	// DefaultTextLimit is the maximum size for extracted text (32 MiB).
	DefaultTextLimit int64 = 32 * 1024 * 1024

	// DefaultDiagnosticLimit is the maximum buffer size for stdout/stderr diagnostics (64 KiB).
	DefaultDiagnosticLimit int64 = 64 * 1024
)

var (
	// ErrTextLimitExceeded is returned when extracted text exceeds the 32 MiB cap.
	ErrTextLimitExceeded = errors.New("extracted text exceeds 32 MiB limit")

	// ErrGhostscriptFailed indicates a non-zero exit code from Ghostscript.
	ErrGhostscriptFailed = errors.New("ghostscript execution failed")

	// ErrGhostscriptWarning indicates Ghostscript emitted warnings or font substitution messages.
	ErrGhostscriptWarning = errors.New("ghostscript emitted warning or font substitution")

	// ErrInvalidFileAccess indicates SAFER denied file access.
	ErrInvalidFileAccess = errors.New("ghostscript SAFER denied file access (invalidfileaccess)")
)

// RewriteOptions configures Ghostscript PDF rewrite execution.
type RewriteOptions struct {
	// Profile specifies the compression profile to use (balanced, compact, lossless).
	// If Profile is empty and ExtraArgs is empty, ProfileBalanced is used by default.
	Profile ProfileName

	// ExtraArgs allows passing custom profile arguments. If non-empty, it overrides Profile.
	ExtraArgs []string
}

// Client provides bounded operations against the Ghostscript CLI.
type Client struct {
	binaryPath string
	runner     worker.Runner
}

// NewClient creates a new Ghostscript Client.
// If binaryPath is empty, "gs" is used.
// If runner is nil, a default LinuxRunner is used.
func NewClient(binaryPath string, runner worker.Runner) *Client {
	if binaryPath == "" {
		binaryPath = "gs"
	}
	if runner == nil {
		runner = worker.NewLinuxRunner(true)
	}
	return &Client{
		binaryPath: binaryPath,
		runner:     runner,
	}
}

// BuildBaselineBalancedArgs constructs the Stage 01 baseline arguments array for pdfwrite.
func BuildBaselineBalancedArgs(inputPath, outputPath string) []string {
	args, err := BuildArgs(ProfileBalanced, inputPath, outputPath)
	if err != nil {
		panic(err)
	}
	return args
}

// Rewrite executes Ghostscript pdfwrite to rewrite an input PDF to outputPath.
func (c *Client) Rewrite(
	ctx context.Context,
	inputPath, outputPath string,
	opts RewriteOptions,
) (Diagnostics, error) {
	var args []string
	if len(opts.ExtraArgs) > 0 {
		args = opts.ExtraArgs
	} else if opts.Profile != "" {
		if opts.Profile == ProfileLossless {
			return Diagnostics{}, ErrLosslessNoRewriteArgs
		}
		var err error
		args, err = BuildArgs(opts.Profile, inputPath, outputPath)
		if err != nil {
			return Diagnostics{}, err
		}
	} else {
		args = BuildBaselineBalancedArgs(inputPath, outputPath)
	}

	cmd_opts := worker.CommandOptions{
		MaxStdoutBytes: DefaultDiagnosticLimit,
		MaxStderrBytes: DefaultDiagnosticLimit,
	}

	res, err := c.runner.RunWithOptions(ctx, cmd_opts, c.binaryPath, args...)

	var stdout_str, stderr_str string
	exit_code := 0
	if res != nil {
		exit_code = res.ExitCode
		stdout_str = string(res.Stdout)
		stderr_str = string(res.Stderr)
	}

	diag := ParseDiagnostics(exit_code, stdout_str, stderr_str)

	if diag.HasInvalidFileAccess {
		return diag, ErrInvalidFileAccess
	}

	if err != nil {
		return diag, fmt.Errorf("%w: %v", ErrGhostscriptFailed, err)
	}

	if diag.ExitCode != 0 {
		return diag, fmt.Errorf("%w with exit code %d", ErrGhostscriptFailed, diag.ExitCode)
	}

	return diag, nil
}

// ExtractText extracts text from inputPath using the txtwrite device with TextFormat=3.
// The output file is written to outputPath and its normalized content is returned.
// An error is returned if the text exceeds DefaultTextLimit (32 MiB).
func (c *Client) ExtractText(ctx context.Context, inputPath, outputPath string) (string, error) {
	args := []string{
		"-q",
		"-dBATCH",
		"-dNOPAUSE",
		"-dSAFER",
		"-sDEVICE=txtwrite",
		"-dTextFormat=3",
		"--permit-file-read=" + inputPath,
		"--permit-file-write=" + outputPath,
		"-sOutputFile=" + outputPath,
		"-f",
		inputPath,
	}

	cmd_opts := worker.CommandOptions{
		MaxStdoutBytes: DefaultDiagnosticLimit,
		MaxStderrBytes: DefaultDiagnosticLimit,
	}

	res, err := c.runner.RunWithOptions(ctx, cmd_opts, c.binaryPath, args...)
	if err != nil {
		if res != nil && strings.Contains(string(res.Stderr), "invalidfileaccess") {
			return "", ErrInvalidFileAccess
		}
		return "", fmt.Errorf("txtwrite extraction failed: %w", err)
	}

	f, err := os.Open(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to open extracted text file %s: %w", outputPath, err)
	}
	defer f.Close()

	lr := io.LimitReader(f, DefaultTextLimit+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		return "", fmt.Errorf("failed to read extracted text: %w", err)
	}

	if int64(len(data)) > DefaultTextLimit {
		return "", ErrTextLimitExceeded
	}

	return NormalizeText(string(data)), nil
}

// NormalizeText standardizes text output per contract:
// 1. CRLF and CR are converted to LF.
// 2. Trailing spaces and tabs on each line are trimmed.
// 3. Terminal blank lines are removed.
func NormalizeText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}
