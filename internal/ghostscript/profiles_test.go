package ghostscript

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestProfileValidation(t *testing.T) {
	if !IsValidProfile(ProfileBalanced) {
		t.Errorf("expected ProfileBalanced to be valid")
	}
	if !IsValidProfile(ProfileCompact) {
		t.Errorf("expected ProfileCompact to be valid")
	}
	if !IsValidProfile(ProfileLossless) {
		t.Errorf("expected ProfileLossless to be valid")
	}
	if IsValidProfile(ProfileName("invalid")) {
		t.Errorf("expected 'invalid' profile to be invalid")
	}

	supported := SupportedProfiles()
	if len(supported) != 3 {
		t.Errorf("expected 3 supported profiles, got %d", len(supported))
	}
}

func TestGetProfileSettings(t *testing.T) {
	t.Run("balanced settings", func(t *testing.T) {
		s, err := GetProfileSettings(ProfileBalanced)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.ColorResolution != 300 || s.GrayResolution != 300 {
			t.Errorf("expected 300 dpi, got color=%d, gray=%d", s.ColorResolution, s.GrayResolution)
		}
		if s.DownsampleMono {
			t.Errorf("DownsampleMono must be false")
		}
		if s.QFactor != 0.4 {
			t.Errorf("expected QFactor 0.4, got %v", s.QFactor)
		}
		expected_samples := []int{1, 1, 1, 1}
		if !reflect.DeepEqual(s.ColorHSamples, expected_samples) {
			t.Errorf("expected ColorHSamples %v, got %v", expected_samples, s.ColorHSamples)
		}
	})

	t.Run("compact settings", func(t *testing.T) {
		s, err := GetProfileSettings(ProfileCompact)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.ColorResolution != 150 || s.GrayResolution != 150 {
			t.Errorf("expected 150 dpi, got color=%d, gray=%d", s.ColorResolution, s.GrayResolution)
		}
		if s.DownsampleMono {
			t.Errorf("DownsampleMono must be false")
		}
		if s.QFactor != 0.76 {
			t.Errorf("expected QFactor 0.76, got %v", s.QFactor)
		}
		expected_color_samples := []int{2, 1, 1, 2}
		if !reflect.DeepEqual(s.ColorHSamples, expected_color_samples) {
			t.Errorf("expected ColorHSamples %v, got %v", expected_color_samples, s.ColorHSamples)
		}
		expected_gray_samples := []int{1, 1, 1, 1}
		if !reflect.DeepEqual(s.GrayHSamples, expected_gray_samples) {
			t.Errorf("expected GrayHSamples %v, got %v", expected_gray_samples, s.GrayHSamples)
		}
	})

	t.Run("lossless returns ErrLosslessNoRewriteArgs", func(t *testing.T) {
		_, err := GetProfileSettings(ProfileLossless)
		if !errors.Is(err, ErrLosslessNoRewriteArgs) {
			t.Errorf("expected ErrLosslessNoRewriteArgs, got %v", err)
		}
	})

	t.Run("unknown profile returns ErrInvalidProfile", func(t *testing.T) {
		_, err := GetProfileSettings(ProfileName("unknown"))
		if !errors.Is(err, ErrInvalidProfile) {
			t.Errorf("expected ErrInvalidProfile, got %v", err)
		}
	})
}

func TestBuildArgs(t *testing.T) {
	in_path := "/tmp/input.pdf"
	out_path := "/tmp/output.pdf"

	t.Run("balanced profile matches baseline exactly", func(t *testing.T) {
		args, err := BuildArgs(ProfileBalanced, in_path, out_path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		baseline := BuildBaselineBalancedArgs(in_path, out_path)
		if !reflect.DeepEqual(args, baseline) {
			t.Fatalf("balanced args mismatch with baseline:\ngot:      %v\nexpected: %v", args, baseline)
		}
	})

	t.Run("compact profile args", func(t *testing.T) {
		args, err := BuildArgs(ProfileCompact, in_path, out_path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "-dColorImageResolution=150") {
			t.Errorf("missing -dColorImageResolution=150")
		}
		if !strings.Contains(joined, "-dGrayImageResolution=150") {
			t.Errorf("missing -dGrayImageResolution=150")
		}
		if !strings.Contains(joined, "-dDownsampleMonoImages=false") {
			t.Errorf("missing -dDownsampleMonoImages=false")
		}
		if !strings.Contains(joined, "-dColorImageDownsampleThreshold=1.1") {
			t.Errorf("missing -dColorImageDownsampleThreshold=1.1")
		}
		if !strings.Contains(joined, "-dColorImageDownsampleType=/Bicubic") {
			t.Errorf("missing -dColorImageDownsampleType=/Bicubic")
		}
		if !strings.Contains(joined, "/QFactor 0.76") {
			t.Errorf("missing /QFactor 0.76 in distiller settings")
		}
		if !strings.Contains(joined, "/HSamples [2 1 1 2]") {
			t.Errorf("missing /HSamples [2 1 1 2] in distiller settings")
		}

		// Contract: No /screen or quality presets
		if strings.Contains(joined, "PDFSETTINGS") || strings.Contains(joined, "/screen") {
			t.Errorf("prohibited preset found in compact args: %s", joined)
		}
	})

	t.Run("lossless profile refuses rewrite args", func(t *testing.T) {
		_, err := BuildArgs(ProfileLossless, in_path, out_path)
		if !errors.Is(err, ErrLosslessNoRewriteArgs) {
			t.Fatalf("expected ErrLosslessNoRewriteArgs, got %v", err)
		}
	})

	t.Run("unknown profile returns error", func(t *testing.T) {
		_, err := BuildArgs(ProfileName("ebook"), in_path, out_path)
		if !errors.Is(err, ErrInvalidProfile) {
			t.Fatalf("expected ErrInvalidProfile, got %v", err)
		}
	})
}

func TestAutoResolutionArgs(t *testing.T) {
	for _, dpi := range []int{150, 200} {
		args, err := BuildArgsWithResolution(ProfileBalanced, "in.pdf", "out.pdf", dpi)
		if err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(args, " ")
		expected := "-dColorImageResolution=200"
		if dpi == 150 {
			expected = "-dColorImageResolution=150"
		}
		if !strings.Contains(joined, expected) || !strings.Contains(joined, "-dDownsampleMonoImages=false") {
			t.Fatal(args)
		}
	}
	if _, err := BuildArgsWithResolution(ProfileBalanced, "in", "out", 300); err == nil {
		t.Fatal("invalid DPI accepted")
	}
}
