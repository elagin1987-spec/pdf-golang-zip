package pdf

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPolicyNestedActions(t *testing.T) {
	for _, tt := range []struct{ name, object string }{
		{"catalog AA", `{"/Type":"/Catalog","/AA":{"/WC":{"/S":"/JavaScript","/JS":"u:alert(1)"}}}`},
		{"page AA", `{"/Type":"/Page","/AA":{"/O":{"/S":"/JavaScript","/JS":"u:alert(1)"}}}`},
		{"link AA", `{"/Subtype":"/Link","/AA":{"/E":{"/S":"/Launch"}}}`},
		{"next dictionary", `{"/S":"/URI","/URI":"u:https://example.org","/Next":{"/S":"/JavaScript","/JS":"u:alert(1)"}}`},
		{"next array", `{"/S":"/GoTo","/Next":[{"/S":"/URI"},{"/S":"/Launch"}]}`},
		{"stream dictionary", `{"/AA":{"/O":{"/S":"/JavaScript","/JS":"u:alert(1)"}}}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			doc := createBaseCleanDoc()
			wrapper := `{"value":` + tt.object + `}`
			if tt.name == "stream dictionary" {
				wrapper = `{"stream":{"dict":` + tt.object + `}}`
			}
			doc.QPDF[0]["obj:20 0 R"] = json.RawMessage(wrapper)
			res := CheckPolicy(doc, nil)
			if res.Allowed || res.RefusalCode != CodeJavaScript {
				t.Fatalf("nested action accepted: %+v", res)
			}
		})
	}
}

func TestPolicyNestedActionsControls(t *testing.T) {
	doc := createBaseCleanDoc()
	// Real qpdf JSON includes a header section with scalar bookkeeping fields.
	doc.QPDF = append([]map[string]json.RawMessage{{
		"jsonversion":                  json.RawMessage(`2`),
		"pushedinheritedpageresources": json.RawMessage(`true`),
		"calledgetallpages":            json.RawMessage(`true`),
		"maxobjectid":                  json.RawMessage(`21`),
	}}, doc.QPDF...)
	// Reference strings are not traversed: every indirect object is scanned once.
	doc.QPDF[0]["obj:20 0 R"] = json.RawMessage(`{"value":{"/S":"/URI","/URI":"u:https://example.org","/Next":"21 0 R","/Description":"u:/JavaScript /Launch /JS"}}`)
	doc.QPDF[0]["obj:21 0 R"] = json.RawMessage(`{"value":{"/S":"/GoTo","/Next":"20 0 R"}}`)
	if res := CheckPolicy(doc, nil); !res.Allowed {
		t.Fatalf("safe actions/text/reference cycle refused: %+v", res)
	}
	doc.QPDF[0]["obj:21 0 R"] = json.RawMessage(`{"value":{"/S":"/JavaScript","/JS":"u:alert(1)"}}`)
	if res := CheckPolicy(doc, nil); res.Allowed || res.RefusalCode != CodeJavaScript {
		t.Fatalf("indirect action accepted: %+v", res)
	}
}

func TestPolicyNestedTraversalLimits(t *testing.T) {
	for _, tt := range []struct{ name, value string }{
		{"depth", strings.Repeat(`{"/Nested":`, 34) + `{}` + strings.Repeat(`}`, 34)},
		{"nodes", `[` + strings.Repeat(`{},`, 100000) + `{}]`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			doc := createBaseCleanDoc()
			doc.QPDF[0]["obj:20 0 R"] = json.RawMessage(`{"value":` + tt.value + `}`)
			if res := CheckPolicy(doc, nil); res.Allowed || res.RefusalCode != "document_limit" {
				t.Fatalf("unbounded policy traversal: %+v", res)
			}
		})
	}
}
