package pdf

import (
	"encoding/json"
	"math"
	"sort"
	"strings"

	"pdf-golang-zip/internal/qpdf"
)

const (
	// MaxImagePixelsCap is the 40 megapixel limit per image XObject.
	MaxImagePixelsCap int64 = 40_000_000

	// Warning codes for GS eligibility.
	WarningComplexColorPreserved = "complex_color_preserved"
	WarningUnsupportedImageFilter = "unsupported_image_filter"
	WarningImagePixelLimit       = "image_pixel_limit"
)

// Supported image filters in Ghostscript pdfwrite baseline.
var supportedFilters = map[string]bool{
	"dctdecode":        true,
	"flatedecode":      true,
	"lzwdecode":        true,
	"runlengthdecode":  true,
	"jpxdecode":        true,
	"ccittfaxdecode":   true,
	"jbig2decode":      true,
}

// ImageSummary describes the properties of a reachable image XObject.
type ImageSummary struct {
	ObjectID          string   `json:"object_id"`
	Name              string   `json:"name"`
	Width             int      `json:"width"`
	Height            int      `json:"height"`
	Pixels            int64    `json:"pixels"`
	BitsPerComponent  int      `json:"bits_per_component"`
	ColorSpace        string   `json:"color_space"`
	Filters           []string `json:"filters"`
	HasMask           bool     `json:"has_mask"`
	HasSMask          bool     `json:"has_smask"`
}

// DocumentInventory collects structural image and resource metrics from the PDF.
//
// NOTICE: Inline images (BI/ID/EI within page content streams) are not exhaustively cataloged
// in JSON inventory. UniqueReachableImagesCount counts distinct reachable image XObjects only.
// Inline image resources are constrained by process resource limits (memory/CPU/timeouts).
// Counts must NEVER be presented as the count of optimized images.
type DocumentInventory struct {
	UniqueReachableImagesCount int            `json:"unique_reachable_images_count"`
	TotalPageImageReferences   int            `json:"total_page_image_references"`
	MaxImagePixels              int64          `json:"max_image_pixels"`
	PixelOverflowDetected       bool           `json:"pixel_overflow_detected"`
	DetectedColorSpaces        []string       `json:"detected_color_spaces"`
	DetectedFilters            []string       `json:"detected_filters"`
	Images                     []ImageSummary `json:"images"`
}

// GSEligibility indicates whether a PDF is eligible for Ghostscript lossy rewrite
// or must fall back to lossless/original.
type GSEligibility struct {
	Eligible bool     `json:"eligible"`
	Warnings []string `json:"warnings"`
}

// ExtractInventory inspects the document structure and builds the resource inventory.
func ExtractInventory(doc *qpdf.JSONOutput) *DocumentInventory {
	inv := &DocumentInventory{
		Images:              make([]ImageSummary, 0),
		DetectedColorSpaces: make([]string, 0),
		DetectedFilters:     make([]string, 0),
	}

	if doc == nil {
		return inv
	}

	unique_map := make(map[string]ImageSummary)
	color_space_set := make(map[string]bool)
	filter_set := make(map[string]bool)

	// 1. Traverse page image references
	for _, page_info := range doc.Pages {
		for _, img := range page_info.Images {
			inv.TotalPageImageReferences++

			norm_id, err := qpdf.NormalizeObjectID(img.Object)
			if err != nil {
				norm_id = img.Object
			}

			if _, exists := unique_map[norm_id]; exists {
				continue
			}

			pixels, overflow := calculatePixels(img.Width, img.Height)
			if overflow {
				inv.PixelOverflowDetected = true
			}
			if pixels > inv.MaxImagePixels {
				inv.MaxImagePixels = pixels
			}

			cs := normalizeColorSpace(doc, img.ColorSpace)
			if cs != "" {
				color_space_set[cs] = true
			}

			var normalized_filters []string
			for _, f := range img.Filter {
				f_clean := strings.TrimPrefix(strings.TrimSpace(f), "/")
				if f_clean != "" {
					normalized_filters = append(normalized_filters, f_clean)
					filter_set[strings.ToLower(f_clean)] = true
				}
			}

			// Check Mask / SMask on the image object dictionary
			has_mask := false
			has_smask := false
			if obj, found := doc.FindObject(img.Object); found {
				if dict := obj.GetDict(); dict != nil {
					if _, ok := dict["/Mask"]; ok {
						has_mask = true
					}
					if _, ok := dict["/SMask"]; ok {
						has_smask = true
					}
				}
			}

			summary := ImageSummary{
				ObjectID:         norm_id,
				Name:             img.Name,
				Width:            img.Width,
				Height:           img.Height,
				Pixels:           pixels,
				BitsPerComponent: img.BitsPerComponent,
				ColorSpace:       cs,
				Filters:          normalized_filters,
				HasMask:          has_mask,
				HasSMask:         has_smask,
			}
			unique_map[norm_id] = summary
		}
	}

	// 2. Also check any XObjects with Subtype /Image not directly listed in pages.images
	for _, section := range doc.QPDF {
		for obj_key, raw_obj := range section {
			if !strings.HasPrefix(obj_key, "obj:") {
				continue
			}
			var entry qpdf.ObjectEntry
			if json.Unmarshal(raw_obj, &entry) != nil {
				continue
			}
			dict := entry.GetDict()
			if dict == nil {
				continue
			}

			var subtype_name string
			if raw_st, ok := dict["/Subtype"]; ok {
				_ = json.Unmarshal(raw_st, &subtype_name)
			}
			if subtype_name != "/Image" {
				continue
			}

			norm_id, err := qpdf.NormalizeObjectID(obj_key)
			if err != nil {
				norm_id = obj_key
			}
			if _, exists := unique_map[norm_id]; exists {
				continue
			}

			var w, h, bpc int
			if raw_w, ok := dict["/Width"]; ok {
				_ = json.Unmarshal(raw_w, &w)
			}
			if raw_h, ok := dict["/Height"]; ok {
				_ = json.Unmarshal(raw_h, &h)
			}
			if raw_bpc, ok := dict["/BitsPerComponent"]; ok {
				_ = json.Unmarshal(raw_bpc, &bpc)
			}

			pixels, overflow := calculatePixels(w, h)
			if overflow {
				inv.PixelOverflowDetected = true
			}
			if pixels > inv.MaxImagePixels {
				inv.MaxImagePixels = pixels
			}

			var cs string
			if raw_cs, ok := dict["/ColorSpace"]; ok {
				cs = normalizeColorSpace(doc, raw_cs)
				if cs != "" {
					color_space_set[cs] = true
				}
			}

			var filters []string
			if raw_f, ok := dict["/Filter"]; ok {
				var single_f string
				if json.Unmarshal(raw_f, &single_f) == nil {
					f_clean := strings.TrimPrefix(strings.TrimSpace(single_f), "/")
					if f_clean != "" {
						filters = append(filters, f_clean)
						filter_set[strings.ToLower(f_clean)] = true
					}
				} else {
					var multi_f []string
					if json.Unmarshal(raw_f, &multi_f) == nil {
						for _, mf := range multi_f {
							f_clean := strings.TrimPrefix(strings.TrimSpace(mf), "/")
							if f_clean != "" {
								filters = append(filters, f_clean)
								filter_set[strings.ToLower(f_clean)] = true
							}
						}
					}
				}
			}

			has_mask := false
			has_smask := false
			if _, ok := dict["/Mask"]; ok {
				has_mask = true
			}
			if _, ok := dict["/SMask"]; ok {
				has_smask = true
			}

			summary := ImageSummary{
				ObjectID:         norm_id,
				Name:             "",
				Width:            w,
				Height:           h,
				Pixels:           pixels,
				BitsPerComponent: bpc,
				ColorSpace:       cs,
				Filters:          filters,
				HasMask:          has_mask,
				HasSMask:         has_smask,
			}
			unique_map[norm_id] = summary
		}
	}

	for _, s := range unique_map {
		inv.Images = append(inv.Images, s)
	}
	sort.Slice(inv.Images, func(i, j int) bool {
		return inv.Images[i].ObjectID < inv.Images[j].ObjectID
	})

	inv.UniqueReachableImagesCount = len(inv.Images)

	for cs := range color_space_set {
		inv.DetectedColorSpaces = append(inv.DetectedColorSpaces, cs)
	}
	sort.Strings(inv.DetectedColorSpaces)

	for f := range filter_set {
		inv.DetectedFilters = append(inv.DetectedFilters, f)
	}
	sort.Strings(inv.DetectedFilters)

	return inv
}

// AssessGSEligibility checks whether the document is eligible for Ghostscript rewrite
// or should fall back with appropriate warnings.
func AssessGSEligibility(inv *DocumentInventory) *GSEligibility {
	res := &GSEligibility{
		Eligible: true,
		Warnings: make([]string, 0),
	}

	if inv == nil {
		return res
	}

	warning_set := make(map[string]bool)

	// 1. Pixel limit check
	if inv.PixelOverflowDetected || inv.MaxImagePixels > MaxImagePixelsCap {
		warning_set[WarningImagePixelLimit] = true
	}

	// 2. Unknown filter check
	for _, img := range inv.Images {
		for _, f := range img.Filters {
			f_lower := strings.ToLower(f)
			if f_lower != "" && !supportedFilters[f_lower] {
				warning_set[WarningUnsupportedImageFilter] = true
			}
		}
	}

	// 3. Complex colors check
	for _, cs := range inv.DetectedColorSpaces {
		if isComplexColorSpace(cs) {
			warning_set[WarningComplexColorPreserved] = true
		}
	}
	for _, img := range inv.Images {
		if isComplexColorSpace(img.ColorSpace) {
			warning_set[WarningComplexColorPreserved] = true
		}
	}

	var ordered_warnings []string
	warning_order := []string{
		WarningComplexColorPreserved,
		WarningUnsupportedImageFilter,
		WarningImagePixelLimit,
	}
	for _, w := range warning_order {
		if warning_set[w] {
			ordered_warnings = append(ordered_warnings, w)
		}
	}

	res.Warnings = ordered_warnings
	if len(res.Warnings) > 0 {
		res.Eligible = false
	}

	return res
}

func calculatePixels(w, h int) (int64, bool) {
	if w <= 0 || h <= 0 {
		return 0, false
	}
	w64 := int64(w)
	h64 := int64(h)
	if w64 > math.MaxInt64/h64 {
		return 0, true
	}
	return w64 * h64, false
}

func normalizeColorSpace(doc *qpdf.JSONOutput, raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var cs_name string
	if json.Unmarshal(raw, &cs_name) == nil {
		if strings.HasPrefix(cs_name, "obj:") {
			if obj, found := doc.FindObject(cs_name); found {
				return normalizeColorSpace(doc, obj.RawValue)
			}
		}
		return strings.TrimPrefix(cs_name, "/")
	}

	var cs_arr []json.RawMessage
	if json.Unmarshal(raw, &cs_arr) == nil && len(cs_arr) > 0 {
		var first_name string
		if json.Unmarshal(cs_arr[0], &first_name) == nil {
			first_clean := strings.TrimPrefix(first_name, "/")
			if len(cs_arr) > 1 && strings.EqualFold(first_clean, "Indexed") {
				base_cs := normalizeColorSpace(doc, cs_arr[1])
				return "Indexed:" + base_cs
			}
			return first_clean
		}
	}

	return string(raw)
}

func isComplexColorSpace(cs string) bool {
	upper := strings.ToUpper(cs)
	if strings.Contains(upper, "CMYK") ||
		strings.Contains(upper, "SEPARATION") ||
		strings.Contains(upper, "DEVICEN") ||
		strings.Contains(upper, "ICCBASED") ||
		strings.Contains(upper, "CALRGB") ||
		strings.Contains(upper, "CALGRAY") ||
		strings.Contains(upper, "LAB") {
		return true
	}
	return false
}
