package pdf

import (
	"encoding/json"
	"testing"

	"pdf-golang-zip/internal/qpdf"
)

func TestExtractInventoryCleanDoc(t *testing.T) {
	doc := &qpdf.JSONOutput{
		Version: 2,
		Pages: []qpdf.PageInfo{
			{
				PagePosFrom1: 1,
				Object:       "3 0 R",
				Images: []qpdf.ImageInfo{
					{
						Object:           "5 0 R",
						Name:             "/Im0",
						Width:            1000,
						Height:           800,
						BitsPerComponent: 8,
						ColorSpace:       json.RawMessage(`"/DeviceRGB"`),
						Filter:           []string{"/DCTDecode"},
					},
					{
						// Duplicate reference on the same page
						Object:           "5 0 R",
						Name:             "/Im0",
						Width:            1000,
						Height:           800,
						BitsPerComponent: 8,
						ColorSpace:       json.RawMessage(`"/DeviceRGB"`),
						Filter:           []string{"/DCTDecode"},
					},
				},
			},
			{
				PagePosFrom1: 2,
				Object:       "6 0 R",
				Images: []qpdf.ImageInfo{
					{
						// Shared reference on second page
						Object:           "5 0 R",
						Name:             "/Im0",
						Width:            1000,
						Height:           800,
						BitsPerComponent: 8,
						ColorSpace:       json.RawMessage(`"/DeviceRGB"`),
						Filter:           []string{"/DCTDecode"},
					},
					{
						Object:           "7 0 R",
						Name:             "/Im1",
						Width:            400,
						Height:           300,
						BitsPerComponent: 8,
						ColorSpace:       json.RawMessage(`"/DeviceGray"`),
						Filter:           []string{"/FlateDecode"},
					},
				},
			},
		},
		QPDF: []map[string]json.RawMessage{
			{
				"obj:5 0 R": json.RawMessage(
					`{"stream":{"dict":{"/Width":1000,"/Height":800,"/BitsPerComponent":8,"/ColorSpace":"/DeviceRGB","/Filter":"/DCTDecode"}}}`,
				),
				"obj:7 0 R": json.RawMessage(
					`{"stream":{"dict":{"/Width":400,"/Height":300,"/BitsPerComponent":8,"/ColorSpace":"/DeviceGray","/Filter":"/FlateDecode","/Mask":"8 0 R"}}}`,
				),
			},
		},
	}

	inv := ExtractInventory(doc)

	if inv.TotalPageImageReferences != 4 {
		t.Fatalf("expected 4 total page image references, got %d", inv.TotalPageImageReferences)
	}

	if inv.UniqueReachableImagesCount != 2 {
		t.Fatalf("expected 2 unique reachable images, got %d", inv.UniqueReachableImagesCount)
	}

	if inv.MaxImagePixels != 800000 {
		t.Fatalf("expected max image pixels 800000, got %d", inv.MaxImagePixels)
	}

	if inv.PixelOverflowDetected {
		t.Fatalf("unexpected pixel overflow detected")
	}

	// Verify mask on Im1
	var img1 *ImageSummary
	for i := range inv.Images {
		if inv.Images[i].ObjectID == "7,0" {
			img1 = &inv.Images[i]
			break
		}
	}
	if img1 == nil || !img1.HasMask {
		t.Fatalf("expected image 7,0 to have HasMask=true, got %+v", img1)
	}

	// Assess eligibility
	elig := AssessGSEligibility(inv)
	if !elig.Eligible {
		t.Fatalf("expected clean images to be GS-eligible, got warnings: %v", elig.Warnings)
	}
}

func TestAssessGSEligibilityComplexColor(t *testing.T) {
	test_cases := []struct {
		name       string
		colorspace string
	}{
		{"DeviceCMYK", "/DeviceCMYK"},
		{"Separation", "Separation"},
		{"DeviceN", "DeviceN"},
		{"ICCBased", "ICCBased"},
		{"CalRGB", "CalRGB"},
		{"Lab", "Lab"},
	}

	for _, tc := range test_cases {
		t.Run(tc.name, func(t *testing.T) {
			inv := &DocumentInventory{
				UniqueReachableImagesCount: 1,
				MaxImagePixels:              1000,
				DetectedColorSpaces:        []string{tc.colorspace},
				Images: []ImageSummary{
					{
						ObjectID:    "1,0",
						Width:       100,
						Height:      10,
						Pixels:      1000,
						ColorSpace:  tc.colorspace,
						Filters:     []string{"DCTDecode"},
					},
				},
			}

			elig := AssessGSEligibility(inv)
			if elig.Eligible {
				t.Fatalf("expected complex colorspace %s to be ineligible", tc.colorspace)
			}

			found_warning := false
			for _, w := range elig.Warnings {
				if w == WarningComplexColorPreserved {
					found_warning = true
					break
				}
			}
			if !found_warning {
				t.Fatalf("expected %s in warnings, got %v", WarningComplexColorPreserved, elig.Warnings)
			}
		})
	}
}

func TestAssessGSEligibilityUnsupportedFilter(t *testing.T) {
	inv := &DocumentInventory{
		UniqueReachableImagesCount: 1,
		MaxImagePixels:              1000,
		DetectedColorSpaces:        []string{"DeviceRGB"},
		DetectedFilters:            []string{"CryptDecode"},
		Images: []ImageSummary{
			{
				ObjectID:   "1,0",
				Width:      100,
				Height:     10,
				Pixels:     1000,
				ColorSpace: "DeviceRGB",
				Filters:    []string{"CryptDecode"},
			},
		},
	}

	elig := AssessGSEligibility(inv)
	if elig.Eligible {
		t.Fatalf("expected unknown filter to be ineligible")
	}

	found_warning := false
	for _, w := range elig.Warnings {
		if w == WarningUnsupportedImageFilter {
			found_warning = true
			break
		}
	}
	if !found_warning {
		t.Fatalf("expected %s in warnings, got %v", WarningUnsupportedImageFilter, elig.Warnings)
	}
}

func TestAssessGSEligibilityPixelLimit(t *testing.T) {
	// 1. > 40 MP image (e.g. 50 MP)
	inv := &DocumentInventory{
		UniqueReachableImagesCount: 1,
		MaxImagePixels:              50_000_000,
		DetectedColorSpaces:        []string{"DeviceRGB"},
		DetectedFilters:            []string{"DCTDecode"},
		Images: []ImageSummary{
			{
				ObjectID:   "1,0",
				Width:      10000,
				Height:     5000,
				Pixels:     50_000_000,
				ColorSpace: "DeviceRGB",
				Filters:    []string{"DCTDecode"},
			},
		},
	}

	elig := AssessGSEligibility(inv)
	if elig.Eligible {
		t.Fatalf("expected >40MP image to be ineligible")
	}

	found_warning := false
	for _, w := range elig.Warnings {
		if w == WarningImagePixelLimit {
			found_warning = true
			break
		}
	}
	if !found_warning {
		t.Fatalf("expected %s in warnings, got %v", WarningImagePixelLimit, elig.Warnings)
	}

	// 2. Pixel calculation overflow
	p, overflow := calculatePixels(int(3_000_000_000_000), 10_000_000)
	if !overflow {
		t.Fatalf("expected integer overflow for large dimensions, got %d", p)
	}

	inv2 := &DocumentInventory{
		PixelOverflowDetected: true,
	}
	elig2 := AssessGSEligibility(inv2)
	if elig2.Eligible {
		t.Fatalf("expected pixel overflow to be ineligible")
	}
}

func TestAssessGSEligibilityMultipleWarningsOrder(t *testing.T) {
	inv := &DocumentInventory{
		UniqueReachableImagesCount: 1,
		MaxImagePixels:              50_000_000,
		DetectedColorSpaces:        []string{"DeviceCMYK"},
		DetectedFilters:            []string{"CustomDecode"},
		Images: []ImageSummary{
			{
				ObjectID:   "1,0",
				Width:      10000,
				Height:     5000,
				Pixels:     50_000_000,
				ColorSpace: "DeviceCMYK",
				Filters:    []string{"CustomDecode"},
			},
		},
	}

	elig := AssessGSEligibility(inv)
	if elig.Eligible {
		t.Fatalf("expected multiple violations to be ineligible")
	}

	if len(elig.Warnings) != 3 {
		t.Fatalf("expected 3 unique warnings, got %v", elig.Warnings)
	}

	expected_order := []string{
		WarningComplexColorPreserved,
		WarningUnsupportedImageFilter,
		WarningImagePixelLimit,
	}
	for i, exp := range expected_order {
		if elig.Warnings[i] != exp {
			t.Fatalf("warning at %d: expected %s, got %s", i, exp, elig.Warnings[i])
		}
	}
}
