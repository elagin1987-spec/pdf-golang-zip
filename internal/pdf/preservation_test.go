package pdf

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"pdf-golang-zip/internal/qpdf"
	"pdf-golang-zip/internal/worker"
)

func preservationDoc(t *testing.T, root, attachment, stream string) *qpdf.JSONOutput {
	t.Helper()
	doc, err := qpdf.ParseJSON([]byte(`{"version":2,"pages":[],"qpdf":[{
 "trailer":{"value":{"/Root":"` + root + ` 0 R"}},
 "obj:` + root + ` 0 R":{"value":{"/Type":"/Catalog","/Names":{"/EmbeddedFiles":{"/Names":["u:file","` + attachment + ` 0 R"]}}}},
 "obj:` + attachment + ` 0 R":{"value":{"/Type":"/Filespec","/F":"u:letter R","/EF":{"/F":"` + stream + ` 0 R"},"/Parent":"` + root + ` 0 R"}},
 "obj:` + stream + ` 0 R":{"stream":{"dict":{"/Type":"/EmbeddedFile","/Length":100,"/Filter":"/FlateDecode"}}}
 }]}`))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestPreservationRenumberingAndChangedPayload(t *testing.T) {
	ctx := context.Background()
	client := qpdf.NewClient("qpdf", &fakeRunner{handler: func(_ context.Context, _ worker.CommandOptions, _ string, args ...string) (*worker.Result, error) {
		payload := "embedded original"
		if args[len(args)-1] == "changed.pdf" {
			payload = "changed"
		}
		return &worker.Result{Stdout: []byte(payload)}, nil
	}})
	orig := preservationDoc(t, "1", "2", "3")
	renumbered := preservationDoc(t, "100", "200", "300")
	// Storage encoding is free to change without changing decoded bytes.
	renumbered.QPDF[0]["obj:300 0 R"] = json.RawMessage(`{"stream":{"dict":{"/Type":"/EmbeddedFile","/Length":50}}}`)
	a, err := PreservationFingerprint(ctx, client, "original.pdf", orig)
	if err != nil {
		t.Fatal(err)
	}
	b, err := PreservationFingerprint(ctx, client, "output.pdf", renumbered)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatal("renumbering/storage encoding changed fingerprint")
	}
	changed, err := PreservationFingerprint(ctx, client, "changed.pdf", renumbered)
	if err != nil {
		t.Fatal(err)
	}
	if a == changed {
		t.Fatal("changed embedded bytes accepted")
	}
	renumbered.QPDF[0]["obj:200 0 R"] = json.RawMessage(`{"value":{"/Type":"/Filespec","/F":"u:changed-name","/EF":{"/F":"300 0 R"},"/Parent":"100 0 R"}}`)
	c, err := PreservationFingerprint(ctx, client, "output.pdf", renumbered)
	if err != nil {
		t.Fatal(err)
	}
	if a == c {
		t.Fatal("changed filename accepted")
	}
}

func TestSpecialPolicyUsesLosslessAndStrictRefuses(t *testing.T) {
	for _, feature := range []string{
		`"/StructTreeRoot":{"/Type":"/StructTreeRoot"}`,
		`"/Names":{"/EmbeddedFiles":{}}`,
		`"/Collection":{}`,
		`"/AcroForm":{"/Fields":[{}]}`,
		`"/OpenAction":{"/S":"/JavaScript","/JS":"u:alert(1)"}`,
	} {
		t.Run(feature, func(t *testing.T) {
			doc := createBaseCleanDoc()
			doc.QPDF[0]["obj:1 0 R"] = json.RawMessage(`{"value":{"/Type":"/Catalog",` + feature + `}}`)
			if CheckPolicy(doc, nil).Allowed {
				t.Fatal("strict accepted special feature")
			}
			res := CheckPolicyWithOptions(doc, nil, PolicyOptions{AllowSpecialTypes: true})
			if !res.Allowed || !res.LosslessOnly || len(res.Warnings) == 0 {
				t.Fatalf("special feature not preserved: %+v", res)
			}
			for _, w := range res.Warnings {
				if strings.Contains(w, "stripped") || strings.Contains(w, "flattened") {
					t.Fatal(w)
				}
			}
			analysis := &DocumentAnalysis{PolicyResult: res, Eligibility: &GSEligibility{Eligible: true}}
			if analysis.IsEligibleForGS() {
				t.Fatal("special document sent to Ghostscript")
			}
		})
	}
}

func TestRelaxedPolicyStillRefusesLaunch(t *testing.T) {
	doc := createBaseCleanDoc()
	doc.QPDF[0]["obj:20 0 R"] = json.RawMessage(`{"value":{"/S":"/Launch"}}`)
	if CheckPolicyWithOptions(doc, nil, PolicyOptions{AllowSpecialTypes: true}).Allowed {
		t.Fatal("Launch accepted")
	}
}

func TestPreservationIndirectEFAndUntypedStream(t *testing.T) {
	doc := preservationDoc(t, "1", "2", "3")
	doc.QPDF[0]["obj:2 0 R"] = json.RawMessage(`{"value":{"/Type":"/Filespec","/EF":"4 0 R"}}`)
	doc.QPDF[0]["obj:4 0 R"] = json.RawMessage(`{"value":{"/F":"3 0 R"}}`)
	doc.QPDF[0]["obj:3 0 R"] = json.RawMessage(`{"stream":{"dict":{"/Length":10}}}`)
	payload := "before"
	client := qpdf.NewClient("qpdf", &fakeRunner{handler: func(context.Context, worker.CommandOptions, string, ...string) (*worker.Result, error) {
		return &worker.Result{Stdout: []byte(payload)}, nil
	}})
	a, err := PreservationFingerprint(context.Background(), client, "x.pdf", doc)
	if err != nil {
		t.Fatal(err)
	}
	payload = "after"
	b, err := PreservationFingerprint(context.Background(), client, "x.pdf", doc)
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("indirect EF payload change accepted")
	}
}
