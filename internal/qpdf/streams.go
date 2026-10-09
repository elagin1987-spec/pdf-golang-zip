package qpdf

import (
	"errors"
	"io"
)

const (
	// DefaultJSONLimit is the maximum memory limit for qpdf JSON output (64 MiB).
	DefaultJSONLimit int64 = 64 * 1024 * 1024

	// DefaultXMPLimit is the maximum byte limit for extracted XMP metadata streams (4 MiB).
	DefaultXMPLimit int64 = 4 * 1024 * 1024

	// DefaultStreamLimit is a general fallback cap for individual stream extraction (32 MiB).
	DefaultStreamLimit int64 = 32 * 1024 * 1024
)

var (
	// ErrStreamExceedsLimit is returned when an extracted stream exceeds its designated byte limit.
	ErrStreamExceedsLimit = errors.New("stream data exceeds configured byte limit")

	// ErrInvalidObjectID is returned when an object identifier cannot be parsed.
	ErrInvalidObjectID = errors.New("invalid or malformed object identifier")
)

// ReadBounded reads up to maxBytes + 1 from r.
// If the content exceeds maxBytes, ErrStreamExceedsLimit is returned along with truncated bytes.
func ReadBounded(r io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return io.ReadAll(r)
	}

	lr := io.LimitReader(r, maxBytes+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}

	if int64(len(data)) > maxBytes {
		return data[:maxBytes], ErrStreamExceedsLimit
	}

	return data, nil
}
