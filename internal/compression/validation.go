package compression

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"pdf-golang-zip/internal/ghostscript"
	"pdf-golang-zip/internal/pdf"
	"pdf-golang-zip/internal/qpdf"
)

// Candidate represents a validated compression candidate ready for selection.
type Candidate struct {
	Path         string
	Size         int64
	Engine       OptimizationEngine
	ImageObjects int
}

// ValidationResult captures the outcome of candidate inspection.
type ValidationResult struct {
	Candidate *Candidate
	Warning   string
	Valid     bool
}

// ValidateCandidate verifies candidate integrity, geometry, links, bookmarks, and text equality.
// For Ghostscript candidates, it additionally verifies txtwrite text against the normalized original text.
// For lossless candidates, Ghostscript text validation is omitted per contract.
func ValidateCandidate(
	ctx context.Context,
	qpdf_client *qpdf.Client,
	gs_client *ghostscript.Client,
	orig_analysis *pdf.DocumentAnalysis,
	orig_text string,
	cand_path string,
	engine OptimizationEngine,
	work_dir string,
) (*ValidationResult, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	fi, err := os.Stat(cand_path)
	if err != nil {
		return &ValidationResult{
			Valid:   false,
			Warning: WarningGhostscriptFailed,
		}, fmt.Errorf("failed to stat candidate %s: %w", cand_path, err)
	}
	cand_size := fi.Size()

	// 1. Structural integrity check via qpdf --check
	check_err := qpdf_client.Check(ctx, cand_path)
	if check_err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !errors.Is(check_err, qpdf.ErrCheckFailed) {
			return nil, check_err
		}
		warning := ""
		if engine == EngineGhostscript {
			warning = WarningGhostscriptFeaturesChanged
		}
		return &ValidationResult{
			Valid:   false,
			Warning: warning,
		}, nil
	}

	// 2. Extract structural JSON and snapshot
	cand_json, analyze_err := qpdf_client.Analyze(ctx, cand_path)
	if analyze_err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !errors.Is(analyze_err, qpdf.ErrCheckFailed) && !errors.Is(analyze_err, qpdf.ErrInvalidJSON) {
			return nil, analyze_err
		}
		warning := ""
		if engine == EngineGhostscript {
			warning = WarningGhostscriptFeaturesChanged
		}
		return &ValidationResult{
			Valid:   false,
			Warning: warning,
		}, nil
	}

	cand_snapshot, snap_err := pdf.BuildSnapshot(cand_json)
	if snap_err != nil {
		warning := ""
		if engine == EngineGhostscript {
			warning = WarningGhostscriptFeaturesChanged
		}
		return &ValidationResult{
			Valid:   false,
			Warning: warning,
		}, nil
	}

	// 3. Compare semantic snapshots (page count, geometry within 0.01 pt, links, outlines)
	matched, _ := pdf.CompareSnapshots(orig_analysis.Snapshot, cand_snapshot)
	if !matched {
		warning := ""
		if engine == EngineGhostscript {
			warning = WarningGhostscriptFeaturesChanged
		}
		return &ValidationResult{
			Valid:   false,
			Warning: warning,
		}, nil
	}

	if orig_analysis.PolicyResult != nil && orig_analysis.PolicyResult.LosslessOnly {
		if engine != EngineQPDF {
			return &ValidationResult{Warning: "special_features_changed"}, nil
		}
		fingerprint, err := pdf.PreservationFingerprint(ctx, qpdf_client, cand_path, cand_json)
		if err != nil {
			return nil, err
		}
		if fingerprint != orig_analysis.PreservationHash {
			return &ValidationResult{Warning: "special_features_changed"}, nil
		}
	}

	// 4. Runtime text validation for Ghostscript candidates
	if engine == EngineGhostscript {
		cand_text_path := filepath.Join(work_dir, filepath.Base(cand_path)+"-extracted.txt")
		cand_text, text_err := gs_client.ExtractText(ctx, cand_path, cand_text_path)
		if text_err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return &ValidationResult{
				Valid:   false,
				Warning: WarningTextValidationUnavailable,
			}, nil
		}

		if cand_text != orig_text {
			return &ValidationResult{
				Valid:   false,
				Warning: WarningGhostscriptTextChanged,
			}, nil
		}
	}

	// 5. Extract reachable image objects count for inventory reporting
	cand_inv := pdf.ExtractInventory(cand_json)
	cand_images := cand_inv.UniqueReachableImagesCount

	return &ValidationResult{
		Valid: true,
		Candidate: &Candidate{
			Path:         cand_path,
			Size:         cand_size,
			Engine:       engine,
			ImageObjects: cand_images,
		},
	}, nil
}
