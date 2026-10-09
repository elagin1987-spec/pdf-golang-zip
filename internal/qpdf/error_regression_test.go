package qpdf

import (
	"context"
	"errors"
	"os/exec"
	"testing"

	"pdf-golang-zip/internal/worker"
)

func TestToolErrorClassification(t *testing.T) {
	for _, operation := range []string{"check", "analyze", "write"} {
		for _, tt := range []struct {
			name    string
			res     *worker.Result
			err     error
			refusal bool
		}{
			{"document error", &worker.Result{ExitCode: 2}, errors.New("document error"), true},
			{"document warning", &worker.Result{ExitCode: 3}, errors.New("warning"), true},
			{"missing", nil, exec.ErrNotFound, false},
			{"crash", &worker.Result{ExitCode: -1}, errors.New("killed"), false},
		} {
			t.Run(operation+"/"+tt.name, func(t *testing.T) {
				c := NewClient("fake", &fakeRunner{runFunc: func(context.Context, worker.CommandOptions, string, ...string) (*worker.Result, error) {
					return tt.res, tt.err
				}})
				var err error
				switch operation {
				case "check":
					err = c.Check(context.Background(), "owned.pdf")
				case "analyze":
					_, err = c.Analyze(context.Background(), "owned.pdf")
				case "write":
					err = c.WriteLossless(context.Background(), "owned.pdf", "out.pdf")
				}
				if errors.Is(err, ErrCheckFailed) != tt.refusal {
					t.Fatalf("wrong classification: %v", err)
				}
				if !tt.refusal && !errors.Is(err, tt.err) {
					t.Fatalf("lost error chain: %v", err)
				}
			})
		}
	}
}

func TestInvalidJSONClassification(t *testing.T) {
	if _, err := ParseJSON([]byte(`{"broken"`)); !errors.Is(err, ErrInvalidJSON) {
		t.Fatal(err)
	}
	if _, err := ParseJSON([]byte(`{"version":1}`)); err == nil || errors.Is(err, ErrInvalidJSON) {
		t.Fatalf("schema version must be an internal error: %v", err)
	}
}
