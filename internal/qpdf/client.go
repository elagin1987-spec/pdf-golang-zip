package qpdf

import (
	"context"
	"errors"
	"fmt"

	"pdf-golang-zip/internal/worker"
)

var (
	// ErrCheckFailed indicates that qpdf --check reported errors or warnings (exit codes 2 or 3).
	ErrCheckFailed = errors.New("qpdf check reported errors or warnings")
)

// Client provides bounded operations against the qpdf CLI.
type Client struct {
	binaryPath string
	runner     worker.Runner
}

// NewClient creates a new qpdf Client.
// If binaryPath is empty, "qpdf" is used.
// If runner is nil, a default LinuxRunner is used.
func NewClient(binaryPath string, runner worker.Runner) *Client {
	if binaryPath == "" {
		binaryPath = "qpdf"
	}
	if runner == nil {
		runner = worker.NewLinuxRunner(true)
	}
	return &Client{
		binaryPath: binaryPath,
		runner:     runner,
	}
}

// IsEncrypted tests whether a PDF file is encrypted using qpdf --is-encrypted.
// In qpdf:
// Exit 0 indicates the document is encrypted.
// Exit 2 indicates the document is NOT encrypted.
// Any other exit code or process failure is reported as an error.
func (c *Client) IsEncrypted(ctx context.Context, path string) (bool, error) {
	opts := worker.CommandOptions{
		MaxStdoutBytes: worker.DefaultDiagnosticLimit,
		MaxStderrBytes: worker.DefaultDiagnosticLimit,
	}

	res, err := c.runner.RunWithOptions(ctx, opts, c.binaryPath, "--is-encrypted", path)
	if res != nil {
		if res.ExitCode == 0 {
			return true, nil
		}
		if res.ExitCode == 2 {
			return false, nil
		}
	}

	if err != nil {
		return false, fmt.Errorf("qpdf is-encrypted failed on %s: %w", path, err)
	}

	return false, fmt.Errorf("qpdf is-encrypted returned unexpected exit code %d on %s", res.ExitCode, path)
}

// Check validates PDF file integrity using qpdf --check.
// Exit code 0 indicates success. Exit codes 2 or 3 indicate refusal/errors.
func (c *Client) Check(ctx context.Context, path string) error {
	opts := worker.CommandOptions{
		MaxStdoutBytes: worker.DefaultDiagnosticLimit,
		MaxStderrBytes: worker.DefaultDiagnosticLimit,
	}

	res, err := c.runner.RunWithOptions(ctx, opts, c.binaryPath, "--check", path)
	if err != nil {
		if res != nil && (res.ExitCode == 2 || res.ExitCode == 3) {
			return fmt.Errorf("%w (exit %d): %s", ErrCheckFailed, res.ExitCode, string(res.Stderr))
		}
		return fmt.Errorf("qpdf check failed on %s: %w", path, err)
	}

	if res != nil && res.ExitCode != 0 {
		if res.ExitCode == 2 || res.ExitCode == 3 {
			return fmt.Errorf("%w (exit %d)", ErrCheckFailed, res.ExitCode)
		}
		return fmt.Errorf("qpdf check process failed (exit %d)", res.ExitCode)
	}

	return nil
}

// AnalyzeRaw extracts structural JSON metadata from a PDF, returning both raw bytes and parsed structure.
func (c *Client) AnalyzeRaw(ctx context.Context, path string) ([]byte, *JSONOutput, error) {
	opts := worker.CommandOptions{
		MaxStdoutBytes: DefaultJSONLimit,
		MaxStderrBytes: worker.DefaultDiagnosticLimit,
	}

	res, err := c.runner.RunWithOptions(
		ctx,
		opts,
		c.binaryPath,
		"--json=2",
		"--json-stream-data=none",
		path,
	)
	if err != nil {
		if res != nil && (res.ExitCode == 2 || res.ExitCode == 3) {
			return nil, nil, fmt.Errorf("%w: qpdf analyze exit %d", ErrCheckFailed, res.ExitCode)
		}
		return nil, nil, fmt.Errorf("qpdf analyze failed on %s: %w", path, err)
	}

	if res.Truncated {
		return nil, nil, fmt.Errorf(
			"%w: json output for %s exceeded %d bytes",
			ErrStreamExceedsLimit,
			path,
			DefaultJSONLimit,
		)
	}

	parsed, err := ParseJSON(res.Stdout)
	if err != nil {
		return res.Stdout, nil, err
	}

	return res.Stdout, parsed, nil
}

// Analyze extracts structural JSON metadata from a PDF using qpdf --json=2 --json-stream-data=none.
// The output size is capped at DefaultJSONLimit (64 MiB).
func (c *Client) Analyze(ctx context.Context, path string) (*JSONOutput, error) {
	_, parsed, err := c.AnalyzeRaw(ctx, path)
	return parsed, err
}

// ReadStream extracts filtered stream data for a given object using qpdf --show-object=<id> --filtered-stream-data.
// If maxBytes <= 0, DefaultXMPLimit (4 MiB) is used.
func (c *Client) ReadStream(
	ctx context.Context,
	path string,
	objectID string,
	maxBytes int64,
) ([]byte, error) {
	norm_id, err := NormalizeObjectID(objectID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidObjectID, err)
	}

	if maxBytes <= 0 {
		maxBytes = DefaultXMPLimit
	}

	opts := worker.CommandOptions{
		MaxStdoutBytes: maxBytes,
		MaxStderrBytes: worker.DefaultDiagnosticLimit,
	}

	res, err := c.runner.RunWithOptions(
		ctx,
		opts,
		c.binaryPath,
		"--show-object="+norm_id,
		"--filtered-stream-data",
		path,
	)
	if err != nil {
		return nil, fmt.Errorf("qpdf read stream object %s failed on %s: %w", norm_id, path, err)
	}

	if res.Truncated {
		return res.Stdout, fmt.Errorf(
			"%w: stream object %s on %s exceeded %d bytes",
			ErrStreamExceedsLimit,
			norm_id,
			path,
			maxBytes,
		)
	}

	return res.Stdout, nil
}

// WriteLossless optimizes PDF streams and objects losslessly using canonical qpdf structural flags.
func (c *Client) WriteLossless(ctx context.Context, inputPath, outputPath string) error {
	args := []string{
		"--compress-streams=y",
		"--decode-level=generalized",
		"--recompress-flate",
		"--compression-level=9",
		"--object-streams=generate",
		inputPath,
		outputPath,
	}

	opts := worker.CommandOptions{
		MaxStdoutBytes: worker.DefaultDiagnosticLimit,
		MaxStderrBytes: worker.DefaultDiagnosticLimit,
	}

	res, err := c.runner.RunWithOptions(ctx, opts, c.binaryPath, args...)
	if err != nil {
		if res != nil && (res.ExitCode == 2 || res.ExitCode == 3) {
			return fmt.Errorf("%w: qpdf optimization exit %d", ErrCheckFailed, res.ExitCode)
		}
		return fmt.Errorf("qpdf lossless optimization failed: %w", err)
	}

	return nil
}
