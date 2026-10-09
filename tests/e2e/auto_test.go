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

func TestRuntimeAutoProfile(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  []string
	}{
		{"default", nil},
		{"200 DPI forced comparison", []string{"AUTO_MIN_SAVED_PERCENT=100", "AUTO_JPEG_QUALITY=90"}},
		{"150 DPI forced comparison", []string{"AUTO_DPI=150", "AUTO_MIN_SAVED_PERCENT=100", "AUTO_JPEG_QUALITY=90"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := startAPI(t, tc.env...)
			for _, fixture := range []string{"flate-rgb-600dpi.pdf", "jpeg-600dpi.pdf"} {
				input, err := os.ReadFile(filepath.Join("../../testdata/pdf", fixture))
				if err != nil {
					t.Fatal(err)
				}
				r := post(t, p, input, "auto")
				output, err := io.ReadAll(r.Body)
				_ = r.Body.Close()
				if err != nil || r.StatusCode != 200 || len(output) > len(input) {
					t.Fatalf("%s: %d %v %s", fixture, r.StatusCode, err, output)
				}
				path := filepath.Join(t.TempDir(), "output.pdf")
				if err := os.WriteFile(path, output, 0600); err != nil {
					t.Fatal(err)
				}
				if log, err := exec.Command("qpdf", "--check", path).CombinedOutput(); err != nil {
					t.Fatal(err, string(log))
				}
				if tc.name == "default" && r.Header.Get("X-Optimization-Engine") != "qpdf" {
					t.Fatal("expected JPEG candidate", r.Header)
				}
				if tc.name != "default" && fixture == "flate-rgb-600dpi.pdf" && r.Header.Get("X-Optimization-Engine") != "ghostscript" {
					t.Fatal("expected GS fallback", r.Header)
				}
				t.Log(fixture, r.Header.Get("X-Optimization-Engine"), r.Header.Get("X-Saved-Percent"), r.Header.Get("X-Compression-Warnings"))
				wait(t, func() bool { return jobsCount(p) == 0 })
			}
		})
	}
	p := startAPI(t, "ENABLE_LOSSY=false")
	r := post(t, p, minimalPDF(false), "auto")
	data, _ := io.ReadAll(r.Body)
	_ = r.Body.Close()
	if r.StatusCode != 400 || !strings.Contains(string(data), "profile_disabled") {
		t.Fatal(r.StatusCode, string(data))
	}
	pSpecial := startAPI(t, "ALLOW_SPECIAL_TYPES=true")
	input, err := os.ReadFile("../../testdata/pdf/special-acroform.pdf")
	if err != nil {
		t.Fatal(err)
	}
	r = post(t, pSpecial, input, "auto")
	data, _ = io.ReadAll(r.Body)
	_ = r.Body.Close()
	if r.StatusCode != 200 || r.Header.Get("X-Optimization-Engine") == "ghostscript" || !strings.Contains(r.Header.Get("X-Compression-Warnings"), "interactive_forms_preserved") {
		t.Fatal(r.StatusCode, r.Header)
	}
	if r.Header.Get("X-Compression-Status") == "unchanged" && !bytes.Equal(input, data) {
		t.Fatal("original changed")
	}
}
