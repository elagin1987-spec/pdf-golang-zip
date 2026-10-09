package ghostscript

import (
	"errors"
	"fmt"
)

// ProfileName represents the supported compression profiles.
type ProfileName string

const (
	// ProfileBalanced targets 300 DPI bicubic downsampling with QFactor 0.4.
	ProfileBalanced ProfileName = "balanced"

	// ProfileCompact targets 150 DPI bicubic downsampling with QFactor 0.76 and [2 1 1 2] sampling.
	ProfileCompact ProfileName = "compact"

	// ProfileLossless uses qpdf stream recompression only and never invokes ghostscript pdfwrite.
	ProfileLossless ProfileName = "lossless"
)

var (
	// ErrInvalidProfile indicates an unrecognized profile name.
	ErrInvalidProfile = errors.New("invalid or unsupported compression profile")

	// ErrLosslessNoRewriteArgs indicates that the lossless profile does not use ghostscript rewrite.
	ErrLosslessNoRewriteArgs = errors.New("lossless profile does not use ghostscript rewrite args")
)

// ProfileSettings captures the exact Ghostscript pdfwrite compression settings.
type ProfileSettings struct {
	Name                     ProfileName
	ColorResolution          int
	GrayResolution           int
	DownsampleColor          bool
	DownsampleGray           bool
	ColorDownsampleType      string
	GrayDownsampleType       string
	ColorDownsampleThreshold float64
	GrayDownsampleThreshold  float64
	DownsampleMono           bool
	AutoFilterColor          bool
	AutoFilterGray           bool
	QFactor                  float64
	ColorHSamples            []int
	ColorVSamples            []int
	GrayHSamples             []int
	GrayVSamples             []int
}

// BalancedSettings defines the standard balanced 300-DPI profile.
var BalancedSettings = ProfileSettings{
	Name:                     ProfileBalanced,
	ColorResolution:          300,
	GrayResolution:           300,
	DownsampleColor:          true,
	DownsampleGray:           true,
	ColorDownsampleType:      "/Bicubic",
	GrayDownsampleType:       "/Bicubic",
	ColorDownsampleThreshold: 1.1,
	GrayDownsampleThreshold:  1.1,
	DownsampleMono:           false,
	AutoFilterColor:          true,
	AutoFilterGray:           true,
	QFactor:                  0.4,
	ColorHSamples:            []int{1, 1, 1, 1},
	ColorVSamples:            []int{1, 1, 1, 1},
	GrayHSamples:             []int{1, 1, 1, 1},
	GrayVSamples:             []int{1, 1, 1, 1},
}

// CompactSettings defines the compact 150-DPI profile with chroma subsampling.
var CompactSettings = ProfileSettings{
	Name:                     ProfileCompact,
	ColorResolution:          150,
	GrayResolution:           150,
	DownsampleColor:          true,
	DownsampleGray:           true,
	ColorDownsampleType:      "/Bicubic",
	GrayDownsampleType:       "/Bicubic",
	ColorDownsampleThreshold: 1.1,
	GrayDownsampleThreshold:  1.1,
	DownsampleMono:           false,
	AutoFilterColor:          true,
	AutoFilterGray:           true,
	QFactor:                  0.76,
	ColorHSamples:            []int{2, 1, 1, 2},
	ColorVSamples:            []int{2, 1, 1, 2},
	GrayHSamples:             []int{1, 1, 1, 1},
	GrayVSamples:             []int{1, 1, 1, 1},
}

// IsValidProfile returns true if the profile name is one of the recognized profiles.
func IsValidProfile(name ProfileName) bool {
	switch name {
	case ProfileBalanced, ProfileCompact, ProfileLossless:
		return true
	default:
		return false
	}
}

// SupportedProfiles returns all supported profile names.
func SupportedProfiles() []ProfileName {
	return []ProfileName{
		ProfileBalanced,
		ProfileCompact,
		ProfileLossless,
	}
}

// GetProfileSettings returns the fixed settings for the specified profile.
// Calling this with ProfileLossless returns ErrLosslessNoRewriteArgs because
// lossless compression never invokes pdfwrite.
func GetProfileSettings(name ProfileName) (ProfileSettings, error) {
	switch name {
	case ProfileBalanced:
		return BalancedSettings, nil
	case ProfileCompact:
		return CompactSettings, nil
	case ProfileLossless:
		return ProfileSettings{}, ErrLosslessNoRewriteArgs
	default:
		return ProfileSettings{}, fmt.Errorf("%w: %q", ErrInvalidProfile, name)
	}
}

// BuildArgs constructs the full Ghostscript CLI arguments array for pdfwrite.
// Returns ErrLosslessNoRewriteArgs if profile is ProfileLossless.
func BuildArgs(profile ProfileName, inputPath, outputPath string) ([]string, error) {
	settings, err := GetProfileSettings(profile)
	if err != nil {
		return nil, err
	}

	distiller_ps := BuildDistillerSettings(settings)

	return []string{
		"-dBATCH",
		"-dNOPAUSE",
		"-dSAFER",
		"-sDEVICE=pdfwrite",
		"-dCompatibilityLevel=1.7",
		"-dAutoRotatePages=/None",
		"-sColorConversionStrategy=LeaveColorUnchanged",
		"-dConvertCMYKImagesToRGB=false",
		"-dPassThroughJPEGImages=true",
		"-dPassThroughJPXImages=true",
		"-dEmbedAllFonts=true",
		"-dSubsetFonts=true",
		"-dCompressFonts=true",
		"-dCompressStreams=true",
		"-dDetectDuplicateImages=true",
		"-dFastWebView=false",
		"-dPreserveAnnots=true",
		fmt.Sprintf("-dDownsampleColorImages=%t", settings.DownsampleColor),
		fmt.Sprintf("-dDownsampleGrayImages=%t", settings.DownsampleGray),
		fmt.Sprintf("-dColorImageResolution=%d", settings.ColorResolution),
		fmt.Sprintf("-dGrayImageResolution=%d", settings.GrayResolution),
		"-dColorImageDownsampleType=" + settings.ColorDownsampleType,
		"-dGrayImageDownsampleType=" + settings.GrayDownsampleType,
		fmt.Sprintf("-dColorImageDownsampleThreshold=%.1f", settings.ColorDownsampleThreshold),
		fmt.Sprintf("-dGrayImageDownsampleThreshold=%.1f", settings.GrayDownsampleThreshold),
		fmt.Sprintf("-dDownsampleMonoImages=%t", settings.DownsampleMono),
		fmt.Sprintf("-dAutoFilterColorImages=%t", settings.AutoFilterColor),
		fmt.Sprintf("-dAutoFilterGrayImages=%t", settings.AutoFilterGray),
		"--permit-file-read=" + inputPath,
		"--permit-file-write=" + outputPath,
		"-sOutputFile=" + outputPath,
		"-c",
		distiller_ps,
		"-f",
		inputPath,
	}, nil
}
