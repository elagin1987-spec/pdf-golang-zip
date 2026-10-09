package worker

import "encoding/json"

// Protocol is private server-to-worker IPC, never populated with client paths or flags.
type Request struct {
	InputPath         string `json:"input_path"`
	WorkDir           string `json:"work_dir"`
	Profile           string `json:"profile"`
	QPDFPath          string `json:"qpdf_path"`
	GSPath            string `json:"gs_path"`
	MaxPages          int    `json:"max_pages"`
	AllowSpecialTypes bool   `json:"allow_special_types,omitempty"`
}
type Response struct {
	Result json.RawMessage `json:"result,omitempty"`
	Code   string          `json:"code,omitempty"`
	Status int             `json:"status,omitempty"`
}
