package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"pdf-golang-zip/internal/compression"
	"pdf-golang-zip/internal/config"
	"pdf-golang-zip/internal/storage"
	"pdf-golang-zip/internal/worker"
)

type APIError struct {
	Status int
	Code   string
}

func (e *APIError) Error() string { return e.Code }

type Processor interface {
	Process(context.Context, *storage.Job, compression.Profile) (*compression.Result, error)
}
type ProcessWorker struct{ Config config.Config }

func (p ProcessWorker) Process(ctx context.Context, job *storage.Job, profile compression.Profile) (*compression.Result, error) {
	c := p.Config
	req := worker.Request{
		AutoJPEGQuality:     &c.Auto.JPEGQuality,
		AutoMinSavedPercent: &c.Auto.MinSavedPercent,
		AutoDPI:             &c.Auto.DPI,
		InputPath:           filepath.Join(job.Dir, "input.pdf"),
		WorkDir:             job.Dir,
		Profile:             string(profile),
		QPDFPath:            c.QPDFPath,
		GSPath:              c.GSPath,
		MaxPages:            c.MaxPages,
		AllowSpecialTypes:   c.AllowSpecialTypes,
	}
	payload, _ := json.Marshal(req)
	jobCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	watchDone := make(chan struct{})
	stopWatch := make(chan struct{})
	go func() {
		defer close(watchDone)
		tick := time.NewTicker(20 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-stopWatch:
				return
			case <-jobCtx.Done():
				return
			case <-tick.C:
				if err := job.CheckSize(c.MaxJobBytes); err != nil {
					cancel(err)
					return
				}
			}
		}
	}()
	env := append(os.Environ(), "GOMEMLIMIT=384MiB", "TMPDIR="+job.Dir, "XDG_CACHE_HOME="+filepath.Join(job.Dir, "cache"))
	res, err := worker.NewLinuxRunner(true).RunWithOptions(jobCtx, worker.CommandOptions{Dir: job.Dir, Env: env, Stdin: bytes.NewReader(payload), MaxStdoutBytes: 16384}, c.WorkerPath)
	close(stopWatch)
	<-watchDone
	if cause := context.Cause(jobCtx); cause != nil {
		if errors.Is(cause, storage.ErrJobLimit) {
			return nil, &APIError{500, "job_storage_limit"}
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, &APIError{500, "storage_failed"}
	}
	if err != nil {
		return nil, &APIError{503, "worker_unavailable"}
	}
	if err := job.CheckSize(c.MaxJobBytes); err != nil {
		return nil, &APIError{500, "job_storage_limit"}
	}
	var reply worker.Response
	if res.Truncated || json.Unmarshal(res.Stdout, &reply) != nil {
		return nil, &APIError{500, "worker_protocol"}
	}
	if reply.Code != "" {
		return nil, &APIError{reply.Status, reply.Code}
	}
	var result compression.Result
	if json.Unmarshal(reply.Result, &result) != nil {
		return nil, &APIError{500, "worker_protocol"}
	}
	// A compromised/malformed IPC result cannot select paths outside its private job.
	rel, err := filepath.Rel(job.Dir, result.OutputPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return nil, &APIError{500, "worker_protocol"}
	}
	info, err := os.Lstat(result.OutputPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() != result.OutputBytes || result.OutputBytes <= 0 || result.OutputBytes > result.OriginalBytes {
		return nil, &APIError{500, "output_validation"}
	}
	return &result, nil
}
