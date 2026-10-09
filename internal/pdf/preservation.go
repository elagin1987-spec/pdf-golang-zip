package pdf

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"pdf-golang-zip/internal/qpdf"
)

// PreservationFingerprint compares the reachable catalog graph independently of
// object numbering. Only stream storage keys may change during a qpdf rewrite.
// Embedded payloads are compared after decoding, with bounded reads and totals.
func PreservationFingerprint(ctx context.Context, client *qpdf.Client, path string, doc *qpdf.JSONOutput) ([32]byte, error) {
	var zero [32]byte
	trailer, ok := doc.Trailer()
	if !ok {
		return zero, fmt.Errorf("preservation: missing trailer")
	}
	var root string
	if json.Unmarshal(trailer["/Root"], &root) != nil || root == "" {
		return zero, fmt.Errorf("preservation: missing root")
	}
	ids := map[string]int{}
	queue := []string{}
	embedded := map[string]bool{}
	nodes := 0
	var decodedBytes int64
	var walk func(any, int, bool) (any, error)
	walk = func(value any, depth int, payload bool) (any, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		nodes++
		if depth > MaxOutlineDepth || nodes > qpdf.MaxDocumentObjects {
			return nil, qpdf.ErrDocumentLimit
		}
		switch v := value.(type) {
		case string:
			if !strings.HasSuffix(v, " R") {
				return v, nil
			}
			id, err := qpdf.NormalizeObjectID(v)
			if err != nil {
				return v, nil // qpdf string values such as "u:letter R" are not references.
			}
			if payload {
				embedded[id] = true
			}
			label, exists := ids[id]
			if !exists {
				label = len(queue)
				ids[id] = label
				queue = append(queue, id)
			}
			return map[string]any{"reference": label}, nil
		case []any:
			out := make([]any, len(v))
			for i, child := range v {
				var err error
				out[i], err = walk(child, depth+1, payload)
				if err != nil {
					return nil, err
				}
			}
			return out, nil
		case map[string]any:
			out := map[string]any{}
			keys := make([]string, 0, len(v))
			for key := range v {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				child, err := walk(v[key], depth+1, payload || key == "/EF")
				if err != nil {
					return nil, err
				}
				out[key] = child
			}
			return out, nil
		default:
			return v, nil
		}
	}
	if _, err := walk(root, 0, false); err != nil {
		return zero, err
	}
	canonical := []any{}
	// References are queued in sorted-key order; cycles reuse a stable label.
	for i := 0; i < len(queue); i++ {
		obj, found := doc.FindObject(queue[i])
		if !found {
			return zero, fmt.Errorf("preservation: missing object")
		}
		var value any
		stream := obj.Stream != nil
		if stream {
			dict := map[string]json.RawMessage{}
			for key, raw := range obj.Stream.Dict {
				if key != "/Length" && key != "/Filter" && key != "/DecodeParms" && key != "/DL" {
					dict[key] = raw
				}
			}
			data, err := json.Marshal(dict)
			if err != nil {
				return zero, err
			}
			dec := json.NewDecoder(bytes.NewReader(data))
			dec.UseNumber()
			if err := dec.Decode(&value); err != nil {
				return zero, err
			}
		} else {
			dec := json.NewDecoder(bytes.NewReader(obj.RawValue))
			dec.UseNumber()
			if err := dec.Decode(&value); err != nil {
				return zero, err
			}
		}
		normalized, err := walk(value, 0, false)
		if err != nil {
			return zero, err
		}
		canonical = append(canonical, map[string]any{"stream": stream, "value": normalized})
	}
	// /EF itself may be an indirect dictionary; propagate payload membership
	// through its descendants even if a container was discovered before /EF.
	processed := map[string]bool{}
	for {
		pending := ""
		for id := range embedded {
			if !processed[id] {
				pending = id
				break
			}
		}
		if pending == "" {
			break
		}
		processed[pending] = true
		obj, found := doc.FindObject(pending)
		if !found {
			return zero, fmt.Errorf("preservation: missing payload object")
		}
		if obj.Stream != nil {
			continue
		}
		var value any
		dec := json.NewDecoder(bytes.NewReader(obj.RawValue))
		dec.UseNumber()
		if err := dec.Decode(&value); err != nil {
			return zero, err
		}
		if _, err := walk(value, 0, true); err != nil {
			return zero, err
		}
	}
	// Hash payloads only after discovery completes, including references found later.
	for i, id := range queue {
		obj, _ := doc.FindObject(id)
		if obj.Stream == nil {
			continue
		}
		var typ string
		_ = json.Unmarshal(obj.Stream.Dict["/Type"], &typ)
		if !embedded[id] && typ != "/EmbeddedFile" {
			continue
		}
		data, err := client.ReadStream(ctx, path, id, qpdf.DefaultStreamLimit)
		if err != nil {
			return zero, fmt.Errorf("preservation: embedded payload: %w", err)
		}
		decodedBytes += int64(len(data))
		if decodedBytes > 300<<20 {
			return zero, qpdf.ErrStreamExceedsLimit
		}
		canonical[i].(map[string]any)["payload_sha256"] = fmt.Sprintf("%x", sha256.Sum256(data))
	}
	data, err := json.Marshal(canonical)
	if err != nil {
		return zero, err
	}
	return sha256.Sum256(data), nil
}
