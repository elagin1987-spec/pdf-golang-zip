package pdf

import (
	"encoding/json"
	"testing"

	"pdf-golang-zip/internal/qpdf"
)

func createBaseCleanDoc() *qpdf.JSONOutput {
	return &qpdf.JSONOutput{
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
				"trailer": json.RawMessage(`{"value":{"/Root":"1 0 R"}}`),
				"obj:1 0 R": json.RawMessage(
					`{"value":{"/Type":"/Catalog","/Pages":"2 0 R"}}`,
				),
				"obj:2 0 R": json.RawMessage(
					`{"value":{"/Type":"/Pages","/Kids":["3 0 R"],"/Count":1}}`,
				),
				"obj:3 0 R": json.RawMessage(
					`{"value":{"/Type":"/Page","/MediaBox":[0,0,612,792],"/Contents":"4 0 R"}}`,
				),
				"obj:4 0 R": json.RawMessage(
					`{"stream":{"dict":{"/Length":10}}}`,
				),
			},
		},
	}
}

func TestCheckPolicyCleanDocument(t *testing.T) {
	doc := createBaseCleanDoc()
	res := CheckPolicy(doc, nil)
	if !res.Allowed {
		t.Fatalf("expected clean doc to be allowed, got refusal: %s (%s)", res.RefusalCode, res.RefusalReason)
	}
}

func TestCheckPolicyEncryption(t *testing.T) {
	doc := createBaseCleanDoc()
	doc.Encrypt = &qpdf.EncryptInfo{Encrypted: true}

	res := CheckPolicy(doc, nil)
	if res.Allowed {
		t.Fatalf("expected encrypted doc to be refused")
	}
	if res.RefusalCode != CodeEncrypted {
		t.Fatalf("expected refusal code %s, got %s", CodeEncrypted, res.RefusalCode)
	}

	// Also test trailer /Encrypt
	doc2 := createBaseCleanDoc()
	doc2.QPDF[0]["trailer"] = json.RawMessage(`{"value":{"/Root":"1 0 R","/Encrypt":"5 0 R"}}`)
	res2 := CheckPolicy(doc2, nil)
	if res2.Allowed || res2.RefusalCode != CodeEncrypted {
		t.Fatalf("expected trailer /Encrypt to trigger refusal %s, got %+v", CodeEncrypted, res2)
	}
}

func TestCheckPolicyAcroFormAndXFA(t *testing.T) {
	doc := createBaseCleanDoc()
	doc.AcroForm = &qpdf.AcroFormInfo{
		HasAcroForm: true,
		Fields:      []json.RawMessage{json.RawMessage(`"field1"`)},
	}

	res := CheckPolicy(doc, nil)
	if res.Allowed || res.RefusalCode != CodeAcroForm {
		t.Fatalf("expected AcroForm refusal %s, got %+v", CodeAcroForm, res)
	}

	// Dynamic XFA form
	doc2 := createBaseCleanDoc()
	doc2.QPDF[0]["obj:1 0 R"] = json.RawMessage(
		`{"value":{"/Type":"/Catalog","/Pages":"2 0 R","/AcroForm":{"/XFA":["xfa_data"]}}}`,
	)
	res2 := CheckPolicy(doc2, nil)
	if res2.Allowed || res2.RefusalCode != CodeAcroForm {
		t.Fatalf("expected XFA refusal %s, got %+v", CodeAcroForm, res2)
	}
}

func TestCheckPolicySignatures(t *testing.T) {
	// 1. Signature field in AcroForm SigFlags
	doc := createBaseCleanDoc()
	doc.QPDF[0]["obj:1 0 R"] = json.RawMessage(
		`{"value":{"/Type":"/Catalog","/Pages":"2 0 R","/AcroForm":{"/SigFlags":3}}}`,
	)
	res := CheckPolicy(doc, nil)
	if res.Allowed || res.RefusalCode != CodeSignature {
		t.Fatalf("expected SigFlags signature refusal, got %+v", res)
	}

	// 2. /Perms in Catalog
	doc2 := createBaseCleanDoc()
	doc2.QPDF[0]["obj:1 0 R"] = json.RawMessage(
		`{"value":{"/Type":"/Catalog","/Pages":"2 0 R","/Perms":{"/DocMDP":"5 0 R"}}}`,
	)
	res2 := CheckPolicy(doc2, nil)
	if res2.Allowed || res2.RefusalCode != CodeSignature {
		t.Fatalf("expected Perms DocMDP signature refusal, got %+v", res2)
	}

	// 3. Signature object with /Type /Sig
	doc3 := createBaseCleanDoc()
	doc3.QPDF[0]["obj:10 0 R"] = json.RawMessage(
		`{"value":{"/Type":"/Sig","/ByteRange":[0,100,200,300],"/Contents":"sig"}}`,
	)
	res3 := CheckPolicy(doc3, nil)
	if res3.Allowed || res3.RefusalCode != CodeSignature {
		t.Fatalf("expected /Type /Sig signature refusal, got %+v", res3)
	}
}

func TestCheckPolicyPDFA(t *testing.T) {
	doc := createBaseCleanDoc()
	xmp_with_pdfa := []byte(`
		<x:xmpmeta xmlns:x="adobe:ns:meta/">
			<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
				<rdf:Description xmlns:pdfaid="http://www.aiim.org/pdfa/ns/id/">
					<pdfaid:part>1</pdfaid:part>
					<pdfaid:conformance>B</pdfaid:conformance>
				</rdf:Description>
			</rdf:RDF>
		</x:xmpmeta>
	`)

	res := CheckPolicy(doc, xmp_with_pdfa)
	if res.Allowed || res.RefusalCode != CodePDFA {
		t.Fatalf("expected PDF/A refusal %s, got %+v", CodePDFA, res)
	}
}

func TestCheckPolicyAttachmentsAndPortfolio(t *testing.T) {
	// 1. Root Collection (Portfolio)
	doc := createBaseCleanDoc()
	doc.QPDF[0]["obj:1 0 R"] = json.RawMessage(
		`{"value":{"/Type":"/Catalog","/Pages":"2 0 R","/Collection":{}}}`,
	)
	res := CheckPolicy(doc, nil)
	if res.Allowed || res.RefusalCode != CodeAttachment {
		t.Fatalf("expected Portfolio refusal, got %+v", res)
	}

	// 2. /Names -> /EmbeddedFiles
	doc2 := createBaseCleanDoc()
	doc2.QPDF[0]["obj:1 0 R"] = json.RawMessage(
		`{"value":{"/Type":"/Catalog","/Pages":"2 0 R","/Names":{"/EmbeddedFiles":{"/Names":["file.txt","5 0 R"]}}}}`,
	)
	res2 := CheckPolicy(doc2, nil)
	if res2.Allowed || res2.RefusalCode != CodeAttachment {
		t.Fatalf("expected EmbeddedFiles refusal, got %+v", res2)
	}

	// 3. /Type /Filespec in indirect objects
	doc3 := createBaseCleanDoc()
	doc3.QPDF[0]["obj:12 0 R"] = json.RawMessage(
		`{"value":{"/Type":"/Filespec","/F":"doc.docx","/EF":{"/F":"13 0 R"}}}`,
	)
	res3 := CheckPolicy(doc3, nil)
	if res3.Allowed || res3.RefusalCode != CodeAttachment {
		t.Fatalf("expected Filespec refusal, got %+v", res3)
	}
}

func TestCheckPolicyJavaScriptAndRichMedia(t *testing.T) {
	// 1. /JavaScript in OpenAction
	doc := createBaseCleanDoc()
	doc.QPDF[0]["obj:1 0 R"] = json.RawMessage(
		`{"value":{"/Type":"/Catalog","/Pages":"2 0 R","/OpenAction":{"/S":"/JavaScript","/JS":"app.alert(1)"}}}`,
	)
	res := CheckPolicy(doc, nil)
	if res.Allowed || res.RefusalCode != CodeJavaScript {
		t.Fatalf("expected JavaScript OpenAction refusal, got %+v", res)
	}

	// 2. /Launch action
	doc2 := createBaseCleanDoc()
	doc2.QPDF[0]["obj:10 0 R"] = json.RawMessage(
		`{"value":{"/S":"/Launch","/F":"calc.exe"}}`,
	)
	res2 := CheckPolicy(doc2, nil)
	if res2.Allowed || res2.RefusalCode != CodeJavaScript {
		t.Fatalf("expected Launch action refusal, got %+v", res2)
	}

	// 3. /RichMedia object
	doc3 := createBaseCleanDoc()
	doc3.QPDF[0]["obj:11 0 R"] = json.RawMessage(
		`{"value":{"/Type":"/RichMedia"}}`,
	)
	res3 := CheckPolicy(doc3, nil)
	if res3.Allowed || res3.RefusalCode != CodeJavaScript {
		t.Fatalf("expected RichMedia refusal, got %+v", res3)
	}
}

func TestCheckPolicyTaggedPDFAndLayers(t *testing.T) {
	// 1. StructTreeRoot
	doc := createBaseCleanDoc()
	doc.QPDF[0]["obj:1 0 R"] = json.RawMessage(
		`{"value":{"/Type":"/Catalog","/Pages":"2 0 R","/StructTreeRoot":"10 0 R"}}`,
	)
	res := CheckPolicy(doc, nil)
	if res.Allowed || res.RefusalCode != CodeTagged {
		t.Fatalf("expected Tagged StructTreeRoot refusal, got %+v", res)
	}

	// 2. Optional content layers (/OCProperties)
	doc2 := createBaseCleanDoc()
	doc2.QPDF[0]["obj:1 0 R"] = json.RawMessage(
		`{"value":{"/Type":"/Catalog","/Pages":"2 0 R","/OCProperties":{}}}`,
	)
	res2 := CheckPolicy(doc2, nil)
	if res2.Allowed || res2.RefusalCode != CodeLayers {
		t.Fatalf("expected OCProperties layers refusal, got %+v", res2)
	}
}

func TestCheckPolicyAnnotations(t *testing.T) {
	// 1. Disallowed annotation subtype (e.g. /Widget, /Highlight, /Text)
	doc := createBaseCleanDoc()
	doc.QPDF[0]["obj:3 0 R"] = json.RawMessage(
		`{"value":{"/Type":"/Page","/MediaBox":[0,0,612,792],"/Annots":[{"/Subtype":"/Widget"}]}}`,
	)
	res := CheckPolicy(doc, nil)
	if res.Allowed || res.RefusalCode != CodeUnsupportedAnnotation {
		t.Fatalf("expected unsupported annotation refusal, got %+v", res)
	}

	// 2. /Link annotation with appearance (/AP)
	doc2 := createBaseCleanDoc()
	doc2.QPDF[0]["obj:3 0 R"] = json.RawMessage(
		`{"value":{"/Type":"/Page","/MediaBox":[0,0,612,792],"/Annots":[{"/Subtype":"/Link","/AP":{"/N":"5 0 R"}}]}}`,
	)
	res2 := CheckPolicy(doc2, nil)
	if res2.Allowed || res2.RefusalCode != CodeLinkAppearance {
		t.Fatalf("expected link appearance refusal, got %+v", res2)
	}

	// 3. /Link annotation with disallowed action (e.g. /S /Launch)
	doc3 := createBaseCleanDoc()
	doc3.QPDF[0]["obj:3 0 R"] = json.RawMessage(
		`{"value":{"/Type":"/Page","/MediaBox":[0,0,612,792],"/Annots":[{"/Subtype":"/Link","/A":{"/S":"/Launch","/F":"run.sh"}}]}}`,
	)
	res3 := CheckPolicy(doc3, nil)
	if res3.Allowed || res3.RefusalCode != CodeUnsupportedLinkAction {
		t.Fatalf("expected unsupported link action refusal, got %+v", res3)
	}

	// 4. Allowed simple Link annotation with /S /URI and no appearance
	doc4 := createBaseCleanDoc()
	doc4.QPDF[0]["obj:3 0 R"] = json.RawMessage(
		`{"value":{"/Type":"/Page","/MediaBox":[0,0,612,792],"/Annots":[{"/Subtype":"/Link","/Rect":[10,10,100,50],"/A":{"/S":"/URI","/URI":"https://example.org"}}]}}`,
	)
	res4 := CheckPolicy(doc4, nil)
	if !res4.Allowed {
		t.Fatalf("expected clean link annotation to be allowed, got refusal: %+v", res4)
	}
}
