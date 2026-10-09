package pdf

import (
	"context"
	"strings"
	"testing"

	"pdf-golang-zip/internal/qpdf"
	"pdf-golang-zip/internal/worker"
)

type fakeRunner struct {
	handler func(ctx context.Context, opts worker.CommandOptions, tool string, args ...string) (*worker.Result, error)
}

func (f *fakeRunner) Run(ctx context.Context, tool string, args ...string) (*worker.Result, error) {
	return f.RunWithOptions(ctx, worker.CommandOptions{}, tool, args...)
}

func (f *fakeRunner) RunWithOptions(
	ctx context.Context,
	opts worker.CommandOptions,
	tool string,
	args ...string,
) (*worker.Result, error) {
	return f.handler(ctx, opts, tool, args...)
}

func TestAnalyzeDocumentEncrypted(t *testing.T) {
	runner := &fakeRunner{
		handler: func(ctx context.Context, opts worker.CommandOptions, tool string, args ...string) (*worker.Result, error) {
			if len(args) > 0 && args[0] == "--is-encrypted" {
				return &worker.Result{ExitCode: 0}, nil
			}
			return &worker.Result{ExitCode: 0}, nil
		},
	}

	client := qpdf.NewClient("qpdf", runner)
	analysis, err := AnalyzeDocument(context.Background(), client, "encrypted.pdf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if analysis.IsAllowed() {
		t.Fatalf("expected encrypted document to not be allowed")
	}
	if analysis.PolicyResult.RefusalCode != CodeEncrypted {
		t.Fatalf("expected refusal code %s, got %s", CodeEncrypted, analysis.PolicyResult.RefusalCode)
	}
}

func TestAnalyzeDocumentCleanFlow(t *testing.T) {
	sample_json := `{
		"version": 2,
		"pages": [
			{
				"pageposfrom1": 1,
				"object": "3 0 R",
				"images": [
					{
						"name": "/Im0",
						"object": "5 0 R",
						"width": 300,
						"height": 300,
						"bitspercomponent": 8,
						"colorspace": "/DeviceRGB",
						"filter": ["/DCTDecode"]
					}
				]
			}
		],
		"qpdf": [
			{
				"trailer": {"value":{"/Root":"1 0 R"}},
				"obj:1 0 R": {"value":{"/Type":"/Catalog","/Pages":"2 0 R"}},
				"obj:2 0 R": {"value":{"/Type":"/Pages","/Kids":["3 0 R"],"/Count":1}},
				"obj:3 0 R": {"value":{"/Type":"/Page","/MediaBox":[0,0,216,216],"/CropBox":[0,0,216,216]}},
				"obj:5 0 R": {"stream":{"dict":{"/Width":300,"/Height":300,"/BitsPerComponent":8,"/ColorSpace":"/DeviceRGB","/Filter":"/DCTDecode"}}}
			}
		]
	}`

	runner := &fakeRunner{
		handler: func(ctx context.Context, opts worker.CommandOptions, tool string, args ...string) (*worker.Result, error) {
			// is-encrypted check
			if len(args) > 0 && args[0] == "--is-encrypted" {
				return &worker.Result{ExitCode: 2}, nil // not encrypted
			}
			// analyze check
			for _, a := range args {
				if strings.HasPrefix(a, "--json=2") {
					return &worker.Result{
						ExitCode: 0,
						Stdout:   []byte(sample_json),
					}, nil
				}
			}
			return &worker.Result{ExitCode: 0}, nil
		},
	}

	client := qpdf.NewClient("qpdf", runner)
	analysis, err := AnalyzeDocument(context.Background(), client, "clean.pdf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !analysis.IsAllowed() {
		t.Fatalf("expected clean doc to be allowed, got refusal: %+v", analysis.PolicyResult)
	}

	if !analysis.IsEligibleForGS() {
		t.Fatalf("expected clean doc to be GS-eligible, got warnings: %v", analysis.Warnings())
	}

	if analysis.Inventory.UniqueReachableImagesCount != 1 {
		t.Fatalf("expected 1 image in inventory, got %d", analysis.Inventory.UniqueReachableImagesCount)
	}

	if analysis.Snapshot.PageCount != 1 {
		t.Fatalf("expected 1 page in snapshot, got %d", analysis.Snapshot.PageCount)
	}
}

func TestAnalyzeDocumentRealProbeFixture(t *testing.T) {
	fixture_json := `{
		"version": 2,
		"pages": [
			{
				"contents": ["4 0 R"],
				"images": [
					{
						"bitspercomponent": 8,
						"colorspace": "/DeviceRGB",
						"filter": ["/DCTDecode"],
						"height": 300,
						"name": "/Im0",
						"object": "5 0 R",
						"width": 300
					}
				],
				"object": "3 0 R",
				"pageposfrom1": 1
			}
		],
		"acroform": {"fields": [], "hasacroform": false, "needappearances": false},
		"attachments": {},
		"encrypt": {"encrypted": false},
		"qpdf": [
			{
				"obj:1 0 R": {"value": {"/Outlines": "8 0 R", "/Pages": "2 0 R", "/Type": "/Catalog"}},
				"obj:2 0 R": {"value": {"/Count": 1, "/Kids": ["3 0 R"], "/Type": "/Pages"}},
				"obj:3 0 R": {
					"value": {
						"/Annots": ["7 0 R"],
						"/Contents": "4 0 R",
						"/CropBox": [0, 0, 216, 216],
						"/MediaBox": [0, 0, 216, 216],
						"/Type": "/Page"
					}
				},
				"obj:4 0 R": {"stream": {"dict": {"/Length": 79}}},
				"obj:5 0 R": {
					"stream": {
						"dict": {
							"/BitsPerComponent": 8,
							"/ColorSpace": "/DeviceRGB",
							"/Filter": "/DCTDecode",
							"/Height": 300,
							"/Subtype": "/Image",
							"/Type": "/XObject",
							"/Width": 300
						}
					}
				},
				"obj:7 0 R": {
					"value": {
						"/A": {"/S": "/URI", "/URI": "u:https://example.org/stage-01"},
						"/Rect": [36, 190, 180, 208],
						"/Subtype": "/Link",
						"/Type": "/Annot"
					}
				},
				"obj:8 0 R": {"value": {"/Count": 1, "/First": "9 0 R", "/Type": "/Outlines"}},
				"obj:9 0 R": {"value": {"/Dest": ["3 0 R", "/Fit"], "/Title": "u:Probe bookmark"}},
				"trailer": {"value": {"/Root": "1 0 R", "/Size": 10}}
			}
		]
	}`

	runner := &fakeRunner{
		handler: func(ctx context.Context, opts worker.CommandOptions, tool string, args ...string) (*worker.Result, error) {
			if len(args) > 0 && args[0] == "--is-encrypted" {
				return &worker.Result{ExitCode: 2}, nil
			}
			for _, a := range args {
				if strings.HasPrefix(a, "--json=2") {
					return &worker.Result{
						ExitCode: 0,
						Stdout:   []byte(fixture_json),
					}, nil
				}
			}
			return &worker.Result{ExitCode: 0}, nil
		},
	}

	client := qpdf.NewClient("qpdf", runner)
	analysis, err := AnalyzeDocument(context.Background(), client, "jpeg-150dpi.pdf")
	if err != nil {
		t.Fatalf("AnalyzeDocument failed: %v", err)
	}

	if !analysis.IsAllowed() {
		t.Fatalf("expected real fixture to be allowed, got refusal: %+v", analysis.PolicyResult)
	}

	if !analysis.IsEligibleForGS() {
		t.Fatalf("expected real fixture to be GS eligible, got warnings: %v", analysis.Warnings())
	}

	if analysis.Inventory.UniqueReachableImagesCount != 1 {
		t.Fatalf("expected 1 unique reachable image, got %d", analysis.Inventory.UniqueReachableImagesCount)
	}

	if analysis.Inventory.MaxImagePixels != 90000 {
		t.Fatalf("expected 90000 pixels, got %d", analysis.Inventory.MaxImagePixels)
	}

	snap := analysis.Snapshot
	if snap.PageCount != 1 {
		t.Fatalf("expected 1 page, got %d", snap.PageCount)
	}

	p1 := snap.Pages[0]
	if p1.Geometry.MediaBox != [4]float64{0, 0, 216, 216} {
		t.Fatalf("unexpected MediaBox: %v", p1.Geometry.MediaBox)
	}

	if len(p1.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(p1.Links))
	}
	if p1.Links[0].TargetType != "URI" || p1.Links[0].URI != "https://example.org/stage-01" {
		t.Fatalf("unexpected link in snapshot: %+v", p1.Links[0])
	}

	if len(snap.Outlines) != 1 {
		t.Fatalf("expected 1 outline, got %d", len(snap.Outlines))
	}
	if snap.Outlines[0].Title != "Probe bookmark" || snap.Outlines[0].DestPage != 1 || snap.Outlines[0].DestFit != "Fit" {
		t.Fatalf("unexpected outline in snapshot: %+v", snap.Outlines[0])
	}
}
