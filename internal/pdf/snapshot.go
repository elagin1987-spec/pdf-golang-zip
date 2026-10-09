package pdf

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"pdf-golang-zip/internal/qpdf"
)

const (
	// TolerancePoints is the coordinate comparison tolerance (0.01 pt).
	TolerancePoints = 0.01

	// MaxOutlineDepth is the recursion cap for outline hierarchy traversal.
	MaxOutlineDepth = 32
)

// PageGeometry contains the physical bounding dimensions and orientation of a page.
type PageGeometry struct {
	MediaBox [4]float64 `json:"media_box"`
	CropBox  [4]float64 `json:"crop_box"`
	Rotate   int        `json:"rotate"`
	UserUnit float64    `json:"user_unit"`
}

// LinkSnapshot captures the semantic properties of a Link annotation.
type LinkSnapshot struct {
	Rect       [4]float64 `json:"rect"`
	TargetType string     `json:"target_type"` // "URI" or "GoTo"
	URI        string     `json:"uri,omitempty"`
	DestPage   int        `json:"dest_page,omitempty"` // 1-based page index
	DestFit    string     `json:"dest_fit,omitempty"`
}

// OutlineSnapshot represents a bookmark/outline node in a document.
type OutlineSnapshot struct {
	Title    string            `json:"title"`
	DestPage int               `json:"dest_page,omitempty"` // 1-based page index
	DestFit  string            `json:"dest_fit,omitempty"`
	Kids     []OutlineSnapshot `json:"kids,omitempty"`
}

// PageSnapshot holds the geometry and interactive links of a single page.
type PageSnapshot struct {
	PagePosFrom1 int            `json:"page_pos_from_1"`
	Geometry     PageGeometry   `json:"geometry"`
	Links        []LinkSnapshot `json:"links"`
}

// DocumentSnapshot contains the structural and semantic snapshot of a document
// used to verify invariant properties across compression candidates.
type DocumentSnapshot struct {
	PageCount int               `json:"page_count"`
	Pages     []PageSnapshot    `json:"pages"`
	Outlines  []OutlineSnapshot `json:"outlines"`
}

// BuildSnapshot extracts physical page geometry, link annotations, and outlines from qpdf JSON v2.
// All GoTo destinations and bookmark references are resolved to 1-based page numbers, making
// the snapshot completely invariant to object renumbering.
func BuildSnapshot(doc *qpdf.JSONOutput) (*DocumentSnapshot, error) {
	if doc == nil {
		return nil, fmt.Errorf("cannot build snapshot from nil document")
	}

	snap := &DocumentSnapshot{
		PageCount: len(doc.Pages),
		Pages:     make([]PageSnapshot, 0, len(doc.Pages)),
		Outlines:  make([]OutlineSnapshot, 0),
	}

	// 1. Build map of object references to 1-based page index
	page_ref_map := make(map[string]int)
	for _, p := range doc.Pages {
		norm_id, err := qpdf.NormalizeObjectID(p.Object)
		if err == nil {
			page_ref_map[norm_id] = p.PagePosFrom1
		}
		raw_trimmed := strings.TrimPrefix(strings.TrimSpace(p.Object), "obj:")
		page_ref_map[raw_trimmed] = p.PagePosFrom1
	}

	// 2. Extract page geometry and links
	for _, p := range doc.Pages {
		page_obj, found := doc.FindObject(p.Object)
		var page_dict map[string]json.RawMessage
		if found && page_obj != nil {
			page_dict = page_obj.GetDict()
		}

		geom := extractPageGeometry(doc, page_dict)
		links := extractPageLinks(doc, page_dict, page_ref_map)

		snap.Pages = append(snap.Pages, PageSnapshot{
			PagePosFrom1: p.PagePosFrom1,
			Geometry:     geom,
			Links:        links,
		})
	}

	// 3. Extract outlines / bookmarks
	snap.Outlines = extractOutlines(doc, page_ref_map)

	return snap, nil
}

// CompareSnapshots verifies semantic equivalence between the original document and a candidate.
// Returns true if all geometry, links, and outlines match within tolerance (0.01 pt),
// regardless of object renumbering.
func CompareSnapshots(orig, candidate *DocumentSnapshot) (bool, []string) {
	var mismatches []string

	if orig == nil || candidate == nil {
		return false, []string{"one or both snapshots are nil"}
	}

	if orig.PageCount != candidate.PageCount {
		mismatches = append(
			mismatches,
			fmt.Sprintf("page count mismatch: orig %d, candidate %d", orig.PageCount, candidate.PageCount),
		)
		return false, mismatches
	}

	for i := 0; i < orig.PageCount; i++ {
		orig_page := orig.Pages[i]
		cand_page := candidate.Pages[i]
		page_num := orig_page.PagePosFrom1

		// MediaBox comparison
		for k := 0; k < 4; k++ {
			diff := math.Abs(orig_page.Geometry.MediaBox[k] - cand_page.Geometry.MediaBox[k])
			if diff > TolerancePoints {
				mismatches = append(mismatches, fmt.Sprintf(
					"page %d MediaBox[%d] diff %.4f exceeds tolerance: orig %.2f, cand %.2f",
					page_num,
					k,
					diff,
					orig_page.Geometry.MediaBox[k],
					cand_page.Geometry.MediaBox[k],
				))
			}
		}

		// CropBox comparison
		for k := 0; k < 4; k++ {
			diff := math.Abs(orig_page.Geometry.CropBox[k] - cand_page.Geometry.CropBox[k])
			if diff > TolerancePoints {
				mismatches = append(mismatches, fmt.Sprintf(
					"page %d CropBox[%d] diff %.4f exceeds tolerance: orig %.2f, cand %.2f",
					page_num,
					k,
					diff,
					orig_page.Geometry.CropBox[k],
					cand_page.Geometry.CropBox[k],
				))
			}
		}

		// Rotate comparison
		if orig_page.Geometry.Rotate != cand_page.Geometry.Rotate {
			mismatches = append(mismatches, fmt.Sprintf(
				"page %d Rotate mismatch: orig %d, cand %d",
				page_num,
				orig_page.Geometry.Rotate,
				cand_page.Geometry.Rotate,
			))
		}

		// UserUnit comparison
		unit_diff := math.Abs(orig_page.Geometry.UserUnit - cand_page.Geometry.UserUnit)
		if unit_diff > TolerancePoints {
			mismatches = append(mismatches, fmt.Sprintf(
				"page %d UserUnit diff %.4f exceeds tolerance: orig %.2f, cand %.2f",
				page_num,
				unit_diff,
				orig_page.Geometry.UserUnit,
				cand_page.Geometry.UserUnit,
			))
		}

		// Links comparison
		if len(orig_page.Links) != len(cand_page.Links) {
			mismatches = append(mismatches, fmt.Sprintf(
				"page %d link count mismatch: orig %d, cand %d",
				page_num,
				len(orig_page.Links),
				len(cand_page.Links),
			))
		} else {
			for link_idx := 0; link_idx < len(orig_page.Links); link_idx++ {
				ol := orig_page.Links[link_idx]
				cl := cand_page.Links[link_idx]

				for k := 0; k < 4; k++ {
					diff := math.Abs(ol.Rect[k] - cl.Rect[k])
					if diff > TolerancePoints {
						mismatches = append(mismatches, fmt.Sprintf(
							"page %d link %d Rect[%d] diff %.4f exceeds tolerance",
							page_num,
							link_idx+1,
							k,
							diff,
						))
					}
				}

				if ol.TargetType != cl.TargetType {
					mismatches = append(mismatches, fmt.Sprintf(
						"page %d link %d TargetType mismatch: orig %s, cand %s",
						page_num,
						link_idx+1,
						ol.TargetType,
						cl.TargetType,
					))
				}

				if ol.TargetType == "URI" && ol.URI != cl.URI {
					mismatches = append(mismatches, fmt.Sprintf(
						"page %d link %d URI mismatch: orig %s, cand %s",
						page_num,
						link_idx+1,
						ol.URI,
						cl.URI,
					))
				}

				if ol.TargetType == "GoTo" {
					if ol.DestPage != cl.DestPage {
						mismatches = append(mismatches, fmt.Sprintf(
							"page %d link %d DestPage mismatch: orig %d, cand %d",
							page_num,
							link_idx+1,
							ol.DestPage,
							cl.DestPage,
						))
					}
				}
			}
		}
	}

	// Outlines comparison
	outline_mismatches := compareOutlineNodes(orig.Outlines, candidate.Outlines, "")
	mismatches = append(mismatches, outline_mismatches...)

	if len(mismatches) > 0 {
		return false, mismatches
	}

	return true, nil
}

func compareOutlineNodes(orig, cand []OutlineSnapshot, path string) []string {
	var mismatches []string

	if len(orig) != len(cand) {
		mismatches = append(mismatches, fmt.Sprintf(
			"outlines count mismatch at %s: orig %d, cand %d",
			path,
			len(orig),
			len(cand),
		))
		return mismatches
	}

	for i := 0; i < len(orig); i++ {
		o := orig[i]
		c := cand[i]
		node_path := fmt.Sprintf("%s[%d:%s]", path, i, o.Title)

		if o.Title != c.Title {
			mismatches = append(mismatches, fmt.Sprintf(
				"outline title mismatch at %s: orig %q, cand %q",
				node_path,
				o.Title,
				c.Title,
			))
		}

		if o.DestPage != c.DestPage {
			mismatches = append(mismatches, fmt.Sprintf(
				"outline DestPage mismatch at %s: orig %d, cand %d",
				node_path,
				o.DestPage,
				c.DestPage,
			))
		}

		sub_errs := compareOutlineNodes(o.Kids, c.Kids, node_path)
		mismatches = append(mismatches, sub_errs...)
	}

	return mismatches
}

func extractPageGeometry(doc *qpdf.JSONOutput, page_dict map[string]json.RawMessage) PageGeometry {
	geom := PageGeometry{
		MediaBox: [4]float64{0, 0, 612, 792}, // standard fallback
		CropBox:  [4]float64{0, 0, 612, 792},
		Rotate:   0,
		UserUnit: 1.0,
	}

	if page_dict == nil {
		return geom
	}

	// MediaBox
	if mb_raw, ok := page_dict["/MediaBox"]; ok {
		if mb, parsed := parseBox(mb_raw); parsed {
			geom.MediaBox = mb
			geom.CropBox = mb
		}
	} else if parent_raw, has_parent := page_dict["/Parent"]; has_parent {
		// Inheritance from parent Pages dictionary
		parent_dict := parseOrResolveDict(doc, parent_raw)
		if parent_dict != nil {
			if mb_raw, ok := parent_dict["/MediaBox"]; ok {
				if mb, parsed := parseBox(mb_raw); parsed {
					geom.MediaBox = mb
					geom.CropBox = mb
				}
			}
		}
	}

	// CropBox
	if cb_raw, ok := page_dict["/CropBox"]; ok {
		if cb, parsed := parseBox(cb_raw); parsed {
			geom.CropBox = cb
		}
	}

	// Rotate
	if rot_raw, ok := page_dict["/Rotate"]; ok {
		var rot int
		if json.Unmarshal(rot_raw, &rot) == nil {
			geom.Rotate = ((rot % 360) + 360) % 360
		}
	}

	// UserUnit
	if uu_raw, ok := page_dict["/UserUnit"]; ok {
		var uu json.Number
		if json.Unmarshal(uu_raw, &uu) == nil {
			if val, err := uu.Float64(); err == nil && val > 0 {
				geom.UserUnit = val
			}
		}
	}

	return geom
}

func extractPageLinks(
	doc *qpdf.JSONOutput,
	page_dict map[string]json.RawMessage,
	page_ref_map map[string]int,
) []LinkSnapshot {
	links := make([]LinkSnapshot, 0)
	if page_dict == nil {
		return links
	}

	annots_raw, has_annots := page_dict["/Annots"]
	if !has_annots {
		return links
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
			continue
		}

		var rect [4]float64
		if rect_raw, ok := annot_dict["/Rect"]; ok {
			if r, parsed := parseBox(rect_raw); parsed {
				rect = r
			}
		}

		link_snap := LinkSnapshot{
			Rect: rect,
		}

		// Action /A
		if action_raw, ok := annot_dict["/A"]; ok {
			action_dict := parseOrResolveDict(doc, action_raw)
			if action_dict != nil {
				var s_action string
				if s_raw, ok := action_dict["/S"]; ok {
					_ = json.Unmarshal(s_raw, &s_action)
				}
				if s_action == "/URI" {
					link_snap.TargetType = "URI"
					if uri_raw, ok := action_dict["/URI"]; ok {
						link_snap.URI = cleanText(string(uri_raw))
					}
				} else if s_action == "/GoTo" {
					link_snap.TargetType = "GoTo"
					if d_raw, ok := action_dict["/D"]; ok {
						dest_page, dest_fit := resolveDest(doc, d_raw, page_ref_map)
						link_snap.DestPage = dest_page
						link_snap.DestFit = dest_fit
					}
				}
			}
		} else if dest_raw, ok := annot_dict["/Dest"]; ok {
			link_snap.TargetType = "GoTo"
			dest_page, dest_fit := resolveDest(doc, dest_raw, page_ref_map)
			link_snap.DestPage = dest_page
			link_snap.DestFit = dest_fit
		}

		if link_snap.TargetType != "" {
			links = append(links, link_snap)
		}
	}

	return links
}

func extractOutlines(doc *qpdf.JSONOutput, page_ref_map map[string]int) []OutlineSnapshot {
	outlines := make([]OutlineSnapshot, 0)
	root_dict, has_root := doc.Root()
	if !has_root || root_dict == nil {
		return outlines
	}

	outlines_raw, has_outlines := root_dict["/Outlines"]
	if !has_outlines {
		return outlines
	}

	outlines_dict := parseOrResolveDict(doc, outlines_raw)
	if outlines_dict == nil {
		return outlines
	}

	first_raw, has_first := outlines_dict["/First"]
	if !has_first {
		return outlines
	}

	var first_ref string
	if json.Unmarshal(first_raw, &first_ref) != nil {
		return outlines
	}

	visited := make(map[string]bool)
	return traverseOutlineChain(doc, first_ref, page_ref_map, visited, 0)
}

func traverseOutlineChain(
	doc *qpdf.JSONOutput,
	start_ref string,
	page_ref_map map[string]int,
	visited map[string]bool,
	depth int,
) []OutlineSnapshot {
	var list []OutlineSnapshot
	if depth > MaxOutlineDepth {
		return list
	}

	curr_ref := start_ref
	for curr_ref != "" {
		norm_curr, _ := qpdf.NormalizeObjectID(curr_ref)
		if visited[norm_curr] {
			break
		}
		visited[norm_curr] = true

		obj, found := doc.FindObject(curr_ref)
		if !found || obj == nil {
			break
		}
		dict := obj.GetDict()
		if dict == nil {
			break
		}

		var title string
		if title_raw, ok := dict["/Title"]; ok {
			title = cleanText(string(title_raw))
		}

		dest_page := 0
		dest_fit := ""
		if dest_raw, ok := dict["/Dest"]; ok {
			dest_page, dest_fit = resolveDest(doc, dest_raw, page_ref_map)
		} else if a_raw, ok := dict["/A"]; ok {
			a_dict := parseOrResolveDict(doc, a_raw)
			if a_dict != nil {
				if d_raw, ok := a_dict["/D"]; ok {
					dest_page, dest_fit = resolveDest(doc, d_raw, page_ref_map)
				}
			}
		}

		node := OutlineSnapshot{
			Title:    title,
			DestPage: dest_page,
			DestFit:  dest_fit,
		}

		// Kids
		if first_raw, ok := dict["/First"]; ok {
			var first_kid string
			if json.Unmarshal(first_raw, &first_kid) == nil && first_kid != "" {
				node.Kids = traverseOutlineChain(doc, first_kid, page_ref_map, visited, depth+1)
			}
		}

		list = append(list, node)

		// Next sibling
		if next_raw, ok := dict["/Next"]; ok {
			var next_ref string
			if json.Unmarshal(next_raw, &next_ref) == nil {
				curr_ref = next_ref
			} else {
				curr_ref = ""
			}
		} else {
			curr_ref = ""
		}
	}

	return list
}

func resolveDest(
	doc *qpdf.JSONOutput,
	raw json.RawMessage,
	page_ref_map map[string]int,
) (int, string) {
	if len(raw) == 0 {
		return 0, ""
	}

	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) == nil && len(arr) > 0 {
		var target_ref string
		if json.Unmarshal(arr[0], &target_ref) == nil {
			norm_id, err := qpdf.NormalizeObjectID(target_ref)
			if err == nil {
				if page_pos, ok := page_ref_map[norm_id]; ok {
					fit_str := ""
					if len(arr) > 1 {
						var fit_name string
						if json.Unmarshal(arr[1], &fit_name) == nil {
							fit_str = strings.TrimPrefix(fit_name, "/")
						}
					}
					return page_pos, fit_str
				}
			}
			raw_trimmed := strings.TrimPrefix(strings.TrimSpace(target_ref), "obj:")
			if page_pos, ok := page_ref_map[raw_trimmed]; ok {
				return page_pos, ""
			}
		}
	}

	var dest_name string
	if json.Unmarshal(raw, &dest_name) == nil {
		clean := cleanText(dest_name)
		if page_pos, ok := page_ref_map[clean]; ok {
			return page_pos, ""
		}
	}

	return 0, ""
}

func parseBox(raw json.RawMessage) ([4]float64, bool) {
	var nums []json.Number
	if json.Unmarshal(raw, &nums) == nil && len(nums) == 4 {
		var box [4]float64
		for i := 0; i < 4; i++ {
			f, err := nums[i].Float64()
			if err != nil {
				return [4]float64{}, false
			}
			box[i] = f
		}
		return box, true
	}
	return [4]float64{}, false
}

func cleanText(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.Trim(s, "\"")
	s = strings.TrimPrefix(s, "u:")
	return s
}
