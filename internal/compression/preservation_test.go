package compression

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"pdf-golang-zip/internal/ghostscript"
	"pdf-golang-zip/internal/qpdf"
	"pdf-golang-zip/internal/worker"
)

func TestSpecialServicePreservesOrFallsBack(t *testing.T) {
	for _, change := range []bool{false, true} {
		t.Run(map[bool]string{false: "preserved", true: "changed form"}[change], func(t *testing.T) {
			input, dir := setupServiceTestDoc(t, 10000, nil)
			original := strings.Replace(sampleValidJSON, `"/Pages":"2 0 R"`, `"/Pages":"2 0 R","/AcroForm":{"/Fields":[{"/T":"u:original"}]}`, 1)
			candidate := original
			if change {
				candidate = strings.Replace(original, "u:original", "u:changed", 1)
			}
			runner := &fakeRunner{handler: func(_ context.Context, _ worker.CommandOptions, _ string, args ...string) (*worker.Result, error) {
				switch args[0] {
				case "--is-encrypted":
					return &worker.Result{ExitCode: 2}, nil
				case "--json=2":
					doc := candidate
					if args[len(args)-1] == input {
						doc = original
					}
					return &worker.Result{Stdout: []byte(doc)}, nil
				case "--compress-streams=y":
					if err := os.WriteFile(args[len(args)-1], make([]byte, 5000), 0600); err != nil {
						return nil, err
					}
				}
				return &worker.Result{}, nil
			}}
			gs := &fakeRunner{handler: func(context.Context, worker.CommandOptions, string, ...string) (*worker.Result, error) {
				t.Fatal("Ghostscript invoked for special PDF")
				return nil, nil
			}}
			svc := NewService(qpdf.NewClient("qpdf", runner), ghostscript.NewClient("gs", gs))
			result, err := svc.Compress(context.Background(), CompressRequest{InputPath: input, WorkDir: dir, Profile: ProfileBalanced, AllowSpecialTypes: true})
			if err != nil {
				t.Fatal(err)
			}
			if !change && result.Engine != EngineQPDF {
				t.Fatalf("expected qpdf: %+v", result)
			}
			if change && (result.Status != StatusUnchanged || result.OutputPath != input || !strings.Contains(strings.Join(result.Warnings, ","), "special_features_changed")) {
				t.Fatalf("lost form accepted: %+v", result)
			}
		})
	}
}

func TestConfiguredPageLimitRemainsEffective(t *testing.T) {
	input, dir := setupServiceTestDoc(t, 1000, nil)
	var raw map[string]any
	if err := json.Unmarshal([]byte(sampleValidJSON), &raw); err != nil {
		t.Fatal(err)
	}
	pages := raw["pages"].([]any)
	raw["pages"] = append(pages, pages[0])
	doc, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{handler: func(_ context.Context, _ worker.CommandOptions, _ string, args ...string) (*worker.Result, error) {
		if args[0] == "--is-encrypted" {
			return &worker.Result{ExitCode: 2}, nil
		}
		if args[0] == "--json=2" {
			return &worker.Result{Stdout: doc}, nil
		}
		if args[0] == "--compress-streams=y" {
			t.Fatal("started compression beyond configured page limit")
		}
		return &worker.Result{}, nil
	}}
	_, err = NewService(qpdf.NewClient("qpdf", runner), nil).Compress(context.Background(), CompressRequest{InputPath: input, WorkDir: dir, Profile: ProfileLossless, MaxPages: 1})
	if !errors.Is(err, qpdf.ErrDocumentLimit) {
		t.Fatal("configured page limit ignored", err)
	}
}

func TestSourceWarningsPreserveExactOriginal(t *testing.T) {
	input, dir := setupServiceTestDoc(t, 1000, nil)
	runner := &fakeRunner{handler: func(_ context.Context, _ worker.CommandOptions, _ string, args ...string) (*worker.Result, error) {
		switch args[0] {
		case "--is-encrypted":
			return &worker.Result{ExitCode: 2}, nil
		case "--check":
			return &worker.Result{ExitCode: 3}, errors.Join(qpdf.ErrCheckFailed, qpdf.ErrCheckWarning)
		case "--json=2":
			return &worker.Result{Stdout: []byte(sampleValidJSON)}, nil
		default:
			t.Fatal("rewrite attempted on warning-bearing source", args)
			return nil, nil
		}
	}}
	res, err := NewService(qpdf.NewClient("qpdf", runner), nil).Compress(context.Background(), CompressRequest{InputPath: input, WorkDir: dir, Profile: ProfileBalanced})
	if err != nil {
		t.Fatal(err)
	}
	if res.Engine != EngineOriginal || res.Status != StatusUnchanged || res.OutputPath != input || res.OutputBytes != 1000 {
		t.Fatalf("original not preserved: %+v", res)
	}
	if !strings.Contains(strings.Join(res.Warnings, ","), "compression_skipped_source_warnings") {
		t.Fatal(res.Warnings)
	}
}
