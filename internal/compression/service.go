package compression

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"pdf-golang-zip/internal/ghostscript"
	"pdf-golang-zip/internal/pdf"
	"pdf-golang-zip/internal/qpdf"
)

var (
	// ErrEmptyInputPath is returned when no input path is provided.
	ErrEmptyInputPath = errors.New("input file path is empty")

	// ErrWorkDirNotFound is returned when the specified working directory does not exist.
	ErrWorkDirNotFound = errors.New("working directory does not exist")

	// ErrPreflightFailed is returned when the original document fails initial integrity check.
	ErrPreflightFailed = errors.New("preflight check failed on original document")
)

// PolicyViolationError indicates the document was refused by strict MVP policy.
type PolicyViolationError struct {
	Code   string
	Reason string
}

func (e *PolicyViolationError) Error() string {
	return fmt.Sprintf("policy refusal [%s]: %s", e.Code, e.Reason)
}

// CompressRequest parameters for compressing a single PDF document.
type CompressRequest struct {
	InputPath         string
	WorkDir           string
	Profile           Profile
	MaxPages          int
	AllowSpecialTypes bool
}

// Service coordinates the PDF compression pipeline and candidate selection.
type Service struct {
	qpdf_client *qpdf.Client
	gs_client   *ghostscript.Client
}

// NewService instantiates a compression Service with the required tool clients.
func NewService(qpdf_client *qpdf.Client, gs_client *ghostscript.Client) *Service {
	return &Service{
		qpdf_client: qpdf_client,
		gs_client:   gs_client,
	}
}

// Compress executes the end-to-end compression pipeline:
// 1. Preflight integrity and policy/inventory analysis.
// 2. QPDF lossless candidate generation.
// 3. Ghostscript rewrite and structural candidates (if eligible and requested).
// 4. Candidate validation and selection of the smallest valid result or original fallback.
func (s *Service) Compress(ctx context.Context, req CompressRequest) (*Result, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if req.InputPath == "" {
		return nil, ErrEmptyInputPath
	}

	orig_stat, err := os.Stat(req.InputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat input file %s: %w", req.InputPath, err)
	}
	original_bytes := orig_stat.Size()

	work_dir_stat, err := os.Stat(req.WorkDir)
	if err != nil || !work_dir_stat.IsDir() {
		return nil, fmt.Errorf("%w: %s", ErrWorkDirNotFound, req.WorkDir)
	}

	profile, err := ParseProfile(string(req.Profile))
	if err != nil {
		return nil, err
	}

	warnings := make([]string, 0)
	source_has_warnings := false

	// 1. Mandatory Preflight: qpdf --check on original document
	check_err := s.qpdf_client.Check(ctx, req.InputPath)
	if check_err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if errors.Is(check_err, qpdf.ErrCheckWarning) {
			warnings = append(warnings, "source_had_stream_warnings")
			source_has_warnings = true
		} else if errors.Is(check_err, qpdf.ErrCheckFailed) {
			return nil, fmt.Errorf("%w: %v", ErrPreflightFailed, check_err)
		} else {
			return nil, check_err
		}
	}

	// 2. Structural Analysis, Policy Check, and Inventory
	orig_analysis, err := pdf.AnalyzeDocumentWithOptions(
		ctx,
		s.qpdf_client,
		req.InputPath,
		pdf.PolicyOptions{
			AllowSpecialTypes: req.AllowSpecialTypes,
		},
	)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("document preflight analysis failed: %w", err)
	}

	if !orig_analysis.IsAllowed() {
		return nil, &PolicyViolationError{
			Code:   orig_analysis.PolicyResult.RefusalCode,
			Reason: orig_analysis.PolicyResult.RefusalReason,
		}
	}

	if req.MaxPages > 0 && len(orig_analysis.JSONOutput.Pages) > req.MaxPages {
		return nil, qpdf.ErrDocumentLimit
	}

	input_images := orig_analysis.Inventory.UniqueReachableImagesCount
	warnings = append(warnings, orig_analysis.Warnings()...)
	// A recoverable source warning is not proof of a valid rewrite. Preserve the
	// exact original rather than accepting a candidate with broken stream data.
	if source_has_warnings {
		warnings = append(warnings, "compression_skipped_source_warnings")
		return unchangedResult(req.InputPath, original_bytes, input_images, warnings), nil
	}

	candidates := make([]*Candidate, 0)

	// 3. Lossless candidate generation via qpdf
	lossless_path := filepath.Join(req.WorkDir, "lossless.pdf")
	lossless_err := s.qpdf_client.WriteLossless(ctx, req.InputPath, lossless_path)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if lossless_err != nil {
		return nil, fmt.Errorf("required lossless generation failed: %w", lossless_err)
	}
	if lossless_err == nil {
		val_res, val_err := ValidateCandidate(
			ctx,
			s.qpdf_client,
			s.gs_client,
			orig_analysis,
			"",
			lossless_path,
			EngineQPDF,
			req.WorkDir,
		)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if val_err != nil {
			return nil, val_err
		}
		if val_res != nil && !val_res.Valid && val_res.Warning == "special_features_changed" {
			warnings = append(warnings, val_res.Warning)
		} else if val_res == nil || !val_res.Valid {
			return nil, fmt.Errorf("required lossless validation failed")
		}
		if val_err == nil && val_res != nil && val_res.Valid && val_res.Candidate != nil {
			candidates = append(candidates, val_res.Candidate)
		}
	}

	// 4. Ghostscript lossy candidates (only for balanced/compact and when eligible)
	if profile != ProfileLossless && orig_analysis.IsEligibleForGS() {
		gs_profile, profile_err := profile.ToGSProfile()
		if profile_err == nil {
			gs_cand_path := filepath.Join(req.WorkDir, "gs-candidate.pdf")
			diag, rw_err := s.gs_client.Rewrite(
				ctx,
				req.InputPath,
				gs_cand_path,
				ghostscript.RewriteOptions{Profile: gs_profile},
			)
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}

			if rw_err != nil {
				warnings = append(warnings, WarningGhostscriptFailed)
			} else if diag.HasWarning || len(diag.FontSubstitutions) > 0 || diag.SuggestedFallback {
				warnings = append(warnings, WarningGhostscriptWarning)
			} else {
				// Rewrite succeeded cleanly without diagnostics warnings
				orig_text_path := filepath.Join(req.WorkDir, "original-extracted.txt")
				orig_text, text_err := s.gs_client.ExtractText(ctx, req.InputPath, orig_text_path)
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}

				if text_err != nil {
					warnings = append(warnings, WarningTextValidationUnavailable)
				} else {
					// Validate gs candidate
					val_res, val_err := ValidateCandidate(
						ctx,
						s.qpdf_client,
						s.gs_client,
						orig_analysis,
						orig_text,
						gs_cand_path,
						EngineGhostscript,
						req.WorkDir,
					)
					if ctx.Err() != nil {
						return nil, ctx.Err()
					}
					if val_err != nil {
						return nil, val_err
					}
					if val_err == nil && val_res != nil {
						if val_res.Valid && val_res.Candidate != nil {
							candidates = append(candidates, val_res.Candidate)
						} else if val_res.Warning != "" {
							warnings = append(warnings, val_res.Warning)
						}
					}

					// Optional qpdf structural candidate from GS output (without image recompression)
					gs_struct_path := filepath.Join(req.WorkDir, "gs-structural.pdf")
					struct_err := s.qpdf_client.WriteLossless(ctx, gs_cand_path, gs_struct_path)
					if ctx.Err() != nil {
						return nil, ctx.Err()
					}
					if struct_err != nil && !errors.Is(struct_err, qpdf.ErrCheckFailed) {
						return nil, struct_err
					}
					if struct_err == nil {
						val_struct, val_err2 := ValidateCandidate(
							ctx,
							s.qpdf_client,
							s.gs_client,
							orig_analysis,
							orig_text,
							gs_struct_path,
							EngineGhostscript,
							req.WorkDir,
						)
						if ctx.Err() != nil {
							return nil, ctx.Err()
						}
						if val_err2 != nil {
							return nil, val_err2
						}
						if val_err2 == nil && val_struct != nil {
							if val_struct.Valid && val_struct.Candidate != nil {
								candidates = append(candidates, val_struct.Candidate)
							} else if val_struct.Warning != "" {
								warnings = append(warnings, val_struct.Warning)
							}
						}
					}
				}
			}
		}
	}

	// 5. Select smallest valid candidate or original fallback
	var best_candidate *Candidate
	for _, c := range candidates {
		if c.Size < original_bytes {
			if best_candidate == nil || c.Size < best_candidate.Size {
				best_candidate = c
			}
		}
	}

	deduped_warnings := DeduplicateWarnings(warnings)

	// If no candidate is smaller than original, return original unchanged
	if best_candidate == nil {
		return unchangedResult(req.InputPath, original_bytes, input_images, deduped_warnings), nil
	}

	saved_percent := math.Round((float64(original_bytes-best_candidate.Size)/float64(original_bytes)*100.0)*100) / 100

	return &Result{
		Status:             StatusCompressed,
		Engine:             best_candidate.Engine,
		OriginalBytes:      original_bytes,
		OutputBytes:        best_candidate.Size,
		SavedPercent:       saved_percent,
		Warnings:           deduped_warnings,
		InputImageObjects:  input_images,
		OutputImageObjects: best_candidate.ImageObjects,
		OutputPath:         best_candidate.Path,
	}, nil
}

func unchangedResult(path string, size int64, images int, warnings []string) *Result {
	return &Result{Status: StatusUnchanged, Engine: EngineOriginal, OriginalBytes: size,
		OutputBytes: size, SavedPercent: 0, Warnings: DeduplicateWarnings(warnings),
		InputImageObjects: images, OutputImageObjects: images, OutputPath: path}
}
