package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"pdf-golang-zip/internal/compression"
	"pdf-golang-zip/internal/qpdf"
)

func TestErrorBoundary(t *testing.T) {
	for _, tt := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"missing tool in PATH", fmt.Errorf("private stderr: %w", exec.ErrNotFound), 503, "tool_unavailable"},
		{"missing absolute tool", &os.PathError{Op: "fork/exec", Path: "/missing", Err: os.ErrNotExist}, 503, "tool_unavailable"},
		{"lost input", &os.PathError{Op: "stat", Path: "/job/input.pdf", Err: os.ErrNotExist}, 500, "processing_failed"},
		{"required validation", fmt.Errorf("private PDF data"), 500, "processing_failed"},
		{"wrapped metadata tool missing", fmt.Errorf("metadata read: %w", exec.ErrNotFound), 503, "tool_unavailable"},
		{"metadata storage failure", fmt.Errorf("metadata read: %w", &os.PathError{Op: "open", Path: "private", Err: os.ErrPermission}), 500, "processing_failed"},
		{"JSON cap", fmt.Errorf("candidate analysis: %w", qpdf.ErrStreamExceedsLimit), 422, "document_limit"},
		{"nested action policy", &compression.PolicyViolationError{Code: "special_type_javascript", Reason: "private"}, 422, "special_type_javascript"},
		{"invalid source", compression.ErrPreflightFailed, 422, "invalid_pdf"},
		{"page limit", qpdf.ErrDocumentLimit, 422, "document_limit"},
		{"timeout", context.DeadlineExceeded, 504, "processing_timeout"},
		{"policy", &compression.PolicyViolationError{Code: "special_type_encrypted", Reason: "private"}, 422, "special_type_encrypted"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := errorResponse(tt.err)
			if r.Status != tt.status || r.Code != tt.code || r.Result != nil {
				t.Fatal(r)
			}
		})
	}
}
