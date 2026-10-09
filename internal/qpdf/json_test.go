package qpdf

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestParseJSON(t *testing.T) {
	raw := `{
		"version": 2,
		"pages": [
			{
				"pageposfrom1": 1,
				"object": "3 0 R",
				"images": [
					{
						"name": "/Im0",
						"object": "5 0 R",
						"width": 1200,
						"height": 1200,
						"bitspercomponent": 8,
						"filter": ["/DCTDecode"]
					}
				],
				"outlines": [
					{
						"title": "Probe bookmark",
						"dest": ["3 0 R", "/Fit"],
						"object": "9 0 R"
					}
				]
			}
		],
		"qpdf": [
			{
				"trailer": {
					"value": {
						"/Root": "1 0 R"
					}
				},
				"obj:1 0 R": {
					"value": {
						"/Type": "/Catalog",
						"/Pages": "2 0 R"
					}
				},
				"obj:3 0 R": {
					"value": {
						"/Type": "/Page",
						"/MediaBox": [0, 0, 612.0001, 792.0001]
					}
				},
				"obj:4 0 R": {
					"stream": {
						"dict": {
							"/Length": 120
						}
					}
				}
			}
		]
	}`

	out, err := ParseJSON([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Version != 2 {
		t.Fatalf("expected version 2, got %d", out.Version)
	}
	if len(out.Pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(out.Pages))
	}
	if len(out.Pages[0].Images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(out.Pages[0].Images))
	}
	if out.Pages[0].Images[0].Width != 1200 {
		t.Fatalf("expected width 1200, got %d", out.Pages[0].Images[0].Width)
	}

	// Test FindObject with different representations
	for _, ref := range []string{"obj:1 0 R", "1 0 R", "1,0", "1"} {
		obj, found := out.FindObject(ref)
		if !found || obj == nil {
			t.Fatalf("expected to find object with ref %q", ref)
		}
		dict := obj.GetDict()
		if dict == nil {
			t.Fatalf("expected dictionary for object %q", ref)
		}
		var type_val string
		_ = json.Unmarshal(dict["/Type"], &type_val)
		if type_val != "/Catalog" {
			t.Fatalf("expected /Catalog for object %q, got %s", ref, type_val)
		}
	}

	// Test stream dict
	stream_obj, found := out.FindObject("4")
	if !found {
		t.Fatalf("expected to find stream object 4")
	}
	dict := stream_obj.GetDict()
	if dict == nil || len(dict["/Length"]) == 0 {
		t.Fatalf("expected stream dictionary with /Length")
	}

	// Test Root discovery
	root_dict, ok := out.Root()
	if !ok || root_dict == nil {
		t.Fatalf("expected Root catalog dictionary")
	}

	// Test Trailer discovery
	trailer, ok := out.Trailer()
	if !ok || trailer == nil {
		t.Fatalf("expected trailer dictionary")
	}
}

func TestParseJSONValidationLimits(t *testing.T) {
	t.Run("invalid version", func(t *testing.T) {
		raw := `{"version": 1, "pages": [], "qpdf": []}`
		_, err := ParseJSON([]byte(raw))
		if err == nil {
			t.Fatalf("expected error for unsupported version 1")
		}
	})

	t.Run("too many pages", func(t *testing.T) {
		var pages []string
		for i := 1; i <= MaxDocumentPages+1; i++ {
			pages = append(pages, fmt.Sprintf(`{"pageposfrom1": %d, "object": "%d 0 R"}`, i, i))
		}
		raw := fmt.Sprintf(`{"version": 2, "pages": [%s], "qpdf": []}`, strings.Join(pages, ","))
		_, err := ParseJSON([]byte(raw))
		if err == nil {
			t.Fatalf("expected error for exceeding page count limit")
		}
	})
}

func TestThousandPageBoundary(t *testing.T) {
	for _, count := range []int{552, 1000, 1001} {
		pages := make([]PageInfo, count)
		data, err := json.Marshal(JSONOutput{Version: 2, Pages: pages})
		if err != nil {
			t.Fatal(err)
		}
		_, err = ParseJSON(data)
		if count <= 1000 && err != nil {
			t.Fatalf("%d pages refused: %v", count, err)
		}
		if count > 1000 && !errors.Is(err, ErrDocumentLimit) {
			t.Fatalf("%d pages accepted: %v", count, err)
		}
	}
}
