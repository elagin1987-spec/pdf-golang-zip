package compression

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"pdf-golang-zip/internal/ghostscript"
	"pdf-golang-zip/internal/qpdf"
	"pdf-golang-zip/internal/worker"
)

func TestCandidateInfrastructureErrors(t *testing.T) {
	for _, stage := range []string{"--check", "--json=2"} {
		for _, cause := range []error{exec.ErrNotFound, errors.New("process crash"), &os.PathError{Op: "open", Path: "private", Err: os.ErrPermission}, qpdf.ErrDocumentLimit, qpdf.ErrStreamExceedsLimit, context.Canceled, context.DeadlineExceeded} {
			t.Run(stage+"/"+cause.Error(), func(t *testing.T) {
				dir, analysis := setupValidationEnv(t)
				path := filepath.Join(dir, "candidate.pdf")
				if err := os.WriteFile(path, []byte("candidate"), 0600); err != nil {
					t.Fatal(err)
				}
				runner := &fakeRunner{handler: func(_ context.Context, _ worker.CommandOptions, _ string, args ...string) (*worker.Result, error) {
					if args[0] == stage {
						return nil, cause
					}
					return &worker.Result{Stdout: []byte(sampleValidJSON)}, nil
				}}
				_, err := ValidateCandidate(context.Background(), qpdf.NewClient("fake", runner), nil, analysis, "", path, EngineGhostscript, dir)
				if !errors.Is(err, cause) {
					t.Fatalf("infrastructure error hidden: %v", err)
				}
			})
		}
	}
}

// Faults happen after the required lossless candidate is already valid.
func TestServiceDoesNotHideCandidateInfrastructureErrors(t *testing.T) {
	for _, target := range []string{"gs-candidate.pdf", "gs-structural.pdf"} {
		for _, stage := range []string{"--check", "--json=2", "--compress-streams=y"} {
			if target == "gs-candidate.pdf" && stage == "--compress-streams=y" {
				continue
			}
			for _, cause := range []error{exec.ErrNotFound, errors.New("tool process failure"), qpdf.ErrDocumentLimit, context.Canceled} {
				t.Run(target+"/"+stage+"/"+cause.Error(), func(t *testing.T) {
					input, dir := setupServiceTestDoc(t, 1000, nil)
					qp := &fakeRunner{handler: func(_ context.Context, _ worker.CommandOptions, _ string, args ...string) (*worker.Result, error) {
						if args[0] == stage && filepath.Base(args[len(args)-1]) == target {
							return nil, cause
						}
						switch args[0] {
						case "--is-encrypted":
							return &worker.Result{ExitCode: 2}, nil
						case "--json=2":
							return &worker.Result{Stdout: []byte(sampleValidJSON)}, nil
						case "--compress-streams=y":
							if err := os.WriteFile(args[len(args)-1], make([]byte, 100), 0600); err != nil {
								return nil, err
							}
						}
						return &worker.Result{}, nil
					}}
					gs := &fakeRunner{handler: func(_ context.Context, _ worker.CommandOptions, _ string, args ...string) (*worker.Result, error) {
						for _, a := range args {
							if strings.HasPrefix(a, "-sOutputFile=") {
								path := strings.TrimPrefix(a, "-sOutputFile=")
								if err := os.WriteFile(path, []byte("same text"), 0600); err != nil {
									return nil, err
								}
							}
						}
						return &worker.Result{}, nil
					}}
					res, err := NewService(qpdf.NewClient("fake", qp), ghostscript.NewClient("fake", gs)).Compress(context.Background(), CompressRequest{InputPath: input, WorkDir: dir, Profile: ProfileBalanced})
					if res != nil || !errors.Is(err, cause) {
						t.Fatalf("failure hidden by fallback: %+v / %v", res, err)
					}
				})
			}
		}
	}
}

func TestServiceXMPFailureStopsRewrite(t *testing.T) {
	input, dir := setupServiceTestDoc(t, 1000, nil)
	cause := errors.New("cannot read declared metadata")
	data := strings.Replace(sampleValidJSON, `"/Pages":"2 0 R"`, `"/Pages":"2 0 R","/Metadata":"10 0 R"`, 1)
	runner := &fakeRunner{handler: func(_ context.Context, _ worker.CommandOptions, _ string, args ...string) (*worker.Result, error) {
		switch args[0] {
		case "--check":
			return &worker.Result{}, nil
		case "--is-encrypted":
			return &worker.Result{ExitCode: 2}, nil
		case "--json=2":
			return &worker.Result{Stdout: []byte(data)}, nil
		case "--show-object=10,0":
			return nil, cause
		default:
			t.Fatalf("rewrite after unreadable XMP: %v", args)
			return nil, nil
		}
	}}
	res, err := NewService(qpdf.NewClient("fake", runner), nil).Compress(context.Background(), CompressRequest{InputPath: input, WorkDir: dir, Profile: ProfileBalanced})
	if res != nil || !errors.Is(err, cause) {
		t.Fatalf("XMP read failure ignored: %+v / %v", res, err)
	}
}
