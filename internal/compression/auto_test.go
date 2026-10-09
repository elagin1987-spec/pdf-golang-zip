package compression

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"pdf-golang-zip/internal/ghostscript"
	"pdf-golang-zip/internal/qpdf"
	"pdf-golang-zip/internal/worker"
	"strings"
	"testing"
)

func TestAutoCandidateSelection(t *testing.T) {
	for _, tc := range []struct {
		name                                                             string
		jpegSize, gsSize, threshold                                      int
		jpegFails, jpegInvalid, gsFails, special, complex, sourceWarning bool
		wantGS                                                           bool
		wantSize                                                         int64
	}{
		{name: "large savings", jpegSize: 15000, gsSize: 10000, threshold: 20, wantSize: 15000},
		{name: "exact threshold", jpegSize: 80000, gsSize: 10000, threshold: 20, wantSize: 80000},
		{name: "below threshold", jpegSize: 80001, gsSize: 50000, threshold: 20, wantGS: true, wantSize: 50000},
		{name: "GS bigger", jpegSize: 90000, gsSize: 98000, threshold: 20, wantGS: true, wantSize: 90000},
		{name: "JPEG failed", jpegFails: true, gsSize: 50000, threshold: 20, wantGS: true, wantSize: 50000},
		{name: "JPEG invalid", jpegInvalid: true, jpegSize: 10000, gsSize: 50000, threshold: 20, wantGS: true, wantSize: 50000},
		{name: "GS failed", jpegSize: 90000, threshold: 20, gsFails: true, wantGS: true, wantSize: 90000},
		{name: "no reduction", jpegSize: 110000, gsSize: 120000, threshold: 20, wantGS: true, wantSize: 100000},
		{name: "custom threshold", jpegSize: 50000, gsSize: 25000, threshold: 60, wantGS: true, wantSize: 25000},
		{name: "special lossless", special: true, threshold: 20, wantSize: 99000},
		{name: "complex colors", complex: true, threshold: 20, wantSize: 99000},
		{name: "source warnings", sourceWarning: true, threshold: 20, wantSize: 100000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input, dir := setupServiceTestDoc(t, 100000, nil)
			doc := sampleValidJSON
			if tc.special {
				doc = strings.Replace(doc, `"/Pages":"2 0 R"`, `"/Pages":"2 0 R","/AcroForm":{"/Fields":[{"/T":"u:test"}]}`, 1)
			}
			if tc.complex {
				doc = sampleCMYKJSON
			}
			jpegCalls, gsCalls := 0, 0
			qr := &fakeRunner{handler: func(_ context.Context, _ worker.CommandOptions, _ string, args ...string) (*worker.Result, error) {
				switch args[0] {
				case "--is-encrypted":
					return &worker.Result{ExitCode: 2}, nil
				case "--check":
					if tc.sourceWarning && args[1] == input {
						return &worker.Result{ExitCode: 3}, errors.New("source warning")
					}
				case "--json=2":
					candidateDoc := doc
					if tc.jpegInvalid && strings.HasSuffix(args[len(args)-1], "qpdf-images.pdf") {
						candidateDoc = strings.Replace(doc, "216,216", "217,217", -1)
					}
					return &worker.Result{Stdout: []byte(candidateDoc)}, nil
				case "--compress-streams=y":
					size := 99000
					if tc.name == "no reduction" {
						size = 110000
					}
					if strings.HasSuffix(args[len(args)-2], "gs-candidate.pdf") {
						size = tc.gsSize
					}
					return &worker.Result{}, os.WriteFile(args[len(args)-1], make([]byte, size), 0600)
				default:
					if args[0] == input {
						jpegCalls++
						if !strings.Contains(strings.Join(args, " "), "--jpeg-quality=70") {
							t.Fatal(args)
						}
						if tc.jpegFails {
							return &worker.Result{ExitCode: 3}, errors.New("JPEG warning")
						}
						return &worker.Result{}, os.WriteFile(args[len(args)-1], make([]byte, tc.jpegSize), 0600)
					}
				}
				return &worker.Result{}, nil
			}}
			gr := &fakeRunner{handler: func(_ context.Context, _ worker.CommandOptions, _ string, args ...string) (*worker.Result, error) {
				output := ""
				for _, arg := range args {
					if strings.HasPrefix(arg, "-sOutputFile=") {
						output = strings.TrimPrefix(arg, "-sOutputFile=")
					}
				}
				if strings.HasSuffix(output, ".txt") {
					return &worker.Result{}, os.WriteFile(output, []byte("same text"), 0600)
				}
				gsCalls++
				joined := strings.Join(args, " ")
				if !strings.Contains(joined, "-dColorImageResolution=200") || args[len(args)-1] != input {
					t.Fatal("GS must use 200 DPI and original", args)
				}
				if tc.gsFails {
					return &worker.Result{ExitCode: 1}, errors.New("GS failed")
				}
				return &worker.Result{}, os.WriteFile(output, make([]byte, tc.gsSize), 0600)
			}}
			auto := DefaultAutoSettings()
			auto.MinSavedPercent = tc.threshold
			res, err := NewService(qpdf.NewClient("qpdf", qr), ghostscript.NewClient("gs", gr)).Compress(context.Background(), CompressRequest{InputPath: input, WorkDir: dir, Profile: ProfileAuto, Auto: &auto, AllowSpecialTypes: true})
			if err != nil {
				t.Fatal(err)
			}
			if res.OutputBytes != tc.wantSize || (gsCalls > 0) != tc.wantGS {
				t.Fatalf("result=%+v GS calls=%d", res, gsCalls)
			}
			if (tc.special || tc.complex || tc.sourceWarning) && jpegCalls != 0 {
				t.Fatal("lossy rewrite of protected input")
			}
			if res.OutputBytes == 100000 && res.OutputPath != input {
				t.Fatal("original fallback changed")
			}
			if res.OutputBytes < 100000 && filepath.Dir(res.OutputPath) != dir {
				t.Fatal("output outside work dir")
			}
		})
	}
}

func TestAutoSettingsValidation(t *testing.T) {
	for _, s := range []AutoSettings{{-1, 20, 200}, {101, 20, 200}, {70, -1, 200}, {70, 101, 200}, {70, 20, 300}} {
		if s.Validate() == nil {
			t.Fatal("accepted", s)
		}
	}
	for _, s := range []AutoSettings{DefaultAutoSettings(), {0, 0, 150}, {100, 100, 200}} {
		if err := s.Validate(); err != nil {
			t.Fatal(s, err)
		}
	}
}
