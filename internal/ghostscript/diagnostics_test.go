package ghostscript

import (
	"testing"
)

func TestParseDiagnostics(t *testing.T) {
	t.Run("clean execution", func(t *testing.T) {
		diag := ParseDiagnostics(0, "Processing page 1\n", "")
		if diag.ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", diag.ExitCode)
		}
		if diag.HasWarning {
			t.Errorf("expected HasWarning = false")
		}
		if diag.HasInvalidFileAccess {
			t.Errorf("expected HasInvalidFileAccess = false")
		}
		if diag.SuggestedFallback {
			t.Errorf("expected SuggestedFallback = false")
		}
		if len(diag.Warnings) != 0 || len(diag.Errors) != 0 || len(diag.FontSubstitutions) != 0 {
			t.Errorf("expected no warnings or errors, got %v / %v / %v", diag.Warnings, diag.Errors, diag.FontSubstitutions)
		}
	})

	t.Run("safer invalidfileaccess detection", func(t *testing.T) {
		stderr := "GPL Ghostscript 10.08.0: Unrecoverable error, exit code 1\n   **** Error: /invalidfileaccess in --file--"
		diag := ParseDiagnostics(1, "", stderr)
		if !diag.HasInvalidFileAccess {
			t.Errorf("expected HasInvalidFileAccess = true")
		}
		if !diag.SuggestedFallback {
			t.Errorf("expected SuggestedFallback = true")
		}
		if len(diag.Errors) == 0 {
			t.Errorf("expected errors to be captured")
		}
	})

	t.Run("font substitution detection", func(t *testing.T) {
		stderr := "Substituting font Helvetica for Arial\n"
		diag := ParseDiagnostics(0, "", stderr)
		if !diag.HasWarning {
			t.Errorf("expected HasWarning = true")
		}
		if len(diag.FontSubstitutions) != 1 {
			t.Fatalf("expected 1 font substitution, got %d", len(diag.FontSubstitutions))
		}
		if diag.FontSubstitutions[0] != "Substituting font Helvetica for Arial" {
			t.Errorf("unexpected font substitution text: %s", diag.FontSubstitutions[0])
		}
		if !diag.SuggestedFallback {
			t.Errorf("expected SuggestedFallback = true on font substitution")
		}
	})

	t.Run("gs warnings detection", func(t *testing.T) {
		stderr := "   **** Warning: stream length is incorrect.\nWarning: isolated color space\n"
		diag := ParseDiagnostics(0, "", stderr)
		if !diag.HasWarning {
			t.Errorf("expected HasWarning = true")
		}
		if len(diag.Warnings) != 2 {
			t.Errorf("expected 2 warnings, got %d: %v", len(diag.Warnings), diag.Warnings)
		}
	})

	t.Run("non-zero exit code triggers fallback", func(t *testing.T) {
		diag := ParseDiagnostics(1, "", "")
		if !diag.SuggestedFallback {
			t.Errorf("expected SuggestedFallback = true on exit code 1")
		}
	})
}
