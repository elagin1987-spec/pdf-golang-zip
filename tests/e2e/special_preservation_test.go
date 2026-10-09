package e2e

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeSpecialPreservation(t *testing.T) {
	p := startAPI(t, "ALLOW_SPECIAL_TYPES=true")
	for _, tc := range []struct{ file, warning string }{
		{"special-attachments.pdf", "attachments_preserved"},
		{"special-tagged.pdf", "tagged_structure_preserved"},
		{"special-acroform.pdf", "interactive_forms_preserved"},
		{"special-layers.pdf", "layers_preserved"},
		{"special-javascript.pdf", "javascript_preserved"},
	} {
		t.Run(tc.file, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join("../../testdata/pdf", tc.file))
			if err != nil {
				t.Fatal(err)
			}
			response := post(t, p, input, "balanced")
			output, err := io.ReadAll(response.Body)
			_ = response.Body.Close()
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != 200 {
				t.Fatalf("%d: %s", response.StatusCode, output)
			}
			if len(output) > len(input) || response.Header.Get("X-Optimization-Engine") == "ghostscript" {
				t.Fatal("unsafe special rewrite", response.Header)
			}
			warnings := response.Header.Get("X-Compression-Warnings")
			if !strings.Contains(warnings, tc.warning) || strings.Contains(warnings, "flattened") || strings.Contains(warnings, "stripped") {
				t.Fatal("wrong feature warnings", warnings)
			}
			if response.Header.Get("X-Compression-Status") == "unchanged" && !bytes.Equal(input, output) {
				t.Fatal("original changed")
			}
			path := filepath.Join(t.TempDir(), "output.pdf")
			if err := os.WriteFile(path, output, 0600); err != nil {
				t.Fatal(err)
			}
			if diagnostics, err := exec.Command("qpdf", "--check", path).CombinedOutput(); err != nil {
				t.Fatal(err, string(diagnostics))
			}
			wait(t, func() bool { return jobsCount(p) == 0 })
		})
	}
}
