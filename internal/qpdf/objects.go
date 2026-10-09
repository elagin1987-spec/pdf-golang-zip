package qpdf

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ObjectEntry represents a PDF object in qpdf JSON v2 output.
type ObjectEntry struct {
	RawValue json.RawMessage `json:"value,omitempty"`
	Stream   *StreamEntry    `json:"stream,omitempty"`
}

// StreamEntry represents the stream portion of an indirect object.
type StreamEntry struct {
	Dict map[string]json.RawMessage `json:"dict"`
}

// AsDict attempts to decode RawValue as a JSON dictionary of PDF keys.
func (e *ObjectEntry) AsDict() (map[string]json.RawMessage, bool) {
	if len(e.RawValue) == 0 {
		return nil, false
	}
	var dict map[string]json.RawMessage
	err := json.Unmarshal(e.RawValue, &dict)
	if err != nil {
		return nil, false
	}
	return dict, true
}

// AsArray attempts to decode RawValue as a JSON array.
func (e *ObjectEntry) AsArray() ([]json.RawMessage, bool) {
	if len(e.RawValue) == 0 {
		return nil, false
	}
	var arr []json.RawMessage
	err := json.Unmarshal(e.RawValue, &arr)
	if err != nil {
		return nil, false
	}
	return arr, true
}

// GetDict returns the object's dictionary whether it is a direct dictionary value
// or the dictionary of a stream object.
func (e *ObjectEntry) GetDict() map[string]json.RawMessage {
	if dict, ok := e.AsDict(); ok {
		return dict
	}
	if e.Stream != nil && e.Stream.Dict != nil {
		return e.Stream.Dict
	}
	return nil
}

// NormalizeObjectID standardizes various object ID representations into the "obj,gen"
// format expected by qpdf --show-object (e.g. "4,0", or "trailer").
func NormalizeObjectID(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("empty object id")
	}
	if strings.EqualFold(s, "trailer") {
		return "trailer", nil
	}
	s = strings.TrimPrefix(s, "obj:")
	s = strings.TrimSpace(s)

	if strings.Contains(s, ",") {
		parts := strings.Split(s, ",")
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid comma-separated object id: %s", raw)
		}
		obj_num, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		gen_num, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err1 != nil || err2 != nil || obj_num < 0 || gen_num < 0 {
			return "", fmt.Errorf("invalid object id numbers: %s", raw)
		}
		return fmt.Sprintf("%d,%d", obj_num, gen_num), nil
	}

	fields := strings.Fields(s)
	if len(fields) == 3 && fields[2] == "R" {
		obj_num, err1 := strconv.Atoi(fields[0])
		gen_num, err2 := strconv.Atoi(fields[1])
		if err1 != nil || err2 != nil || obj_num < 0 || gen_num < 0 {
			return "", fmt.Errorf("invalid indirect reference: %s", raw)
		}
		return fmt.Sprintf("%d,%d", obj_num, gen_num), nil
	}

	if len(fields) == 1 {
		obj_num, err := strconv.Atoi(fields[0])
		if err != nil || obj_num < 0 {
			return "", fmt.Errorf("invalid object number: %s", raw)
		}
		return fmt.Sprintf("%d,0", obj_num), nil
	}

	return "", fmt.Errorf("unrecognized object id format: %s", raw)
}
