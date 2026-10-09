package compression

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pdf-golang-zip/internal/ghostscript"
	"pdf-golang-zip/internal/pdf"
	"pdf-golang-zip/internal/qpdf"
	"pdf-golang-zip/internal/worker"
)

const sampleValidJSON = `{
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
					"colorspace": "/DeviceRGB",
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
			"obj:5 0 R": {"stream":{"dict":{"/Width":300,"/Height":300,"/BitsPerComponent":8,"/ColorSpace":"/DeviceRGB","/Filter":"/DCTDecode"}}}
		}
	]
}`

const sampleGeometryMismatchJSON = `{
	"version": 2,
	"pages": [
		{
			"pageposfrom1": 1,
			"object": "3 0 R"
		}
	],
	"qpdf": [
		{
			"trailer": {"value":{"/Root":"1 0 R"}},
			"obj:1 0 R": {"value":{"/Type":"/Catalog","/Pages":"2 0 R"}},
			"obj:2 0 R": {"value":{"/Type":"/Pages","/Kids":["3 0 R"],"/Count":1}},
			"obj:3 0 R": {"value":{"/Type":"/Page","/MediaBox":[0,0,500,500],"/CropBox":[0,0,500,500]}}
		}
	]
}`

func setupValidationEnv(t *testing.T) (string, *pdf.DocumentAnalysis) {
	t.Helper()
	temp_dir := t.TempDir()

	orig_doc, err := qpdf.ParseJSON([]byte(sampleValidJSON))
	if err != nil {
		t.Fatalf("failed to parse sample JSON: %v", err)
	}

	snap, err := pdf.BuildSnapshot(orig_doc)
	if err != nil {
		t.Fatalf("failed to build snapshot: %v", err)
	}

	analysis := &pdf.DocumentAnalysis{
		JSONOutput: orig_doc,
		PolicyResult: &pdf.PolicyResult{
			Allowed: true,
		},
		Inventory: pdf.ExtractInventory(orig_doc),
		Snapshot:  snap,
	}

	return temp_dir, analysis
}

func TestValidateCandidateSuccess(t *testing.T) {
	temp_dir, analysis := setupValidationEnv(t)
	cand_path := filepath.Join(temp_dir, "candidate.pdf")
	if err := os.WriteFile(cand_path, []byte("%PDF-1.7 mock candidate"), 0600); err != nil {
		t.Fatalf("failed to write cand file: %v", err)
	}

	qpdf_runner := &fakeRunner{
		handler: func(ctx context.Context, opts worker.CommandOptions, tool string, args ...string) (*worker.Result, error) {
			for _, a := range args {
				if strings.HasPrefix(a, "--json=2") {
					return &worker.Result{ExitCode: 0, Stdout: []byte(sampleValidJSON)}, nil
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
					_ = os.WriteFile(out_file, []byte("Hello World\n"), 0600)
				}
			}
			return &worker.Result{ExitCode: 0}, nil
		},
	}
	gs_client := ghostscript.NewClient("gs", gs_runner)

	// 1. Ghostscript candidate with matching text
	res, err := ValidateCandidate(
		context.Background(),
		qpdf_client,
		gs_client,
		analysis,
		"Hello World",
		cand_path,
		EngineGhostscript,
		temp_dir,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Valid {
		t.Fatalf("expected candidate to be valid, got warning %s", res.Warning)
	}
	if res.Candidate.Engine != EngineGhostscript {
		t.Fatalf("expected engine %s, got %s", EngineGhostscript, res.Candidate.Engine)
	}
	if res.Candidate.ImageObjects != 1 {
		t.Fatalf("expected 1 image object, got %d", res.Candidate.ImageObjects)
	}

	// 2. QPDF lossless candidate (text check omitted)
	res_lossless, err := ValidateCandidate(
		context.Background(),
		qpdf_client,
		gs_client,
		analysis,
		"",
		cand_path,
		EngineQPDF,
		temp_dir,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res_lossless.Valid {
		t.Fatalf("expected lossless candidate to be valid, got warning %s", res_lossless.Warning)
	}
	if res_lossless.Candidate.Engine != EngineQPDF {
		t.Fatalf("expected engine %s, got %s", EngineQPDF, res_lossless.Candidate.Engine)
	}
}

func TestValidateCandidateCheckFailure(t *testing.T) {
	temp_dir, analysis := setupValidationEnv(t)
	cand_path := filepath.Join(temp_dir, "corrupt.pdf")
	_ = os.WriteFile(cand_path, []byte("corrupt"), 0600)

	qpdf_runner := &fakeRunner{
		handler: func(ctx context.Context, opts worker.CommandOptions, tool string, args ...string) (*worker.Result, error) {
			if len(args) > 0 && args[0] == "--check" {
				return &worker.Result{ExitCode: 2, Stderr: []byte("damaged file")}, fmt.Errorf("damaged file")
			}
			return &worker.Result{ExitCode: 0}, nil
		},
	}
	qpdf_client := qpdf.NewClient("qpdf", qpdf_runner)
	gs_client := ghostscript.NewClient("gs", &fakeRunner{})

	res, err := ValidateCandidate(
		context.Background(),
		qpdf_client,
		gs_client,
		analysis,
		"orig",
		cand_path,
		EngineGhostscript,
		temp_dir,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Fatalf("expected candidate to be invalid on check failure")
	}
	if res.Warning != WarningGhostscriptFeaturesChanged {
		t.Fatalf("expected warning %s, got %s", WarningGhostscriptFeaturesChanged, res.Warning)
	}
}

func TestValidateCandidateGeometryMismatch(t *testing.T) {
	temp_dir, analysis := setupValidationEnv(t)
	cand_path := filepath.Join(temp_dir, "mismatch.pdf")
	_ = os.WriteFile(cand_path, []byte("valid pdf"), 0600)

	qpdf_runner := &fakeRunner{
		handler: func(ctx context.Context, opts worker.CommandOptions, tool string, args ...string) (*worker.Result, error) {
			for _, a := range args {
				if strings.HasPrefix(a, "--json=2") {
					return &worker.Result{ExitCode: 0, Stdout: []byte(sampleGeometryMismatchJSON)}, nil
				}
			}
			return &worker.Result{ExitCode: 0}, nil
		},
	}
	qpdf_client := qpdf.NewClient("qpdf", qpdf_runner)
	gs_client := ghostscript.NewClient("gs", &fakeRunner{})

	res, err := ValidateCandidate(
		context.Background(),
		qpdf_client,
		gs_client,
		analysis,
		"orig",
		cand_path,
		EngineGhostscript,
		temp_dir,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Fatalf("expected candidate to be invalid on geometry mismatch")
	}
	if res.Warning != WarningGhostscriptFeaturesChanged {
		t.Fatalf("expected warning %s, got %s", WarningGhostscriptFeaturesChanged, res.Warning)
	}
}

func TestValidateCandidateTextMismatch(t *testing.T) {
	temp_dir, analysis := setupValidationEnv(t)
	cand_path := filepath.Join(temp_dir, "text_mismatch.pdf")
	_ = os.WriteFile(cand_path, []byte("valid pdf"), 0600)

	qpdf_runner := &fakeRunner{
		handler: func(ctx context.Context, opts worker.CommandOptions, tool string, args ...string) (*worker.Result, error) {
			for _, a := range args {
				if strings.HasPrefix(a, "--json=2") {
					return &worker.Result{ExitCode: 0, Stdout: []byte(sampleValidJSON)}, nil
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
					_ = os.WriteFile(out_file, []byte("Candidate Modified Text\n"), 0600)
				}
			}
			return &worker.Result{ExitCode: 0}, nil
		},
	}
	gs_client := ghostscript.NewClient("gs", gs_runner)

	res, err := ValidateCandidate(
		context.Background(),
		qpdf_client,
		gs_client,
		analysis,
		"Original Intact Text",
		cand_path,
		EngineGhostscript,
		temp_dir,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Fatalf("expected candidate to be rejected due to text mismatch")
	}
	if res.Warning != WarningGhostscriptTextChanged {
		t.Fatalf("expected warning %s, got %s", WarningGhostscriptTextChanged, res.Warning)
	}
}

func TestValidateCandidateTextExtractionFailure(t *testing.T) {
	temp_dir, analysis := setupValidationEnv(t)
	cand_path := filepath.Join(temp_dir, "text_failure.pdf")
	_ = os.WriteFile(cand_path, []byte("valid pdf"), 0600)

	qpdf_runner := &fakeRunner{
		handler: func(ctx context.Context, opts worker.CommandOptions, tool string, args ...string) (*worker.Result, error) {
			for _, a := range args {
				if strings.HasPrefix(a, "--json=2") {
					return &worker.Result{ExitCode: 0, Stdout: []byte(sampleValidJSON)}, nil
				}
			}
			return &worker.Result{ExitCode: 0}, nil
		},
	}
	qpdf_client := qpdf.NewClient("qpdf", qpdf_runner)

	gs_runner := &fakeRunner{
		handler: func(ctx context.Context, opts worker.CommandOptions, tool string, args ...string) (*worker.Result, error) {
			return &worker.Result{ExitCode: 1, Stderr: []byte("txtwrite internal crash")}, fmt.Errorf("txtwrite crashed")
		},
	}
	gs_client := ghostscript.NewClient("gs", gs_runner)

	res, err := ValidateCandidate(
		context.Background(),
		qpdf_client,
		gs_client,
		analysis,
		"Original Intact Text",
		cand_path,
		EngineGhostscript,
		temp_dir,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Valid {
		t.Fatalf("expected candidate to be rejected due to text extraction failure")
	}
	if res.Warning != WarningTextValidationUnavailable {
		t.Fatalf("expected warning %s, got %s", WarningTextValidationUnavailable, res.Warning)
	}
}
