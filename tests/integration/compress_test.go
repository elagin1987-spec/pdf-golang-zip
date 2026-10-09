package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"pdf-golang-zip/internal/pdf"
	"slices"
	"testing"
	"time"

	"pdf-golang-zip/internal/compression"
	"pdf-golang-zip/internal/ghostscript"
	"pdf-golang-zip/internal/qpdf"
)

func findTool(name string) (string, error) {
	// 1. Check PATH
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	// 2. Check /opt/pdf-tools/bin (Docker container location)
	opt_path := filepath.Join("/opt/pdf-tools/bin", name)
	if _, err := os.Stat(opt_path); err == nil {
		return opt_path, nil
	}
	return "", exec.ErrNotFound
}

func setupIntegrationService(t *testing.T) (*compression.Service, string, string) {
	t.Helper()
	qpdf_path, err := findTool("qpdf")
	if err != nil {
		t.Skipf("qpdf binary not found; skipping integration test: %v", err)
	}

	gs_path, err := findTool("gs")
	if err != nil {
		t.Skipf("gs binary not found; skipping integration test: %v", err)
	}

	qpdf_client := qpdf.NewClient(qpdf_path, nil)
	gs_client := ghostscript.NewClient(gs_path, nil)
	svc := compression.NewService(qpdf_client, gs_client)

	return svc, qpdf_path, gs_path
}

func findFixturesDir(t *testing.T) string {
	t.Helper()
	candidates := []string{
		"../../testdata/pdf",
		"../testdata/pdf",
		"testdata/pdf",
		"/fixtures",
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			abs, err := filepath.Abs(c)
			if err == nil {
				return abs
			}
		}
	}
	t.Skip("fixtures directory testdata/pdf not found; skipping integration test")
	return ""
}

func TestIntegrationCompressFlateRGB600DPI(t *testing.T) {
	svc, _, _ := setupIntegrationService(t)
	fixtures_dir := findFixturesDir(t)
	input_pdf := filepath.Join(fixtures_dir, "flate-rgb-600dpi.pdf")
	if _, err := os.Stat(input_pdf); err != nil {
		t.Skipf("fixture %s not found: %v", input_pdf, err)
	}

	temp_dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 1. Balanced profile test (561KB down to ~50KB Ghostscript)
	work_balanced := filepath.Join(temp_dir, "work-balanced")
	_ = os.MkdirAll(work_balanced, 0700)

	res_balanced, err := svc.Compress(ctx, compression.CompressRequest{
		InputPath: input_pdf,
		WorkDir:   work_balanced,
		Profile:   compression.ProfileBalanced,
	})
	if err != nil {
		t.Fatalf("balanced compression failed on %s: %v", input_pdf, err)
	}

	if res_balanced.Status != compression.StatusCompressed {
		t.Fatalf("expected compressed status, got %s (warnings: %v)", res_balanced.Status, res_balanced.Warnings)
	}
	if res_balanced.Engine != compression.EngineGhostscript {
		t.Fatalf("expected ghostscript engine, got %s, warnings: %v", res_balanced.Engine, res_balanced.Warnings)
	}
	if res_balanced.OutputBytes >= res_balanced.OriginalBytes {
		t.Fatalf("expected output bytes %d < original bytes %d", res_balanced.OutputBytes, res_balanced.OriginalBytes)
	}
	if res_balanced.SavedPercent < 80.0 {
		t.Fatalf("expected >=80%% saved percent for flate rgb 600dpi, got %.2f%%", res_balanced.SavedPercent)
	}

	// 2. Compact profile test (561KB down to ~16KB Ghostscript)
	work_compact := filepath.Join(temp_dir, "work-compact")
	_ = os.MkdirAll(work_compact, 0700)

	res_compact, err := svc.Compress(ctx, compression.CompressRequest{
		InputPath: input_pdf,
		WorkDir:   work_compact,
		Profile:   compression.ProfileCompact,
	})
	if err != nil {
		t.Fatalf("compact compression failed on %s: %v", input_pdf, err)
	}

	if res_compact.Status != compression.StatusCompressed {
		t.Fatalf("expected compressed status, got %s", res_compact.Status)
	}
	if res_compact.Engine != compression.EngineGhostscript {
		t.Fatalf("expected ghostscript engine, got %s", res_compact.Engine)
	}
	if res_compact.OutputBytes > res_balanced.OutputBytes {
		t.Fatalf("expected compact size %d <= balanced size %d", res_compact.OutputBytes, res_balanced.OutputBytes)
	}
}

func TestIntegrationCompressJPEG600DPI(t *testing.T) {
	svc, _, _ := setupIntegrationService(t)
	fixtures_dir := findFixturesDir(t)
	input_pdf := filepath.Join(fixtures_dir, "jpeg-600dpi.pdf")
	if _, err := os.Stat(input_pdf); err != nil {
		t.Skipf("fixture %s not found: %v", input_pdf, err)
	}

	temp_dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 1. Balanced profile test:
	// For jpeg-600dpi, GS balanced rewrite is ~227KB (due to QFactor 0.4 and sampling [1 1 1 1]),
	// whereas qpdf lossless is ~111.7KB (< original 111.9KB).
	// Truthful smallest selection must pick qpdf!
	work_balanced := filepath.Join(temp_dir, "work-balanced")
	_ = os.MkdirAll(work_balanced, 0700)

	res_balanced, err := svc.Compress(ctx, compression.CompressRequest{
		InputPath: input_pdf,
		WorkDir:   work_balanced,
		Profile:   compression.ProfileBalanced,
	})
	if err != nil {
		t.Fatalf("balanced compression failed on %s: %v", input_pdf, err)
	}

	if res_balanced.Status != compression.StatusCompressed {
		t.Fatalf("expected compressed status, got %s (warnings: %v)", res_balanced.Status, res_balanced.Warnings)
	}
	if res_balanced.Engine != compression.EngineQPDF {
		t.Fatalf("expected truthful smallest engine qpdf, got %s (warnings: %v)", res_balanced.Engine, res_balanced.Warnings)
	}
	if res_balanced.OutputBytes >= res_balanced.OriginalBytes {
		t.Fatalf("expected output bytes %d < original bytes %d", res_balanced.OutputBytes, res_balanced.OriginalBytes)
	}

	// 2. Compact profile test:
	// In compact mode, 150 DPI downsampling and QFactor 0.76 produces ~65.5KB Ghostscript candidate (< 111.7KB qpdf).
	// Winner must be Ghostscript!
	work_compact := filepath.Join(temp_dir, "work-compact")
	_ = os.MkdirAll(work_compact, 0700)

	res_compact, err := svc.Compress(ctx, compression.CompressRequest{
		InputPath: input_pdf,
		WorkDir:   work_compact,
		Profile:   compression.ProfileCompact,
	})
	if err != nil {
		t.Fatalf("compact compression failed on %s: %v", input_pdf, err)
	}

	if res_compact.Status != compression.StatusCompressed {
		t.Fatalf("expected compressed status, got %s", res_compact.Status)
	}
	if res_compact.Engine != compression.EngineGhostscript {
		t.Fatalf("expected ghostscript engine for compact, got %s", res_compact.Engine)
	}
	if res_compact.OutputBytes > res_balanced.OutputBytes {
		t.Fatalf("expected compact size %d <= balanced size %d", res_compact.OutputBytes, res_balanced.OutputBytes)
	}

	// 3. Lossless profile test (must NEVER use Ghostscript)
	work_lossless := filepath.Join(temp_dir, "work-lossless")
	_ = os.MkdirAll(work_lossless, 0700)

	res_lossless, err := svc.Compress(ctx, compression.CompressRequest{
		InputPath: input_pdf,
		WorkDir:   work_lossless,
		Profile:   compression.ProfileLossless,
	})
	if err != nil {
		t.Fatalf("lossless compression failed on %s: %v", input_pdf, err)
	}

	if res_lossless.Engine == compression.EngineGhostscript {
		t.Fatalf("contract violation: lossless profile selected ghostscript engine")
	}
	if res_lossless.Engine != compression.EngineQPDF {
		t.Fatalf("expected qpdf engine for lossless, got %s", res_lossless.Engine)
	}
}

func TestIntegrationCompressMonoPreservation(t *testing.T) {
	svc, _, _ := setupIntegrationService(t)
	fixtures_dir := findFixturesDir(t)
	input_pdf := filepath.Join(fixtures_dir, "mono-flate-600dpi.pdf")
	if _, err := os.Stat(input_pdf); err != nil {
		t.Skipf("fixture %s not found: %v", input_pdf, err)
	}

	temp_dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	work_dir := filepath.Join(temp_dir, "work-mono")
	_ = os.MkdirAll(work_dir, 0700)

	res, err := svc.Compress(ctx, compression.CompressRequest{
		InputPath: input_pdf,
		WorkDir:   work_dir,
		Profile:   compression.ProfileBalanced,
	})
	if err != nil {
		t.Fatalf("mono compression failed: %v", err)
	}

	// Mono must remain lossless or valid without downsample distortion
	if res.OutputBytes > res.OriginalBytes {
		t.Fatalf("expected output bytes <= original bytes, got out=%d in=%d", res.OutputBytes, res.OriginalBytes)
	}
}

func TestIntegrationCompressNoUpscale(t *testing.T) {
	svc, _, _ := setupIntegrationService(t)
	fixtures_dir := findFixturesDir(t)
	input_pdf := filepath.Join(fixtures_dir, "jpeg-150dpi.pdf")
	if _, err := os.Stat(input_pdf); err != nil {
		t.Skipf("fixture %s not found: %v", input_pdf, err)
	}

	temp_dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	work_dir := filepath.Join(temp_dir, "work-150dpi")
	_ = os.MkdirAll(work_dir, 0700)

	// Balanced profile targets 300 DPI; the 150 DPI image must NOT be upscaled
	res, err := svc.Compress(ctx, compression.CompressRequest{
		InputPath: input_pdf,
		WorkDir:   work_dir,
		Profile:   compression.ProfileBalanced,
	})
	if err != nil {
		t.Fatalf("compression failed on 150dpi fixture: %v", err)
	}

	// If Ghostscript produces larger output due to recompression/cloning,
	// pipeline must cleanly select original unchanged or lossless
	if res.OutputBytes > res.OriginalBytes {
		t.Fatalf("contract violation: returned PDF %d bytes is larger than original %d bytes", res.OutputBytes, res.OriginalBytes)
	}
}

// The Stage 07 matrix must be present: missing fixtures are failures, not skips.
func TestQualityFixtureMatrix(t *testing.T) {
	svc, qpdfPath, _ := setupIntegrationService(t)
	dir := findFixturesDir(t)
	data, err := os.ReadFile(filepath.Join(dir, "quality-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		File       string `json:"file"`
		SHA256     string `json:"sha256"`
		HTTPStatus int    `json:"http_status"`
		Warning    string `json:"warning"`
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) < 25 {
		t.Fatal("incomplete acceptance matrix")
	}
	qc := qpdf.NewClient(qpdfPath, nil)
	for _, fixture := range fixtures {
		for _, profile := range []compression.Profile{compression.ProfileLossless, compression.ProfileBalanced, compression.ProfileCompact} {
			t.Run(fixture.File+"/"+string(profile), func(t *testing.T) {
				input := filepath.Join(dir, fixture.File)
				original, err := os.ReadFile(input)
				if err != nil {
					t.Fatal(err)
				}
				hash := fmt.Sprintf("%x", sha256.Sum256(original))
				if hash != fixture.SHA256 {
					t.Fatal("fixture checksum mismatch")
				}
				ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
				defer cancel()
				res, err := svc.Compress(ctx, compression.CompressRequest{InputPath: input, WorkDir: t.TempDir(), Profile: profile, MaxPages: 500})
				if fixture.HTTPStatus != 0 {
					var refusal *compression.PolicyViolationError
					if !errors.As(err, &refusal) {
						t.Fatalf("expected pre-rewrite policy refusal: %v", err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if res.OutputBytes > res.OriginalBytes {
					t.Fatal("output grew")
				}
				output, err := os.ReadFile(res.OutputPath)
				if err != nil {
					t.Fatal(err)
				}
				if res.IsUnchanged() && !bytes.Equal(original, output) {
					t.Fatal("unchanged bytes differ")
				}
				before, err := pdf.AnalyzeDocument(ctx, qc, input)
				if err != nil {
					t.Fatal(err)
				}
				after, err := pdf.AnalyzeDocument(ctx, qc, res.OutputPath)
				if err != nil {
					t.Fatal(err)
				}
				if ok, reason := pdf.CompareSnapshots(before.Snapshot, after.Snapshot); !ok {
					t.Fatal(reason)
				}
				if fixture.Warning != "" && profile != compression.ProfileLossless {
					if !slices.Contains(res.Warnings, fixture.Warning) || res.Engine == compression.EngineGhostscript {
						t.Fatal("false fallback", res)
					}
				}
				if profile == compression.ProfileLossless && res.Engine == compression.EngineGhostscript {
					t.Fatal("lossless rewrote")
				}
				if res.Engine == compression.EngineGhostscript && len(res.Warnings) > 0 {
					t.Fatal("warned GS candidate accepted", res)
				}
				t.Logf("engine=%s bytes=%d->%d warnings=%v snapshot geometry/links/bookmarks equal", res.Engine, res.OriginalBytes, res.OutputBytes, res.Warnings)
			})
		}
	}
}
