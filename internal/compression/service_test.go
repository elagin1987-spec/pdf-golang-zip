package compression

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"pdf-golang-zip/internal/ghostscript"
	"pdf-golang-zip/internal/pdf"
	"pdf-golang-zip/internal/qpdf"
	"pdf-golang-zip/internal/worker"
)

const sampleCMYKJSON = `{
	"version": 2,
	"pages": [
		{
			"pageposfrom1": 1,
			"object": "3 0 R",
			"images": [
				{
					"name": "/Im0",
					"object": "5 0 R",
					"width": 300,
					"height": 300,
					"bitspercomponent": 8,
					"colorspace": "/DeviceCMYK",
					"filter": ["/DCTDecode"]
				}
			]
		}
	],
	"qpdf": [
		{
			"trailer": {"value":{"/Root":"1 0 R"}},
			"obj:1 0 R": {"value":{"/Type":"/Catalog","/Pages":"2 0 R"}},
			"obj:2 0 R": {"value":{"/Type":"/Pages","/Kids":["3 0 R"],"/Count":1}},
			"obj:3 0 R": {"value":{"/Type":"/Page","/MediaBox":[0,0,216,216],"/CropBox":[0,0,216,216]}},
			"obj:5 0 R": {"stream":{"dict":{"/Width":300,"/Height":300,"/BitsPerComponent":8,"/ColorSpace":"/DeviceCMYK","/Filter":"/DCTDecode"}}}
		}
	]
}`

func setupServiceTestDoc(t *testing.T, size int, content []byte) (string, string) {
	t.Helper()
	temp_dir := t.TempDir()
	input_path := filepath.Join(temp_dir, "input.pdf")

	if len(content) == 0 {
		content = make([]byte, size)
		copy(content, []byte("%PDF-1.7 mock document\n"))
	}
	if err := os.WriteFile(input_path, content, 0600); err != nil {
		t.Fatalf("failed to write test input file: %v", err)
	}

	work_dir := filepath.Join(temp_dir, "work")
	if err := os.MkdirAll(work_dir, 0700); err != nil {
		t.Fatalf("failed to create work dir: %v", err)
	}

	return input_path, work_dir
}

func TestServiceCompressSuccessBalanced(t *testing.T) {
	input_path, work_dir := setupServiceTestDoc(t, 100000, nil)

	qpdf_runner := &fakeRunner{
		handler: func(ctx context.Context, opts worker.CommandOptions, tool string, args ...string) (*worker.Result, error) {
			if len(args) > 0 && args[0] == "--is-encrypted" {
				return &worker.Result{ExitCode: 2}, nil
			}
			for _, a := range args {
				if strings.HasPrefix(a, "--json=2") {
					return &worker.Result{ExitCode: 0, Stdout: []byte(sampleValidJSON)}, nil
				}
			}
			// WriteLossless creates output file
			for i, a := range args {
				if a == "--object-streams=generate" && i+2 < len(args) {
					out_file := args[i+2]
					_ = os.WriteFile(out_file, make([]byte, 80000), 0600)
					return &worker.Result{ExitCode: 0}, nil
				}
			}
			return &worker.Result{ExitCode: 0}, nil
		},
	}
	qpdf_client := qpdf.NewClient("qpdf", qpdf_runner)

	gs_runner := &fakeRunner{
		handler: func(ctx context.Context, opts worker.CommandOptions, tool string, args ...string) (*worker.Result, error) {
			for _, a := range args {
				if strings.HasPrefix(a, "-sOutputFile=") {
					out_file := strings.TrimPrefix(a, "-sOutputFile=")
					if strings.HasSuffix(out_file, ".txt") {
						_ = os.WriteFile(out_file, []byte("Test Document Content\n"), 0600)
					} else {
						// Ghostscript compressed output
						_ = os.WriteFile(out_file, make([]byte, 45000), 0600)
					}
					return &worker.Result{ExitCode: 0}, nil
				}
			}
			return &worker.Result{ExitCode: 0}, nil
		},
	}
	gs_client := ghostscript.NewClient("gs", gs_runner)

	svc := NewService(qpdf_client, gs_client)
	res, err := svc.Compress(context.Background(), CompressRequest{
		InputPath: input_path,
		WorkDir:   work_dir,
		Profile:   ProfileBalanced,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Status != StatusCompressed {
		t.Fatalf("expected status %s, got %s", StatusCompressed, res.Status)
	}
	if res.Engine != EngineGhostscript {
		t.Fatalf("expected engine %s, got %s", EngineGhostscript, res.Engine)
	}
	if res.OriginalBytes != 100000 {
		t.Fatalf("expected original bytes 100000, got %d", res.OriginalBytes)
	}
	if res.OutputBytes > 80000 {
		t.Fatalf("expected output bytes <= 80000, got %d", res.OutputBytes)
	}
	if res.SavedPercent <= 0 {
		t.Fatalf("expected positive saved percent, got %.2f", res.SavedPercent)
	}
	if res.InputImageObjects != 1 || res.OutputImageObjects != 1 {
		t.Fatalf("expected 1 image object, got in=%d out=%d", res.InputImageObjects, res.OutputImageObjects)
	}
}

func TestServiceCompressLosslessNeverCallsGhostscript(t *testing.T) {
	input_path, work_dir := setupServiceTestDoc(t, 100000, nil)

	var gs_invoked int32

	qpdf_runner := &fakeRunner{
		handler: func(ctx context.Context, opts worker.CommandOptions, tool string, args ...string) (*worker.Result, error) {
			if len(args) > 0 && args[0] == "--is-encrypted" {
				return &worker.Result{ExitCode: 2}, nil
			}
			for _, a := range args {
				if strings.HasPrefix(a, "--json=2") {
					return &worker.Result{ExitCode: 0, Stdout: []byte(sampleValidJSON)}, nil
				}
			}
			for i, a := range args {
				if a == "--object-streams=generate" && i+2 < len(args) {
					out_file := args[i+2]
					_ = os.WriteFile(out_file, make([]byte, 75000), 0600)
					return &worker.Result{ExitCode: 0}, nil
				}
			}
			return &worker.Result{ExitCode: 0}, nil
		},
	}
	qpdf_client := qpdf.NewClient("qpdf", qpdf_runner)

	gs_runner := &fakeRunner{
		handler: func(ctx context.Context, opts worker.CommandOptions, tool string, args ...string) (*worker.Result, error) {
			atomic.AddInt32(&gs_invoked, 1)
			return &worker.Result{ExitCode: 0}, nil
		},
	}
	gs_client := ghostscript.NewClient("gs", gs_runner)

	svc := NewService(qpdf_client, gs_client)
	res, err := svc.Compress(context.Background(), CompressRequest{
		InputPath: input_path,
		WorkDir:   work_dir,
		Profile:   ProfileLossless,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if atomic.LoadInt32(&gs_invoked) != 0 {
		t.Fatalf("contract violation: ghostscript was invoked %d times during lossless profile", gs_invoked)
	}

	if res.Status != StatusCompressed {
		t.Fatalf("expected status %s, got %s", StatusCompressed, res.Status)
	}
	if res.Engine != EngineQPDF {
		t.Fatalf("expected engine %s, got %s", EngineQPDF, res.Engine)
	}
	if res.OutputBytes != 75000 {
		t.Fatalf("expected output bytes 75000, got %d", res.OutputBytes)
	}
	if res.SavedPercent != 25.0 {
		t.Fatalf("expected saved percent 25.0, got %.2f", res.SavedPercent)
	}
}

func TestServiceCompressUnchangedWhenNoSizeReduction(t *testing.T) {
	input_path, work_dir := setupServiceTestDoc(t, 100000, nil)

	qpdf_runner := &fakeRunner{
		handler: func(ctx context.Context, opts worker.CommandOptions, tool string, args ...string) (*worker.Result, error) {
			if len(args) > 0 && args[0] == "--is-encrypted" {
				return &worker.Result{ExitCode: 2}, nil
			}
			for _, a := range args {
				if strings.HasPrefix(a, "--json=2") {
					return &worker.Result{ExitCode: 0, Stdout: []byte(sampleValidJSON)}, nil
				}
			}
			for i, a := range args {
				if a == "--object-streams=generate" && i+2 < len(args) {
					out_file := args[i+2]
					// Candidate is LARGER than original
					_ = os.WriteFile(out_file, make([]byte, 105000), 0600)
					return &worker.Result{ExitCode: 0}, nil
				}
			}
			return &worker.Result{ExitCode: 0}, nil
		},
	}
	qpdf_client := qpdf.NewClient("qpdf", qpdf_runner)

	gs_runner := &fakeRunner{
		handler: func(ctx context.Context, opts worker.CommandOptions, tool string, args ...string) (*worker.Result, error) {
			for _, a := range args {
				if strings.HasPrefix(a, "-sOutputFile=") {
					out_file := strings.TrimPrefix(a, "-sOutputFile=")
					if strings.HasSuffix(out_file, ".txt") {
						_ = os.WriteFile(out_file, []byte("Content\n"), 0600)
					} else {
						// Candidate is LARGER than original
						_ = os.WriteFile(out_file, make([]byte, 102000), 0600)
					}
					return &worker.Result{ExitCode: 0}, nil
				}
			}
			return &worker.Result{ExitCode: 0}, nil
		},
	}
	gs_client := ghostscript.NewClient("gs", gs_runner)

	svc := NewService(qpdf_client, gs_client)
	res, err := svc.Compress(context.Background(), CompressRequest{
		InputPath: input_path,
		WorkDir:   work_dir,
		Profile:   ProfileBalanced,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Status != StatusUnchanged {
		t.Fatalf("expected status %s, got %s", StatusUnchanged, res.Status)
	}
	if res.Engine != EngineOriginal {
		t.Fatalf("expected engine %s, got %s", EngineOriginal, res.Engine)
	}
	if res.OutputBytes != 100000 {
		t.Fatalf("expected output bytes 100000, got %d", res.OutputBytes)
	}
	if res.SavedPercent != 0.0 {
		t.Fatalf("expected saved percent 0.0, got %.2f", res.SavedPercent)
	}
	if res.OutputPath != input_path {
		t.Fatalf("expected output path %s, got %s", input_path, res.OutputPath)
	}
}

func TestServiceCompressGSWarningFallbackToQPDF(t *testing.T) {
	input_path, work_dir := setupServiceTestDoc(t, 100000, nil)

	qpdf_runner := &fakeRunner{
		handler: func(ctx context.Context, opts worker.CommandOptions, tool string, args ...string) (*worker.Result, error) {
			if len(args) > 0 && args[0] == "--is-encrypted" {
				return &worker.Result{ExitCode: 2}, nil
			}
			for _, a := range args {
				if strings.HasPrefix(a, "--json=2") {
					return &worker.Result{ExitCode: 0, Stdout: []byte(sampleValidJSON)}, nil
				}
			}
			for i, a := range args {
				if a == "--object-streams=generate" && i+2 < len(args) {
					out_file := args[i+2]
					_ = os.WriteFile(out_file, make([]byte, 85000), 0600)
					return &worker.Result{ExitCode: 0}, nil
				}
			}
			return &worker.Result{ExitCode: 0}, nil
		},
	}
	qpdf_client := qpdf.NewClient("qpdf", qpdf_runner)

	gs_runner := &fakeRunner{
		handler: func(ctx context.Context, opts worker.CommandOptions, tool string, args ...string) (*worker.Result, error) {
			for _, a := range args {
				if strings.HasPrefix(a, "-sOutputFile=") {
					out_file := strings.TrimPrefix(a, "-sOutputFile=")
					_ = os.WriteFile(out_file, make([]byte, 40000), 0600)
					// Emit font substitution warning in stderr
					return &worker.Result{
						ExitCode: 0,
						Stderr:   []byte("Substituting font Helvetica for Arial\n"),
					}, nil
				}
			}
			return &worker.Result{ExitCode: 0}, nil
		},
	}
	gs_client := ghostscript.NewClient("gs", gs_runner)

	svc := NewService(qpdf_client, gs_client)
	res, err := svc.Compress(context.Background(), CompressRequest{
		InputPath: input_path,
		WorkDir:   work_dir,
		Profile:   ProfileBalanced,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Engine != EngineQPDF {
		t.Fatalf("expected fallback to QPDF, got engine %s", res.Engine)
	}
	if res.OutputBytes != 85000 {
		t.Fatalf("expected output bytes 85000, got %d", res.OutputBytes)
	}

	has_warning := false
	for _, w := range res.Warnings {
		if w == WarningGhostscriptWarning {
			has_warning = true
			break
		}
	}
	if !has_warning {
		t.Fatalf("expected warning %s in %v", WarningGhostscriptWarning, res.Warnings)
	}
}

func TestServiceCompressGSTextMismatchFallbackToOriginal(t *testing.T) {
	input_path, work_dir := setupServiceTestDoc(t, 100000, nil)

	qpdf_runner := &fakeRunner{
		handler: func(ctx context.Context, opts worker.CommandOptions, tool string, args ...string) (*worker.Result, error) {
			if len(args) > 0 && args[0] == "--is-encrypted" {
				return &worker.Result{ExitCode: 2}, nil
			}
			for _, a := range args {
				if strings.HasPrefix(a, "--json=2") {
					return &worker.Result{ExitCode: 0, Stdout: []byte(sampleValidJSON)}, nil
				}
			}
			for i, a := range args {
				if a == "--object-streams=generate" && i+2 < len(args) {
					out_file := args[i+2]
					// Lossless is larger than original
					_ = os.WriteFile(out_file, make([]byte, 105000), 0600)
					return &worker.Result{ExitCode: 0}, nil
				}
			}
			return &worker.Result{ExitCode: 0}, nil
		},
	}
	qpdf_client := qpdf.NewClient("qpdf", qpdf_runner)

	gs_runner := &fakeRunner{
		handler: func(ctx context.Context, opts worker.CommandOptions, tool string, args ...string) (*worker.Result, error) {
			for _, a := range args {
				if strings.HasPrefix(a, "-sOutputFile=") {
					out_file := strings.TrimPrefix(a, "-sOutputFile=")
					if strings.Contains(out_file, "original-extracted.txt") {
						_ = os.WriteFile(out_file, []byte("Original Document Text\n"), 0600)
					} else if strings.HasSuffix(out_file, ".txt") {
						_ = os.WriteFile(out_file, []byte("Corrupted Changed Text\n"), 0600)
					} else {
						_ = os.WriteFile(out_file, make([]byte, 30000), 0600)
					}
					return &worker.Result{ExitCode: 0}, nil
				}
			}
			return &worker.Result{ExitCode: 0}, nil
		},
	}
	gs_client := ghostscript.NewClient("gs", gs_runner)

	svc := NewService(qpdf_client, gs_client)
	res, err := svc.Compress(context.Background(), CompressRequest{
		InputPath: input_path,
		WorkDir:   work_dir,
		Profile:   ProfileBalanced,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Status != StatusUnchanged {
		t.Fatalf("expected status %s on text mismatch fallback, got %s", StatusUnchanged, res.Status)
	}
	if res.Engine != EngineOriginal {
		t.Fatalf("expected engine %s, got %s", EngineOriginal, res.Engine)
	}

	has_warning := false
	for _, w := range res.Warnings {
		if w == WarningGhostscriptTextChanged {
			has_warning = true
			break
		}
	}
	if !has_warning {
		t.Fatalf("expected warning %s in %v", WarningGhostscriptTextChanged, res.Warnings)
	}
}

func TestServiceCompressIneligibleComplexColor(t *testing.T) {
	input_path, work_dir := setupServiceTestDoc(t, 100000, nil)

	var gs_invoked int32

	qpdf_runner := &fakeRunner{
		handler: func(ctx context.Context, opts worker.CommandOptions, tool string, args ...string) (*worker.Result, error) {
			if len(args) > 0 && args[0] == "--is-encrypted" {
				return &worker.Result{ExitCode: 2}, nil
			}
			for _, a := range args {
				if strings.HasPrefix(a, "--json=2") {
					return &worker.Result{ExitCode: 0, Stdout: []byte(sampleCMYKJSON)}, nil
				}
			}
			for i, a := range args {
				if a == "--object-streams=generate" && i+2 < len(args) {
					out_file := args[i+2]
					_ = os.WriteFile(out_file, make([]byte, 80000), 0600)
					return &worker.Result{ExitCode: 0}, nil
				}
			}
			return &worker.Result{ExitCode: 0}, nil
		},
	}
	qpdf_client := qpdf.NewClient("qpdf", qpdf_runner)

	gs_runner := &fakeRunner{
		handler: func(ctx context.Context, opts worker.CommandOptions, tool string, args ...string) (*worker.Result, error) {
			atomic.AddInt32(&gs_invoked, 1)
			return &worker.Result{ExitCode: 0}, nil
		},
	}
	gs_client := ghostscript.NewClient("gs", gs_runner)

	svc := NewService(qpdf_client, gs_client)
	res, err := svc.Compress(context.Background(), CompressRequest{
		InputPath: input_path,
		WorkDir:   work_dir,
		Profile:   ProfileBalanced,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if atomic.LoadInt32(&gs_invoked) != 0 {
		t.Fatalf("ghostscript should not be invoked for complex color PDF")
	}

	if res.Engine != EngineQPDF {
		t.Fatalf("expected engine %s, got %s", EngineQPDF, res.Engine)
	}

	has_warning := false
	for _, w := range res.Warnings {
		if w == WarningComplexColorPreserved {
			has_warning = true
			break
		}
	}
	if !has_warning {
		t.Fatalf("expected warning %s in %v", WarningComplexColorPreserved, res.Warnings)
	}
}

func TestServiceCompressMandatoryErrors(t *testing.T) {
	input_path, work_dir := setupServiceTestDoc(t, 100000, nil)

	// 1. Missing input path
	svc := NewService(qpdf.NewClient("qpdf", &fakeRunner{}), ghostscript.NewClient("gs", &fakeRunner{}))
	_, err := svc.Compress(context.Background(), CompressRequest{
		InputPath: "",
		WorkDir:   work_dir,
		Profile:   ProfileBalanced,
	})
	if !errors.Is(err, ErrEmptyInputPath) {
		t.Fatalf("expected ErrEmptyInputPath, got %v", err)
	}

	// 2. Missing work dir
	_, err = svc.Compress(context.Background(), CompressRequest{
		InputPath: input_path,
		WorkDir:   "/nonexistent/directory",
		Profile:   ProfileBalanced,
	})
	if !errors.Is(err, ErrWorkDirNotFound) {
		t.Fatalf("expected ErrWorkDirNotFound, got %v", err)
	}

	// 3. Preflight check fails on original
	qpdf_fail_check := qpdf.NewClient("qpdf", &fakeRunner{
		handler: func(ctx context.Context, opts worker.CommandOptions, tool string, args ...string) (*worker.Result, error) {
			if len(args) > 0 && args[0] == "--check" {
				return &worker.Result{ExitCode: 2, Stderr: []byte("damaged file")}, fmt.Errorf("damaged file")
			}
			return &worker.Result{ExitCode: 0}, nil
		},
	})
	svc_check_fail := NewService(qpdf_fail_check, ghostscript.NewClient("gs", &fakeRunner{}))
	_, err = svc_check_fail.Compress(context.Background(), CompressRequest{
		InputPath: input_path,
		WorkDir:   work_dir,
		Profile:   ProfileBalanced,
	})
	if !errors.Is(err, ErrPreflightFailed) {
		t.Fatalf("expected ErrPreflightFailed, got %v", err)
	}

	// 4. Policy refusal (encrypted document)
	qpdf_encrypted := qpdf.NewClient("qpdf", &fakeRunner{
		handler: func(ctx context.Context, opts worker.CommandOptions, tool string, args ...string) (*worker.Result, error) {
			if len(args) > 0 && args[0] == "--is-encrypted" {
				return &worker.Result{ExitCode: 0}, nil // encrypted!
			}
			return &worker.Result{ExitCode: 0}, nil
		},
	})
	svc_encrypted := NewService(qpdf_encrypted, ghostscript.NewClient("gs", &fakeRunner{}))
	_, err = svc_encrypted.Compress(context.Background(), CompressRequest{
		InputPath: input_path,
		WorkDir:   work_dir,
		Profile:   ProfileBalanced,
	})
	var policy_err *PolicyViolationError
	if !errors.As(err, &policy_err) {
		t.Fatalf("expected PolicyViolationError, got %v", err)
	}
	if policy_err.Code != pdf.CodeEncrypted {
		t.Fatalf("expected refusal code %s, got %s", pdf.CodeEncrypted, policy_err.Code)
	}

	// 5. Context cancelled
	ctx_canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = svc.Compress(ctx_canceled, CompressRequest{
		InputPath: input_path,
		WorkDir:   work_dir,
		Profile:   ProfileBalanced,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestRequiredLosslessFailureIsNotSuccess(t *testing.T) {
	input, dir := setupServiceTestDoc(t, 1000, nil)
	for _, mode := range []string{"write", "validation"} {
		t.Run(mode, func(t *testing.T) {
			runner := &fakeRunner{handler: func(ctx context.Context, opts worker.CommandOptions, tool string, args ...string) (*worker.Result, error) {
				if args[0] == "--is-encrypted" {
					return &worker.Result{ExitCode: 2}, nil
				}
				if args[0] == "--json=2" {
					return &worker.Result{Stdout: []byte(sampleValidJSON)}, nil
				}
				if args[0] == "--check" && args[1] != input && mode == "validation" {
					return &worker.Result{ExitCode: 2}, fmt.Errorf("invalid candidate")
				}
				if args[0] == "--compress-streams=y" {
					if mode == "write" {
						return nil, fmt.Errorf("storage failed")
					}
					_ = os.WriteFile(args[len(args)-1], []byte("candidate"), 0600)
				}
				return &worker.Result{}, nil
			}}
			svc := NewService(qpdf.NewClient("qpdf", runner), nil)
			result, err := svc.Compress(context.Background(), CompressRequest{InputPath: input, WorkDir: dir, Profile: ProfileLossless})
			if err == nil || result != nil {
				t.Fatal("mandatory failure hidden", result, err)
			}
		})
	}
}

func TestServicePreflightWarningAllowed(t *testing.T) {
	input_path, work_dir := setupServiceTestDoc(t, 100000, nil)

	qpdf_runner := &fakeRunner{
		handler: func(
			ctx context.Context,
			opts worker.CommandOptions,
			tool string,
			args ...string,
		) (*worker.Result, error) {
			if len(args) > 0 && args[0] == "--is-encrypted" {
				return &worker.Result{ExitCode: 2}, nil
			}
			if len(args) > 0 && args[0] == "--check" && args[1] == input_path {
				return &worker.Result{
					ExitCode: 3,
					Stderr:   []byte("stream inflate warning"),
				}, fmt.Errorf("exit status 3")
			}
			for _, a := range args {
				if strings.HasPrefix(a, "--json=2") {
					return &worker.Result{ExitCode: 0, Stdout: []byte(sampleValidJSON)}, nil
				}
			}
			for i, a := range args {
				if a == "--object-streams=generate" && i+2 < len(args) {
					out_file := args[i+2]
					_ = os.WriteFile(out_file, make([]byte, 80000), 0600)
					return &worker.Result{ExitCode: 0}, nil
				}
			}
			return &worker.Result{ExitCode: 0}, nil
		},
	}
	qpdf_client := qpdf.NewClient("qpdf", qpdf_runner)
	svc := NewService(qpdf_client, ghostscript.NewClient("gs", &fakeRunner{}))

	res, err := svc.Compress(context.Background(), CompressRequest{
		InputPath:         input_path,
		WorkDir:           work_dir,
		Profile:           ProfileLossless,
		AllowSpecialTypes: true,
	})
	if err != nil {
		t.Fatalf("expected Compress to succeed with exit 3 warning, got err: %v", err)
	}
	if res == nil {
		t.Fatalf("expected non-nil result")
	}

	found_warning := false
	for _, w := range res.Warnings {
		if w == "source_had_stream_warnings" {
			found_warning = true
			break
		}
	}
	if !found_warning {
		t.Fatalf("expected 'source_had_stream_warnings' in warnings, got: %v", res.Warnings)
	}
}
