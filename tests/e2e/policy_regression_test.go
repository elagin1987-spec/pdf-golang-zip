package e2e

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRuntimeNestedActionRegressions(t *testing.T) {
	p := startAPI(t)
	dir := "../../testdata/pdf/regressions"
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		File, SHA256, Code string
		HTTPStatus         int `json:"http_status"`
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 8 {
		t.Fatal("incomplete policy regression corpus")
	}
	for _, fixture := range fixtures {
		inputPath := filepath.Join(dir, fixture.File)
		input, err := os.ReadFile(inputPath)
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(input)) != fixture.SHA256 {
			t.Fatal("fixture checksum changed", fixture.File)
		}
		if out, err := exec.Command("qpdf", "--check", inputPath).CombinedOutput(); err != nil {
			t.Fatalf("invalid input fixture: %v %s", err, out)
		}
		for _, profile := range []string{"lossless", "balanced", "compact"} {
			t.Run(fixture.File+"/"+profile, func(t *testing.T) {
				r := post(t, p, input, profile)
				output, err := io.ReadAll(r.Body)
				_ = r.Body.Close()
				if err != nil {
					t.Fatal(err)
				}
				if r.StatusCode != fixture.HTTPStatus {
					t.Fatalf("status %d: %s", r.StatusCode, output)
				}
				if fixture.HTTPStatus == 422 {
					var reply struct{ Code string }
					if err := json.Unmarshal(output, &reply); err != nil || reply.Code != fixture.Code || r.Header.Get("Content-Type") != "application/json" {
						t.Fatalf("wrong refusal: %s", output)
					}
				} else {
					if len(output) > len(input) || r.Header.Get("Content-Type") != "application/pdf" {
						t.Fatal("invalid successful response")
					}
					outpath := filepath.Join(t.TempDir(), "returned.pdf")
					if err := os.WriteFile(outpath, output, 0600); err != nil {
						t.Fatal(err)
					}
					if out, err := exec.Command("qpdf", "--check", outpath).CombinedOutput(); err != nil {
						t.Fatalf("invalid returned PDF: %v %s", err, out)
					}
				}
				wait(t, func() bool { return jobsCount(p) == 0 })
			})
		}
	}
}
