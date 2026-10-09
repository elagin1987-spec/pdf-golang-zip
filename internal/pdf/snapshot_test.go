package pdf

import (
	"encoding/json"
	"testing"

	"pdf-golang-zip/internal/qpdf"
)

func TestSnapshotExtractionAndTolerance(t *testing.T) {
	doc := &qpdf.JSONOutput{
		Version: 2,
		Pages: []qpdf.PageInfo{
			{
				PagePosFrom1: 1,
				Object:       "3 0 R",
				Contents:     []string{"4 0 R"},
			},
		},
		QPDF: []map[string]json.RawMessage{
			{
				"trailer": json.RawMessage(`{"/Root":"1 0 R"}`),
				"obj:1 0 R": json.RawMessage(
					`{"value":{"/Type":"/Catalog","/Pages":"2 0 R","/Outlines":"8 0 R"}}`,
				),
				"obj:2 0 R": json.RawMessage(
					`{"value":{"/Type":"/Pages","/Kids":["3 0 R"],"/Count":1}}`,
				),
				"obj:3 0 R": json.RawMessage(
					`{"value":{"/Type":"/Page","/MediaBox":[0,0,612,792],"/CropBox":[10,10,600,780],"/Rotate":90,"/UserUnit":1.5,"/Annots":["7 0 R"]}}`,
				),
				"obj:7 0 R": json.RawMessage(
					`{"value":{"/Type":"/Annot","/Subtype":"/Link","/Rect":[20,20,200,50],"/A":{"/S":"/URI","/URI":"u:https://example.com"}}}`,
				),
				"obj:8 0 R": json.RawMessage(
					`{"value":{"/Type":"/Outlines","/First":"9 0 R","/Count":1}}`,
				),
				"obj:9 0 R": json.RawMessage(
					`{"value":{"/Title":"u:Introduction","/Dest":["3 0 R","/Fit"]}}`,
				),
			},
		},
	}

	snap, err := BuildSnapshot(doc)
	if err != nil {
		t.Fatalf("BuildSnapshot failed: %v", err)
	}

	if snap.PageCount != 1 {
		t.Fatalf("expected 1 page, got %d", snap.PageCount)
	}

	p1 := snap.Pages[0]
	if p1.Geometry.MediaBox != [4]float64{0, 0, 612, 792} {
		t.Fatalf("unexpected MediaBox: %v", p1.Geometry.MediaBox)
	}
	if p1.Geometry.CropBox != [4]float64{10, 10, 600, 780} {
		t.Fatalf("unexpected CropBox: %v", p1.Geometry.CropBox)
	}
	if p1.Geometry.Rotate != 90 {
		t.Fatalf("unexpected Rotate: %d", p1.Geometry.Rotate)
	}
	if p1.Geometry.UserUnit != 1.5 {
		t.Fatalf("unexpected UserUnit: %f", p1.Geometry.UserUnit)
	}

	if len(p1.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(p1.Links))
	}
	link := p1.Links[0]
	if link.TargetType != "URI" || link.URI != "https://example.com" {
		t.Fatalf("unexpected link properties: %+v", link)
	}

	if len(snap.Outlines) != 1 {
		t.Fatalf("expected 1 outline, got %d", len(snap.Outlines))
	}
	outline := snap.Outlines[0]
	if outline.Title != "Introduction" || outline.DestPage != 1 || outline.DestFit != "Fit" {
		t.Fatalf("unexpected outline properties: %+v", outline)
	}

	// Compare with cloned snapshot within tolerance
	snap_clone, _ := BuildSnapshot(doc)
	// Small jitter <= 0.005 pt
	snap_clone.Pages[0].Geometry.MediaBox[2] += 0.005
	snap_clone.Pages[0].Geometry.CropBox[3] -= 0.005
	snap_clone.Pages[0].Geometry.UserUnit += 0.002
	snap_clone.Pages[0].Links[0].Rect[0] += 0.008

	equal, mismatches := CompareSnapshots(snap, snap_clone)
	if !equal {
		t.Fatalf("expected snapshots within 0.01 pt tolerance to match, got: %v", mismatches)
	}

	// Greater jitter > 0.01 pt should fail
	snap_clone.Pages[0].Geometry.MediaBox[2] += 0.02
	equal2, mismatches2 := CompareSnapshots(snap, snap_clone)
	if equal2 || len(mismatches2) == 0 {
		t.Fatalf("expected tolerance violation to fail comparison, got equal: %v", equal2)
	}
}

func TestSnapshotObjectRenumberingInvariance(t *testing.T) {
	// Original document with objects 1..9
	doc_orig := &qpdf.JSONOutput{
		Version: 2,
		Pages: []qpdf.PageInfo{
			{
				PagePosFrom1: 1,
				Object:       "3 0 R",
			},
			{
				PagePosFrom1: 2,
				Object:       "6 0 R",
			},
		},
		QPDF: []map[string]json.RawMessage{
			{
				"trailer": json.RawMessage(`{"/Root":"1 0 R"}`),
				"obj:1 0 R": json.RawMessage(
					`{"value":{"/Type":"/Catalog","/Pages":"2 0 R","/Outlines":"8 0 R"}}`,
				),
				"obj:2 0 R": json.RawMessage(
					`{"value":{"/Type":"/Pages","/Kids":["3 0 R","6 0 R"],"/Count":2}}`,
				),
				"obj:3 0 R": json.RawMessage(
					`{"value":{"/Type":"/Page","/MediaBox":[0,0,612,792],"/Annots":["7 0 R"]}}`,
				),
				"obj:6 0 R": json.RawMessage(
					`{"value":{"/Type":"/Page","/MediaBox":[0,0,612,792]}}`,
				),
				"obj:7 0 R": json.RawMessage(
					`{"value":{"/Type":"/Annot","/Subtype":"/Link","/Rect":[10,10,100,50],"/Dest":["6 0 R","/Fit"]}}`,
				),
				"obj:8 0 R": json.RawMessage(
					`{"value":{"/Type":"/Outlines","/First":"9 0 R","/Count":1}}`,
				),
				"obj:9 0 R": json.RawMessage(
					`{"value":{"/Title":"u:Chapter 2","/Dest":["6 0 R","/Fit"]}}`,
				),
			},
		},
	}

	// Rewritten candidate with completely different object numbering (e.g. Ghostscript output)
	// Page 1 is now 15 0 R, Page 2 is now 25 0 R
	// Link is now 35 0 R, Outlines is 45 0 R, outline item is 55 0 R
	doc_rewritten := &qpdf.JSONOutput{
		Version: 2,
		Pages: []qpdf.PageInfo{
			{
				PagePosFrom1: 1,
				Object:       "15 0 R",
			},
			{
				PagePosFrom1: 2,
				Object:       "25 0 R",
			},
		},
		QPDF: []map[string]json.RawMessage{
			{
				"trailer": json.RawMessage(`{"/Root":"10 0 R"}`),
				"obj:10 0 R": json.RawMessage(
					`{"value":{"/Type":"/Catalog","/Pages":"20 0 R","/Outlines":"45 0 R"}}`,
				),
				"obj:20 0 R": json.RawMessage(
					`{"value":{"/Type":"/Pages","/Kids":["15 0 R","25 0 R"],"/Count":2}}`,
				),
				"obj:15 0 R": json.RawMessage(
					`{"value":{"/Type":"/Page","/MediaBox":[0,0,612,792],"/Annots":["35 0 R"]}}`,
				),
				"obj:25 0 R": json.RawMessage(
					`{"value":{"/Type":"/Page","/MediaBox":[0,0,612,792]}}`,
				),
				"obj:35 0 R": json.RawMessage(
					`{"value":{"/Type":"/Annot","/Subtype":"/Link","/Rect":[10,10,100,50],"/Dest":["25 0 R","/Fit"]}}`,
				),
				"obj:45 0 R": json.RawMessage(
					`{"value":{"/Type":"/Outlines","/First":"55 0 R","/Count":1}}`,
				),
				"obj:55 0 R": json.RawMessage(
					`{"value":{"/Title":"u:Chapter 2","/Dest":["25 0 R","/Fit"]}}`,
				),
			},
		},
	}

	snap_orig, err1 := BuildSnapshot(doc_orig)
	if err1 != nil {
		t.Fatalf("BuildSnapshot orig failed: %v", err1)
	}

	snap_rewritten, err2 := BuildSnapshot(doc_rewritten)
	if err2 != nil {
		t.Fatalf("BuildSnapshot rewritten failed: %v", err2)
	}

	// Verify both resolved destinations point to page 2 despite different object IDs
	if snap_orig.Pages[0].Links[0].DestPage != 2 {
		t.Fatalf("expected orig link DestPage=2, got %d", snap_orig.Pages[0].Links[0].DestPage)
	}
	if snap_rewritten.Pages[0].Links[0].DestPage != 2 {
		t.Fatalf("expected rewritten link DestPage=2, got %d", snap_rewritten.Pages[0].Links[0].DestPage)
	}
	if snap_orig.Outlines[0].DestPage != 2 {
		t.Fatalf("expected orig outline DestPage=2, got %d", snap_orig.Outlines[0].DestPage)
	}
	if snap_rewritten.Outlines[0].DestPage != 2 {
		t.Fatalf("expected rewritten outline DestPage=2, got %d", snap_rewritten.Outlines[0].DestPage)
	}

	equal, mismatches := CompareSnapshots(snap_orig, snap_rewritten)
	if !equal {
		t.Fatalf("expected renumbered document snapshots to be semantically equivalent, got mismatches: %v", mismatches)
	}
}

func TestSnapshotMismatches(t *testing.T) {
	orig := &DocumentSnapshot{
		PageCount: 1,
		Pages: []PageSnapshot{
			{
				PagePosFrom1: 1,
				Geometry: PageGeometry{
					MediaBox: [4]float64{0, 0, 100, 100},
					CropBox:  [4]float64{0, 0, 100, 100},
					Rotate:   0,
					UserUnit: 1.0,
				},
				Links: []LinkSnapshot{
					{
						Rect:       [4]float64{10, 10, 20, 20},
						TargetType: "URI",
						URI:        "https://example.com/a",
					},
				},
			},
		},
		Outlines: []OutlineSnapshot{
			{
				Title:    "Title A",
				DestPage: 1,
			},
		},
	}

	// 1. Different page count
	cand1 := &DocumentSnapshot{PageCount: 2}
	eq1, _ := CompareSnapshots(orig, cand1)
	if eq1 {
		t.Fatalf("expected page count mismatch to fail")
	}

	// 2. Different rotate
	cand2 := &DocumentSnapshot{
		PageCount: 1,
		Pages: []PageSnapshot{
			{
				PagePosFrom1: 1,
				Geometry: PageGeometry{
					MediaBox: [4]float64{0, 0, 100, 100},
					CropBox:  [4]float64{0, 0, 100, 100},
					Rotate:   90,
					UserUnit: 1.0,
				},
			},
		},
	}
	eq2, _ := CompareSnapshots(orig, cand2)
	if eq2 {
		t.Fatalf("expected rotate mismatch to fail")
	}

	// 3. Different link target
	cand3 := &DocumentSnapshot{
		PageCount: 1,
		Pages: []PageSnapshot{
			{
				PagePosFrom1: 1,
				Geometry: orig.Pages[0].Geometry,
				Links: []LinkSnapshot{
					{
						Rect:       [4]float64{10, 10, 20, 20},
						TargetType: "URI",
						URI:        "https://example.com/b",
					},
				},
			},
		},
		Outlines: orig.Outlines,
	}
	eq3, _ := CompareSnapshots(orig, cand3)
	if eq3 {
		t.Fatalf("expected link URI mismatch to fail")
	}
}
