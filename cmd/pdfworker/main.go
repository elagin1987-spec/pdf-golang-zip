package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"pdf-golang-zip/internal/compression"
	"pdf-golang-zip/internal/ghostscript"
	"pdf-golang-zip/internal/qpdf"
	"pdf-golang-zip/internal/worker"
)

func main() {
	// All generated job files (including tool-created files) are private.
	syscall.Umask(0077)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	var req worker.Request
	dec := json.NewDecoder(io.LimitReader(os.Stdin, 16384))
	dec.DisallowUnknownFields()
	response := worker.Response{}
	if err := dec.Decode(&req); err != nil {
		response.Code = "worker_protocol"
		response.Status = 500
	} else {
		runner := worker.NewLinuxRunner(false) // gs/qpdf inherit the pdfworker job group.
		svc := compression.NewService(qpdf.NewClient(req.QPDFPath, runner), ghostscript.NewClient(req.GSPath, runner))
		result, err := svc.Compress(ctx, compression.CompressRequest{InputPath: req.InputPath, WorkDir: req.WorkDir, Profile: compression.Profile(req.Profile), MaxPages: req.MaxPages})
		if err == nil {
			response.Result, _ = json.Marshal(result)
		} else {
			response = errorResponse(err)
		}
	}
	_ = json.NewEncoder(os.Stdout).Encode(response)
}

func errorResponse(err error) worker.Response {
	response := worker.Response{}
	var pathErr *os.PathError
	var refusal *compression.PolicyViolationError
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		response.Status = 504
		response.Code = "processing_timeout"
	case errors.Is(err, exec.ErrNotFound), errors.As(err, &pathErr) && pathErr.Op == "fork/exec":
		response.Status = 503
		response.Code = "tool_unavailable"
	case errors.As(err, &refusal):
		response.Status = 422
		response.Code = refusal.Code
	case errors.Is(err, compression.ErrPreflightFailed):
		response.Status = 422
		response.Code = "invalid_pdf"
	case errors.Is(err, qpdf.ErrDocumentLimit), errors.Is(err, qpdf.ErrStreamExceedsLimit):
		response.Status = 422
		response.Code = "document_limit"
	default:
		response.Status = 500
		response.Code = "processing_failed"
	}
	return response
}
