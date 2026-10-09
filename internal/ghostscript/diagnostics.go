package ghostscript

import (
	"strings"
)

// Diagnostics captures execution feedback and diagnostics from Ghostscript.
type Diagnostics struct {
	ExitCode             int
	Stdout               string
	Stderr               string
	HasWarning           bool
	HasInvalidFileAccess bool
	Warnings             []string
	Errors               []string
	FontSubstitutions    []string
	SuggestedFallback    bool
}

// ParseDiagnostics inspects Ghostscript execution results and extracts structured diagnostics.
// It detects SAFER sandbox violations (invalidfileaccess), font substitutions, warnings, and unrecoverable errors.
func ParseDiagnostics(exitCode int, stdout, stderr string) Diagnostics {
	diag := Diagnostics{
		ExitCode: exitCode,
		Stdout:   stdout,
		Stderr:   stderr,
	}

	combined := stdout + "\n" + stderr
	if strings.Contains(combined, "invalidfileaccess") {
		diag.HasInvalidFileAccess = true
		diag.SuggestedFallback = true
	}

	parseLines(stderr, &diag)
	parseLines(stdout, &diag)

	if exitCode != 0 {
		diag.SuggestedFallback = true
	}

	return diag
}

func parseLines(output string, diag *Diagnostics) {
	for _, raw_line := range strings.Split(output, "\n") {
		line := strings.TrimSpace(raw_line)
		if line == "" {
			continue
		}

		if strings.Contains(line, "Substituting font") {
			diag.FontSubstitutions = append(diag.FontSubstitutions, line)
			diag.HasWarning = true
			diag.SuggestedFallback = true
		} else if strings.Contains(line, "Warning:") {
			diag.Warnings = append(diag.Warnings, line)
			diag.HasWarning = true
			diag.SuggestedFallback = true
		} else if strings.Contains(line, "Error:") || strings.Contains(line, "Unrecoverable error") {
			diag.Errors = append(diag.Errors, line)
			diag.SuggestedFallback = true
		} else if strings.Contains(line, "Cannot find color profile") || strings.Contains(line, "Corrupt JPEG data") {
			diag.Warnings = append(diag.Warnings, line)
			diag.HasWarning = true
			diag.SuggestedFallback = true
		}
	}
}
