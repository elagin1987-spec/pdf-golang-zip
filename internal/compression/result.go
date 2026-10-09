package compression

// CompressionStatus indicates whether the output document achieved size reduction.
type CompressionStatus string

const (
	// StatusCompressed indicates a valid smaller candidate was produced.
	StatusCompressed CompressionStatus = "compressed"

	// StatusUnchanged indicates no candidate was smaller than the original or candidates were rejected.
	StatusUnchanged CompressionStatus = "unchanged"
)

// OptimizationEngine identifies the tool engine that produced the selected output.
type OptimizationEngine string

const (
	// EngineOriginal indicates the original unaltered document was selected.
	EngineOriginal OptimizationEngine = "original"

	// EngineQPDF indicates qpdf structural or auto JPEG optimization was selected.
	EngineQPDF OptimizationEngine = "qpdf"

	// EngineGhostscript indicates Ghostscript lossy rewrite was selected.
	EngineGhostscript OptimizationEngine = "ghostscript"
)

// Warning codes emitted during document eligibility assessment and candidate validation.
const (
	WarningComplexColorPreserved      = "complex_color_preserved"
	WarningUnsupportedImageFilter     = "unsupported_image_filter"
	WarningImagePixelLimit            = "image_pixel_limit"
	WarningGhostscriptFailed          = "ghostscript_failed"
	WarningGhostscriptWarning         = "ghostscript_warning"
	WarningGhostscriptTextChanged     = "ghostscript_text_changed"
	WarningGhostscriptFeaturesChanged = "ghostscript_features_changed"
	WarningTextValidationUnavailable  = "text_validation_unavailable"
)

// Result holds the final compression outcome and metadata.
type Result struct {
	Status             CompressionStatus  `json:"status"`
	Engine             OptimizationEngine `json:"engine"`
	OriginalBytes      int64              `json:"original_bytes"`
	OutputBytes        int64              `json:"output_bytes"`
	SavedPercent       float64            `json:"saved_percent"`
	Warnings           []string           `json:"warnings"`
	InputImageObjects  int                `json:"input_image_objects"`
	OutputImageObjects int                `json:"output_image_objects"`
	OutputPath         string             `json:"output_path"`
}

// IsUnchanged returns true if the result preserves the original file bytes without compression.
func (r *Result) IsUnchanged() bool {
	if r == nil {
		return false
	}
	return r.Status == StatusUnchanged
}

// DeduplicateWarnings filters duplicate warning codes while preserving their original order of appearance.
func DeduplicateWarnings(warnings []string) []string {
	if len(warnings) == 0 {
		return []string{}
	}
	seen := make(map[string]bool, len(warnings))
	deduped := make([]string, 0, len(warnings))
	for _, w := range warnings {
		if !seen[w] {
			seen[w] = true
			deduped = append(deduped, w)
		}
	}
	return deduped
}
