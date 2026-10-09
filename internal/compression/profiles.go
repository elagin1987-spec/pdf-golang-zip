package compression

import (
	"errors"
	"fmt"
	"strings"

	"pdf-golang-zip/internal/ghostscript"
)

// Profile represents a compression profile supported by the service.
type Profile string

const (
	// ProfileBalanced compresses images targeting 300 DPI (standard print/viewing quality).
	ProfileBalanced Profile = "balanced"

	// ProfileCompact compresses images targeting 150 DPI (screen/web distribution).
	ProfileCompact Profile = "compact"

	// ProfileLossless optimizes PDF streams and object structure using qpdf without altering image bytes.
	ProfileLossless Profile = "lossless"
)

var (
	// ErrUnknownProfile is returned when an invalid compression profile is requested.
	ErrUnknownProfile = errors.New("unknown compression profile")

	// ErrLosslessCannotMapToGS is returned when attempting to map lossless profile to Ghostscript.
	ErrLosslessCannotMapToGS = errors.New("lossless profile cannot map to ghostscript rewrite")
)

// ParseProfile parses and validates a profile name string.
// If the input string is empty, ProfileBalanced is returned by default.
func ParseProfile(raw_name string) (Profile, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw_name))
	switch trimmed {
	case "", string(ProfileBalanced):
		return ProfileBalanced, nil
	case string(ProfileCompact):
		return ProfileCompact, nil
	case string(ProfileLossless):
		return ProfileLossless, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrUnknownProfile, raw_name)
	}
}

// ToGSProfile converts the compression profile to the corresponding Ghostscript profile name.
// ProfileLossless returns an error because lossless compression never invokes Ghostscript.
func (p Profile) ToGSProfile() (ghostscript.ProfileName, error) {
	switch p {
	case ProfileBalanced:
		return ghostscript.ProfileBalanced, nil
	case ProfileCompact:
		return ghostscript.ProfileCompact, nil
	case ProfileLossless:
		return "", ErrLosslessCannotMapToGS
	default:
		return "", fmt.Errorf("%w: %q", ErrUnknownProfile, p)
	}
}
