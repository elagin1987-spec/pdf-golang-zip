package compression

import (
	"errors"
	"testing"

	"pdf-golang-zip/internal/ghostscript"
)

func TestParseProfile(t *testing.T) {
	tests := []struct {
		input    string
		expected Profile
		err      error
	}{
		{input: "", expected: ProfileAuto, err: nil},
		{input: "balanced", expected: ProfileBalanced, err: nil},
		{input: "BALANCED", expected: ProfileBalanced, err: nil},
		{input: "  compact  ", expected: ProfileCompact, err: nil},
		{input: "auto", expected: ProfileAuto, err: nil},
		{input: "lossless", expected: ProfileLossless, err: nil},
		{input: "LOSSLESS", expected: ProfileLossless, err: nil},
		{input: "screen", expected: "", err: ErrUnknownProfile},
		{input: "printer", expected: "", err: ErrUnknownProfile},
		{input: "invalid_profile", expected: "", err: ErrUnknownProfile},
	}

	for _, tc := range tests {
		t.Run("parse_"+tc.input, func(t *testing.T) {
			got, err := ParseProfile(tc.input)
			if tc.err != nil {
				if err == nil {
					t.Fatalf("expected error for %q, got nil", tc.input)
				}
				if !errors.Is(err, tc.err) {
					t.Fatalf("expected error wrapping %v, got %v", tc.err, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error for %q: %v", tc.input, err)
				}
				if got != tc.expected {
					t.Fatalf("expected profile %q, got %q", tc.expected, got)
				}
			}
		})
	}
}

func TestProfileToGSProfile(t *testing.T) {
	t.Run("balanced_maps_to_gs_balanced", func(t *testing.T) {
		gs_name, err := ProfileBalanced.ToGSProfile()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gs_name != ghostscript.ProfileBalanced {
			t.Fatalf("expected gs balanced, got %v", gs_name)
		}
	})

	t.Run("compact_maps_to_gs_compact", func(t *testing.T) {
		gs_name, err := ProfileCompact.ToGSProfile()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gs_name != ghostscript.ProfileCompact {
			t.Fatalf("expected gs compact, got %v", gs_name)
		}
	})

	t.Run("lossless_refuses_gs_mapping", func(t *testing.T) {
		_, err := ProfileLossless.ToGSProfile()
		if err == nil {
			t.Fatalf("expected error for lossless mapping to GS, got nil")
		}
		if !errors.Is(err, ErrLosslessCannotMapToGS) {
			t.Fatalf("expected ErrLosslessCannotMapToGS, got %v", err)
		}
	})

	t.Run("unknown_profile_refuses_mapping", func(t *testing.T) {
		p := Profile("unknown")
		_, err := p.ToGSProfile()
		if err == nil {
			t.Fatalf("expected error for unknown profile, got nil")
		}
		if !errors.Is(err, ErrUnknownProfile) {
			t.Fatalf("expected ErrUnknownProfile, got %v", err)
		}
	})
}
