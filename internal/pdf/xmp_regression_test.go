package pdf

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"testing"

	"pdf-golang-zip/internal/qpdf"
	"pdf-golang-zip/internal/worker"
)

func TestMetadataReadRequired(t *testing.T) {
	readFailure := errors.New("private metadata read failure")
	for _, tt := range []struct {
		name, metadata, xmp string
		readErr             error
		wantErr             error
		code                string
	}{
		{name: "absent"},
		{name: "plain", metadata: `"10 0 R"`, xmp: "<xmp/>"},
		{name: "pdfa", metadata: `"10 0 R"`, xmp: `<xmp xmlns:pdfaid="urn:pdfa"/>`, code: CodePDFA},
		{name: "cap", metadata: `"10 0 R"`, readErr: qpdf.ErrStreamExceedsLimit, code: CodePDFA},
		{name: "read error", metadata: `"10 0 R"`, readErr: readFailure, wantErr: readFailure},
		{name: "missing tool", metadata: `"10 0 R"`, readErr: exec.ErrNotFound, wantErr: exec.ErrNotFound},
		{name: "canceled", metadata: `"10 0 R"`, readErr: context.Canceled, wantErr: context.Canceled},
		{name: "invalid reference", metadata: `123`, wantErr: qpdf.ErrInvalidObjectID},
		{name: "invalid string", metadata: `"not-a-reference"`, wantErr: qpdf.ErrInvalidObjectID},
	} {
		t.Run(tt.name, func(t *testing.T) {
			doc := createBaseCleanDoc()
			if tt.metadata != "" {
				doc.QPDF[0]["obj:1 0 R"] = json.RawMessage(`{"value":{"/Type":"/Catalog","/Pages":"2 0 R","/Metadata":` + tt.metadata + `}}`)
			}
			data, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			reads := 0
			runner := &fakeRunner{handler: func(_ context.Context, _ worker.CommandOptions, _ string, args ...string) (*worker.Result, error) {
				switch args[0] {
				case "--is-encrypted":
					return &worker.Result{ExitCode: 2}, nil
				case "--json=2":
					return &worker.Result{Stdout: data}, nil
				default:
					reads++
					return &worker.Result{Stdout: []byte(tt.xmp)}, tt.readErr
				}
			}}
			analysis, err := AnalyzeDocument(context.Background(), qpdf.NewClient("fake", runner), "owned.pdf")
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) || analysis != nil {
					t.Fatalf("missing error chain: %v / %+v", err, analysis)
				}
				return
			}
			if err != nil || analysis.PolicyResult.RefusalCode != tt.code {
				t.Fatalf("%v / %+v", err, analysis)
			}
			if tt.metadata == "" && reads != 0 {
				t.Fatal("absent XMP was read")
			}
		})
	}
}
