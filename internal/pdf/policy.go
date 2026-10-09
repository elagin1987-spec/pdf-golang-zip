package pdf

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"pdf-golang-zip/internal/qpdf"
)

const (
	// Policy refusal codes.
	CodeEncrypted             = "special_type_encrypted"
	CodeSignature             = "special_type_signature"
	CodeAcroForm              = "special_type_acroform"
	CodePDFA                  = "special_type_pdfa"
	CodeAttachment            = "special_type_attachment"
	CodeJavaScript            = "special_type_javascript"
	CodeTagged                = "special_type_tagged"
	CodeLayers                = "special_type_layers"
	CodeUnsupportedAnnotation = "special_type_unsupported_annotation"
	CodeLinkAppearance        = "special_type_link_appearance"
	CodeUnsupportedLinkAction = "special_type_unsupported_link_action"
)

// PolicyOptions provides fine-grained control over document policy validation.
type PolicyOptions struct {
	AllowSpecialTypes bool
}

// PolicyResult holds the outcome of checking a PDF against acceptance criteria.
type PolicyResult struct {
	Allowed       bool     `json:"allowed"`
	RefusalCode   string   `json:"refusal_code,omitempty"`
	RefusalReason string   `json:"refusal_reason,omitempty"`
	LosslessOnly  bool     `json:"lossless_only,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
}

// CheckPolicy validates a document against the strict MVP refusal rules before any rewrite is attempted.
func CheckPolicy(doc *qpdf.JSONOutput, xmp_data []byte) *PolicyResult {
	return CheckPolicyWithOptions(doc, xmp_data, PolicyOptions{AllowSpecialTypes: false})
}

// CheckPolicyWithOptions validates a document with custom options.
func CheckPolicyWithOptions(doc *qpdf.JSONOutput, xmp_data []byte, opts PolicyOptions) *PolicyResult {
	if doc == nil {
		return &PolicyResult{
			Allowed:       false,
			RefusalCode:   "nil_document",
			RefusalReason: "document structure is nil",
		}
	}

	warning_map := make(map[string]bool)

	// 1. Encryption check
	if doc.Encrypt != nil && doc.Encrypt.Encrypted {
		return &PolicyResult{
			Allowed:       false,
			RefusalCode:   CodeEncrypted,
			RefusalReason: "document is encrypted",
		}
	}
	if trailer, ok := doc.Trailer(); ok {
		if _, has_encrypt := trailer["/Encrypt"]; has_encrypt {
			return &PolicyResult{
				Allowed:       false,
				RefusalCode:   CodeEncrypted,
				RefusalReason: "document trailer contains /Encrypt dictionary",
			}
		}
	}

	// 2. Forms check (AcroForm / XFA)
	if doc.AcroForm != nil {
		if doc.AcroForm.HasAcroForm || len(doc.AcroForm.Fields) > 0 {
			if !opts.AllowSpecialTypes {
				return &PolicyResult{
					Allowed:       false,
					RefusalCode:   CodeAcroForm,
					RefusalReason: "document contains interactive form fields",
				}
			}
			warning_map["interactive_forms_preserved"] = true
		}
	}

	root_dict, has_root := doc.Root()
	if has_root && root_dict != nil {
		if acro_raw, exists := root_dict["/AcroForm"]; exists {
			acro_dict := parseOrResolveDict(doc, acro_raw)
			if acro_dict != nil {
				if _, has_xfa := acro_dict["/XFA"]; has_xfa {
					if !opts.AllowSpecialTypes {
						return &PolicyResult{
							Allowed:       false,
							RefusalCode:   CodeAcroForm,
							RefusalReason: "document contains XFA dynamic form",
						}
					}
					warning_map["interactive_forms_preserved"] = true
				}
				if fields_raw, has_fields := acro_dict["/Fields"]; has_fields {
					fields_arr := resolveArray(doc, fields_raw)
					if len(fields_arr) > 0 {
						if !opts.AllowSpecialTypes {
							return &PolicyResult{
								Allowed:       false,
								RefusalCode:   CodeAcroForm,
								RefusalReason: "document contains AcroForm fields",
							}
						}
						warning_map["interactive_forms_preserved"] = true
					}
				}
				if sig_flags_raw, has_sig_flags := acro_dict["/SigFlags"]; has_sig_flags {
					var sig_flags int
					if json.Unmarshal(sig_flags_raw, &sig_flags) == nil && (sig_flags&3) != 0 {
						return &PolicyResult{
							Allowed:       false,
							RefusalCode:   CodeSignature,
							RefusalReason: "document contains signed form fields (SigFlags)",
						}
					}
				}
			}
		}

		// 3. Catalog Signature permissions
		if perms_raw, has_perms := root_dict["/Perms"]; has_perms {
			perms_str := string(perms_raw)
			if strings.Contains(perms_str, "DocMDP") || strings.Contains(perms_str, "/UR") {
				return &PolicyResult{
					Allowed:       false,
					RefusalCode:   CodeSignature,
					RefusalReason: "document catalog contains /Perms signature permissions",
				}
			}
		}

		// 4. Tagged PDF
		if _, has_struct_tree := root_dict["/StructTreeRoot"]; has_struct_tree {
			if !opts.AllowSpecialTypes {
				return &PolicyResult{
					Allowed:       false,
					RefusalCode:   CodeTagged,
					RefusalReason: "document is tagged (contains /StructTreeRoot)",
				}
			}
			warning_map["tagged_structure_preserved"] = true
		}
		if mark_info_raw, has_mark_info := root_dict["/MarkInfo"]; has_mark_info {
			mark_info_dict := parseOrResolveDict(doc, mark_info_raw)
			if mark_info_dict != nil {
				var is_marked bool
				if marked_raw, ok := mark_info_dict["/Marked"]; ok {
					if json.Unmarshal(marked_raw, &is_marked) == nil && is_marked {
						if !opts.AllowSpecialTypes {
							return &PolicyResult{
								Allowed:       false,
								RefusalCode:   CodeTagged,
								RefusalReason: "document is tagged (MarkInfo /Marked true)",
							}
						}
						warning_map["tagged_structure_preserved"] = true
					}
				}
			}
		}

		// 5. Optional Content (Layers)
		if _, has_oc := root_dict["/OCProperties"]; has_oc {
			if !opts.AllowSpecialTypes {
				return &PolicyResult{
					Allowed:       false,
					RefusalCode:   CodeLayers,
					RefusalReason: "document contains optional content layers (/OCProperties)",
				}
			}
			warning_map["layers_preserved"] = true
		}

		// 6. Portfolio collection
		if _, has_collection := root_dict["/Collection"]; has_collection {
			if !opts.AllowSpecialTypes {
				return &PolicyResult{
					Allowed:       false,
					RefusalCode:   CodeAttachment,
					RefusalReason: "document is a PDF portfolio (/Collection)",
				}
			}
			warning_map["attachments_preserved"] = true
		}

		// 7. Embedded files or JS in Names
		if names_raw, has_names := root_dict["/Names"]; has_names {
			names_dict := parseOrResolveDict(doc, names_raw)
			if names_dict != nil {
				if _, has_ef := names_dict["/EmbeddedFiles"]; has_ef {
					if !opts.AllowSpecialTypes {
						return &PolicyResult{
							Allowed:       false,
							RefusalCode:   CodeAttachment,
							RefusalReason: "document /Names contains /EmbeddedFiles",
						}
					}
					warning_map["attachments_preserved"] = true
				}
				if _, has_js := names_dict["/JavaScript"]; has_js {
					if !opts.AllowSpecialTypes {
						return &PolicyResult{
							Allowed:       false,
							RefusalCode:   CodeJavaScript,
							RefusalReason: "document /Names contains /JavaScript",
						}
					}
					warning_map["javascript_preserved"] = true
				}
			}
		}

		// 8. OpenAction check
		if open_action_raw, has_open_action := root_dict["/OpenAction"]; has_open_action {
			oa_dict := parseOrResolveDict(doc, open_action_raw)
			if oa_dict != nil {
				var s_action string
				if s_raw, ok := oa_dict["/S"]; ok {
					_ = json.Unmarshal(s_raw, &s_action)
					if s_action == "/Launch" {
						return &PolicyResult{
							Allowed:       false,
							RefusalCode:   CodeJavaScript,
							RefusalReason: "document /OpenAction contains disallowed action: " + s_action,
						}
					}
					if s_action == "/JavaScript" {
						if !opts.AllowSpecialTypes {
							return &PolicyResult{
								Allowed:       false,
								RefusalCode:   CodeJavaScript,
								RefusalReason: "document /OpenAction contains disallowed action: " + s_action,
							}
						}
						warning_map["javascript_preserved"] = true
					}
				}
			}
		}
	}

	// 9. Root-level attachments map
	if len(doc.Attachments) > 0 {
		if !opts.AllowSpecialTypes {
			return &PolicyResult{
				Allowed:       false,
				RefusalCode:   CodeAttachment,
				RefusalReason: "document contains attachments",
			}
		}
		warning_map["attachments_preserved"] = true
	}

	// 10. PDF/A declared in bounded XMP metadata
	if len(xmp_data) > 0 {
		xmp_lower := strings.ToLower(string(xmp_data))
		if strings.Contains(xmp_lower, "pdfaid:part") ||
			strings.Contains(xmp_lower, "pdfaextension:schemas") ||
			strings.Contains(xmp_lower, "pdfaschema:schema") ||
			strings.Contains(xmp_lower, "xmlns:pdfaid") {
			return &PolicyResult{
				Allowed:       false,
				RefusalCode:   CodePDFA,
				RefusalReason: "document declares PDF/A conformance in XMP metadata",
			}
		}
	}

	// 11. Scan indirect objects for signatures, attachments, multimedia, JS, layers
	for _, section := range doc.QPDF {
		for _, raw_obj := range section {
			var entry qpdf.ObjectEntry
			if json.Unmarshal(raw_obj, &entry) != nil {
				continue
			}
			dict := entry.GetDict()
			if dict == nil {
				continue
			}

			var type_val, ft_val, subtype_val string
			if raw, ok := dict["/Type"]; ok {
				_ = json.Unmarshal(raw, &type_val)
			}
			if raw, ok := dict["/FT"]; ok {
				_ = json.Unmarshal(raw, &ft_val)
			}
			if raw, ok := dict["/Subtype"]; ok {
				_ = json.Unmarshal(raw, &subtype_val)
			}

			if type_val == "/Sig" || ft_val == "/Sig" {
				return &PolicyResult{
					Allowed:       false,
					RefusalCode:   CodeSignature,
					RefusalReason: "document contains signature object (/Type or /FT /Sig)",
				}
			}
			if _, has_byte_range := dict["/ByteRange"]; has_byte_range {
				if _, has_contents := dict["/Contents"]; has_contents {
					return &PolicyResult{
						Allowed:       false,
						RefusalCode:   CodeSignature,
						RefusalReason: "document contains signature dictionary with /ByteRange",
					}
				}
			}
			if _, has_doc_checksum := dict["/DocChecksum"]; has_doc_checksum {
				return &PolicyResult{
					Allowed:       false,
					RefusalCode:   CodeSignature,
					RefusalReason: "document contains signature dictionary with /DocChecksum",
				}
			}
			if subtype_val == "/ETSI.CAdES.detached" || subtype_val == "/adbe.pkcs7.detached" {
				return &PolicyResult{
					Allowed:       false,
					RefusalCode:   CodeSignature,
					RefusalReason: "document contains digital signature subtype " + subtype_val,
				}
			}

			if type_val == "/Filespec" || type_val == "/EmbeddedFile" {
				if !opts.AllowSpecialTypes {
					return &PolicyResult{
						Allowed:       false,
						RefusalCode:   CodeAttachment,
						RefusalReason: "document contains embedded file / attachment object",
					}
				}
				warning_map["attachments_preserved"] = true
			}
			if _, has_ef := dict["/EF"]; has_ef {
				if !opts.AllowSpecialTypes {
					return &PolicyResult{
						Allowed:       false,
						RefusalCode:   CodeAttachment,
						RefusalReason: "document contains embedded file stream dictionary (/EF)",
					}
				}
				warning_map["attachments_preserved"] = true
			}

			if _, has_js := dict["/JS"]; has_js {
				if !opts.AllowSpecialTypes {
					return &PolicyResult{
						Allowed:       false,
						RefusalCode:   CodeJavaScript,
						RefusalReason: "document contains JavaScript object (/JS)",
					}
				}
				warning_map["javascript_preserved"] = true
			}
			if _, has_javascript := dict["/JavaScript"]; has_javascript {
				if !opts.AllowSpecialTypes {
					return &PolicyResult{
						Allowed:       false,
						RefusalCode:   CodeJavaScript,
						RefusalReason: "document contains JavaScript object (/JavaScript)",
					}
				}
				warning_map["javascript_preserved"] = true
			}
			if type_val == "/RichMedia" || type_val == "/RichMediaInstance" ||
				type_val == "/Sound" || type_val == "/Movie" {
				if !opts.AllowSpecialTypes {
					return &PolicyResult{
						Allowed:       false,
						RefusalCode:   CodeJavaScript,
						RefusalReason: "document contains multimedia object (" + type_val + ")",
					}
				}
				warning_map["multimedia_preserved"] = true
			}

			if raw_s, ok := dict["/S"]; ok {
				var s_action string
				_ = json.Unmarshal(raw_s, &s_action)
				if disallowedAction(s_action) {
					if s_action == "/Launch" {
						return &PolicyResult{
							Allowed:       false,
							RefusalCode:   CodeJavaScript,
							RefusalReason: "document contains disallowed action type: " + s_action,
						}
					}
					if !opts.AllowSpecialTypes {
						return &PolicyResult{
							Allowed:       false,
							RefusalCode:   CodeJavaScript,
							RefusalReason: "document contains disallowed action type: " + s_action,
						}
					}
					warning_map["javascript_preserved"] = true
				}
			}

			if type_val == "/OCG" || type_val == "/OCMD" {
				if !opts.AllowSpecialTypes {
					return &PolicyResult{
						Allowed:       false,
						RefusalCode:   CodeLayers,
						RefusalReason: "document contains optional content layer (" + type_val + ")",
					}
				}
				warning_map["layers_preserved"] = true
			}
		}
	}

	// 12. Annotations check on all pages
	for page_idx, page_info := range doc.Pages {
		page_obj, found := doc.FindObject(page_info.Object)
		if !found || page_obj == nil {
			continue
		}
		page_dict := page_obj.GetDict()
		if page_dict == nil {
			continue
		}

		annots_raw, has_annots := page_dict["/Annots"]
		if !has_annots {
			continue
		}

		annots_arr := resolveArray(doc, annots_raw)
		for _, annot_raw := range annots_arr {
			annot_dict := parseOrResolveDict(doc, annot_raw)
			if annot_dict == nil {
				continue
			}

			var subtype_name string
			if sub_raw, ok := annot_dict["/Subtype"]; ok {
				_ = json.Unmarshal(sub_raw, &subtype_name)
			}

			if subtype_name != "/Link" {
				if !opts.AllowSpecialTypes {
					return &PolicyResult{
						Allowed:     false,
						RefusalCode: CodeUnsupportedAnnotation,
						RefusalReason: fmt.Sprintf(
							"page %d contains unsupported annotation subtype: %s",
							page_idx+1,
							subtype_name,
						),
					}
				}
				warning_map["annotations_preserved"] = true
			}

			// For /Link: must NOT have /AP (appearance) or /AS
			if _, has_ap := annot_dict["/AP"]; has_ap {
				if !opts.AllowSpecialTypes {
					return &PolicyResult{
						Allowed:     false,
						RefusalCode: CodeLinkAppearance,
						RefusalReason: fmt.Sprintf(
							"page %d link annotation contains appearance dictionary (/AP)",
							page_idx+1,
						),
					}
				}
				warning_map["link_appearance_preserved"] = true
			}
			if _, has_as := annot_dict["/AS"]; has_as {
				if !opts.AllowSpecialTypes {
					return &PolicyResult{
						Allowed:     false,
						RefusalCode: CodeLinkAppearance,
						RefusalReason: fmt.Sprintf(
							"page %d link annotation contains appearance state (/AS)",
							page_idx+1,
						),
					}
				}
				warning_map["link_appearance_preserved"] = true
			}

			// Validate Action /A if present
			if action_raw, has_action := annot_dict["/A"]; has_action {
				action_dict := parseOrResolveDict(doc, action_raw)
				if action_dict != nil {
					var s_action string
					if s_raw, ok := action_dict["/S"]; ok {
						_ = json.Unmarshal(s_raw, &s_action)
					}
					if s_action != "/URI" && s_action != "/GoTo" {
						if s_action == "/Launch" {
							return &PolicyResult{
								Allowed:     false,
								RefusalCode: CodeUnsupportedLinkAction,
								RefusalReason: fmt.Sprintf(
									"page %d link annotation has disallowed action type: %s",
									page_idx+1,
									s_action,
								),
							}
						}
						if !opts.AllowSpecialTypes {
							return &PolicyResult{
								Allowed:     false,
								RefusalCode: CodeUnsupportedLinkAction,
								RefusalReason: fmt.Sprintf(
									"page %d link annotation has unsupported action type: %s",
									page_idx+1,
									s_action,
								),
							}
						}
						warning_map["non_standard_action_preserved"] = true
					}
				}
			}
		}
	}

	if refusal := checkNestedActions(doc, opts, warning_map); refusal != nil && !refusal.Allowed {
		return refusal
	}

	var ordered_warnings []string
	for w := range warning_map {
		ordered_warnings = append(ordered_warnings, w)
	}
	sort.Strings(ordered_warnings)

	return &PolicyResult{
		Allowed:      true,
		LosslessOnly: len(ordered_warnings) > 0,
		Warnings:     ordered_warnings,
	}
}

// Every indirect object is present in qpdf JSON. Scan its direct containers once;
// reference strings need no graph traversal and cannot create recursion cycles.
func checkNestedActions(doc *qpdf.JSONOutput, opts PolicyOptions, warning_map map[string]bool) *PolicyResult {
	nodes := 0
	var walk func(any, int) *PolicyResult
	walk = func(value any, depth int) *PolicyResult {
		switch value.(type) {
		case map[string]any, []any:
		default:
			return nil
		}
		nodes++
		if depth > MaxOutlineDepth || nodes > qpdf.MaxDocumentObjects {
			return &PolicyResult{RefusalCode: "document_limit", RefusalReason: "policy traversal limit exceeded"}
		}
		switch v := value.(type) {
		case map[string]any:
			_, js := v["/JS"]
			_, javascript := v["/JavaScript"]
			action, _ := v["/S"].(string)
			if js || javascript || disallowedAction(action) {
				if action == "/Launch" {
					return &PolicyResult{
						RefusalCode:   CodeJavaScript,
						RefusalReason: "document contains a disallowed nested action",
					}
				}
				if !opts.AllowSpecialTypes {
					return &PolicyResult{
						RefusalCode:   CodeJavaScript,
						RefusalReason: "document contains a disallowed nested action",
					}
				}
				warning_map["javascript_preserved"] = true
			}
			for _, child := range v {
				if refusal := walk(child, depth+1); refusal != nil {
					return refusal
				}
			}
		case []any:
			for _, child := range v {
				if refusal := walk(child, depth+1); refusal != nil {
					return refusal
				}
			}
		}
		return nil
	}
	for _, section := range doc.QPDF {
		for key, raw := range section {
			if key != "trailer" && !strings.HasPrefix(key, "obj:") {
				continue // qpdf JSON header fields are not PDF objects.
			}
			var entry qpdf.ObjectEntry
			if json.Unmarshal(raw, &entry) != nil {
				return &PolicyResult{RefusalCode: "invalid_pdf", RefusalReason: "invalid policy object"}
			}
			var value any
			if entry.Stream != nil {
				data, err := json.Marshal(entry.Stream.Dict)
				if err != nil || json.Unmarshal(data, &value) != nil {
					return &PolicyResult{RefusalCode: "invalid_pdf", RefusalReason: "invalid stream dictionary"}
				}
			} else if len(entry.RawValue) > 0 {
				if json.Unmarshal(entry.RawValue, &value) != nil {
					return &PolicyResult{RefusalCode: "invalid_pdf", RefusalReason: "invalid policy value"}
				}
			}
			if refusal := walk(value, 0); refusal != nil {
				return refusal
			}
		}
	}
	return &PolicyResult{Allowed: true}
}

func disallowedAction(action string) bool {
	switch action {
	case "/JavaScript", "/Launch", "/SubmitForm", "/ImportData", "/ResetForm", "/Sound", "/Movie", "/RichMediaExecute":
		return true
	default:
		return false
	}
}

func parseOrResolveDict(doc *qpdf.JSONOutput, raw json.RawMessage) map[string]json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var dict map[string]json.RawMessage
	if json.Unmarshal(raw, &dict) == nil {
		return dict
	}
	var ref string
	if json.Unmarshal(raw, &ref) == nil {
		if obj, found := doc.FindObject(ref); found {
			return obj.GetDict()
		}
	}
	return nil
}

func resolveArray(doc *qpdf.JSONOutput, raw json.RawMessage) []json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) == nil {
		return arr
	}
	var ref string
	if json.Unmarshal(raw, &ref) == nil {
		if obj, found := doc.FindObject(ref); found {
			if a, ok := obj.AsArray(); ok {
				return a
			}
		}
	}
	return nil
}
