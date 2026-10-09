package pdf

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"pdf-golang-zip/internal/qpdf"
)

// DocumentAnalysis represents the complete analysis, inventory, policy check,
// eligibility, and semantic snapshot of a PDF document.
type DocumentAnalysis struct {
	JSONOutput   *qpdf.JSONOutput   `json:"-"`
	RawJSON      []byte             `json:"-"`
	XMPData      []byte             `json:"-"`
	PolicyResult *PolicyResult      `json:"policy_result"`
	Inventory    *DocumentInventory `json:"inventory"`
	Eligibility  *GSEligibility     `json:"eligibility"`
	Snapshot     *DocumentSnapshot  `json:"snapshot"`
}

// IsAllowed returns true if the document satisfies the strict MVP policy.
func (a *DocumentAnalysis) IsAllowed() bool {
	if a == nil || a.PolicyResult == nil {
		return false
	}
	return a.PolicyResult.Allowed
}

// IsEligibleForGS returns true if the document is allowed by policy and eligible for Ghostscript rewrite.
func (a *DocumentAnalysis) IsEligibleForGS() bool {
	if !a.IsAllowed() {
		return false
	}
	if a.Eligibility == nil {
		return false
	}
	return a.Eligibility.Eligible
}

// Warnings returns the list of eligibility warnings.
func (a *DocumentAnalysis) Warnings() []string {
	if a == nil || a.Eligibility == nil {
		return nil
	}
	return a.Eligibility.Warnings
}

// AnalyzeDocument performs the full document inspection pipeline using qpdf.
func AnalyzeDocument(ctx context.Context, client *qpdf.Client, filePath string) (*DocumentAnalysis, error) {
	if client == nil {
		return nil, fmt.Errorf("qpdf client is nil")
	}

	// 1. Fast encryption check
	encrypted, err := client.IsEncrypted(ctx, filePath)
	if err != nil {
		return nil, fmt.Errorf("encryption probe failed: %w", err)
	}
	if encrypted {
		return &DocumentAnalysis{
			PolicyResult: &PolicyResult{
				Allowed:       false,
				RefusalCode:   CodeEncrypted,
				RefusalReason: "document is encrypted",
			},
			Inventory:   &DocumentInventory{},
			Eligibility: &GSEligibility{Eligible: false},
		}, nil
	}

	// 2. Structural JSON extraction via qpdf
	raw_json, parsed, err := client.AnalyzeRaw(ctx, filePath)
	if err != nil {
		return nil, fmt.Errorf("structural analysis failed: %w", err)
	}

	// 3. Bounded XMP stream read if /Metadata is present in catalog
	var xmp_data []byte
	if root_dict, has_root := parsed.Root(); has_root && root_dict != nil {
		if meta_raw, has_meta := root_dict["/Metadata"]; has_meta {
			var meta_ref string
			if json.Unmarshal(meta_raw, &meta_ref) != nil || meta_ref == "" {
				return nil, fmt.Errorf("metadata must reference a stream: %w", qpdf.ErrInvalidObjectID)
			}
			stream_data, stream_err := client.ReadStream(ctx, filePath, meta_ref, qpdf.DefaultXMPLimit)
			if stream_err != nil {
				if errors.Is(stream_err, qpdf.ErrStreamExceedsLimit) {
					return &DocumentAnalysis{
						JSONOutput: parsed,
						RawJSON:    raw_json,
						PolicyResult: &PolicyResult{
							Allowed:       false,
							RefusalCode:   CodePDFA,
							RefusalReason: "xmp metadata stream exceeds bounded limit",
						},
						Inventory:   &DocumentInventory{},
						Eligibility: &GSEligibility{Eligible: false},
					}, nil
				}
				return nil, fmt.Errorf("required metadata read failed: %w", stream_err)
			} else {
				xmp_data = stream_data
			}
		}
	}

	// 4. Policy validation
	policy_res := CheckPolicy(parsed, xmp_data)

	// 5. Inventory extraction
	inv := ExtractInventory(parsed)

	// 6. Ghostscript eligibility assessment
	eligibility := AssessGSEligibility(inv)

	// 7. Semantic snapshot extraction
	snap, snap_err := BuildSnapshot(parsed)
	if snap_err != nil {
		return nil, fmt.Errorf("failed to build document snapshot: %w", snap_err)
	}

	return &DocumentAnalysis{
		JSONOutput:   parsed,
		RawJSON:      raw_json,
		XMPData:      xmp_data,
		PolicyResult: policy_res,
		Inventory:    inv,
		Eligibility:  eligibility,
		Snapshot:     snap,
	}, nil
}
