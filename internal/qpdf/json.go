package qpdf

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

var ErrDocumentLimit = errors.New("document limit exceeded")

// ErrInvalidJSON marks malformed candidate structure, distinct from tool failures.
var ErrInvalidJSON = errors.New("invalid qpdf json")

const (
	// MaxDocumentPages is the maximum allowed page count (1000).
	MaxDocumentPages = 1000

	// MaxDocumentObjects is the maximum allowed object count in traversal (100,000).
	MaxDocumentObjects = 100000
)

// JSONOutput represents the root structure of qpdf --json=2 output.
type JSONOutput struct {
	Version     int                          `json:"version"`
	Pages       []PageInfo                   `json:"pages"`
	PageLabels  []json.RawMessage            `json:"pagelabels,omitempty"`
	AcroForm    *AcroFormInfo                `json:"acroform,omitempty"`
	Attachments map[string]json.RawMessage   `json:"attachments,omitempty"`
	Encrypt     *EncryptInfo                 `json:"encrypt,omitempty"`
	QPDF        []map[string]json.RawMessage `json:"qpdf"`
}

// EncryptInfo represents encryption metadata in qpdf JSON v2.
type EncryptInfo struct {
	Encrypted bool `json:"encrypted"`
}

// PageInfo represents a page element in qpdf JSON v2.
type PageInfo struct {
	PagePosFrom1 int           `json:"pageposfrom1"`
	Object       string        `json:"object"`
	Label        *string       `json:"label"`
	Images       []ImageInfo   `json:"images"`
	Outlines     []OutlineInfo `json:"outlines"`
	Contents     []string      `json:"contents"`
}

// ImageInfo represents an image entry inside a page in qpdf JSON v2.
type ImageInfo struct {
	Name             string          `json:"name"`
	Object           string          `json:"object"`
	Width            int             `json:"width"`
	Height           int             `json:"height"`
	BitsPerComponent int             `json:"bitspercomponent"`
	ColorSpace       json.RawMessage `json:"colorspace"`
	Filter           []string        `json:"filter"`
}

// OutlineInfo represents a bookmark/outline entry in qpdf JSON v2.
type OutlineInfo struct {
	Title  string            `json:"title"`
	Dest   []json.RawMessage `json:"dest"`
	Object string            `json:"object"`
}

// AcroFormInfo represents form fields dictionary metadata.
type AcroFormInfo struct {
	Fields          []json.RawMessage `json:"fields"`
	HasAcroForm     bool              `json:"hasacroform,omitempty"`
	NeedAppearances bool              `json:"needappearances,omitempty"`
}

// ParseJSON parses qpdf JSON v2 data using json.Number to preserve exact numerical precision.
func ParseJSON(data []byte) (*JSONOutput, error) {
	return ParseJSONReader(bytes.NewReader(data))
}

// ParseJSONReader decodes qpdf JSON v2 from an io.Reader with UseNumber and structural caps.
func ParseJSONReader(r io.Reader) (*JSONOutput, error) {
	decoder := json.NewDecoder(r)
	decoder.UseNumber()

	var output JSONOutput
	err := decoder.Decode(&output)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidJSON, err)
	}

	if output.Version != 2 {
		return nil, fmt.Errorf("unsupported qpdf json version: %d (expected 2)", output.Version)
	}

	if len(output.Pages) > MaxDocumentPages {
		return nil, fmt.Errorf(
			"%w: document page count %d exceeds limit of %d",
			ErrDocumentLimit,
			len(output.Pages),
			MaxDocumentPages,
		)
	}

	total_objects := output.ObjectCount()
	if total_objects > MaxDocumentObjects {
		return nil, fmt.Errorf(
			"%w: document object count %d exceeds limit of %d",
			ErrDocumentLimit,
			total_objects,
			MaxDocumentObjects,
		)
	}

	return &output, nil
}

// ObjectCount returns the total number of indirect objects and special keys across all QPDF sections.
func (out *JSONOutput) ObjectCount() int {
	total := 0
	for _, section := range out.QPDF {
		total += len(section)
	}
	return total
}

// FindObject looks up an indirect object by reference (e.g. "obj:1 0 R", "1 0 R", "1,0", or "1").
func (out *JSONOutput) FindObject(ref string) (*ObjectEntry, bool) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return nil, false
	}

	var candidate_keys []string
	candidate_keys = append(candidate_keys, trimmed)
	if !strings.HasPrefix(trimmed, "obj:") {
		candidate_keys = append(candidate_keys, "obj:"+trimmed)
	} else {
		candidate_keys = append(candidate_keys, strings.TrimPrefix(trimmed, "obj:"))
	}

	normalized, err := NormalizeObjectID(trimmed)
	if err == nil {
		parts := strings.Split(normalized, ",")
		if len(parts) == 2 {
			candidate_keys = append(
				candidate_keys,
				fmt.Sprintf("obj:%s %s R", parts[0], parts[1]),
				fmt.Sprintf("%s %s R", parts[0], parts[1]),
			)
		}
	}

	for _, section := range out.QPDF {
		for _, key := range candidate_keys {
			if raw, exists := section[key]; exists {
				var entry ObjectEntry
				if json.Unmarshal(raw, &entry) == nil {
					return &entry, true
				}
			}
		}
	}

	return nil, false
}

// Trailer returns the trailer dictionary from the QPDF sections, if present.
func (out *JSONOutput) Trailer() (map[string]json.RawMessage, bool) {
	for _, section := range out.QPDF {
		if raw, exists := section["trailer"]; exists {
			var entry ObjectEntry
			if json.Unmarshal(raw, &entry) == nil {
				if d := entry.GetDict(); d != nil {
					return d, true
				}
			}
			var direct map[string]json.RawMessage
			if json.Unmarshal(raw, &direct) == nil {
				return direct, true
			}
		}
	}
	return nil, false
}

// Root finds and returns the Catalog (/Root) dictionary of the document.
func (out *JSONOutput) Root() (map[string]json.RawMessage, bool) {
	trailer, ok := out.Trailer()
	if ok && trailer != nil {
		if root_ref, exists := trailer["/Root"]; exists {
			var ref_str string
			if json.Unmarshal(root_ref, &ref_str) == nil {
				if obj, found := out.FindObject(ref_str); found {
					return obj.GetDict(), true
				}
			}
		}
	}

	// Fallback scan for /Type /Catalog
	for _, section := range out.QPDF {
		for _, raw := range section {
			var entry ObjectEntry
			if json.Unmarshal(raw, &entry) == nil {
				dict := entry.GetDict()
				if dict != nil {
					var type_name string
					if json.Unmarshal(dict["/Type"], &type_name) == nil && type_name == "/Catalog" {
						return dict, true
					}
				}
			}
		}
	}

	return nil, false
}
