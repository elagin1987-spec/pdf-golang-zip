package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"pdf-golang-zip/internal/compression"
	"pdf-golang-zip/internal/config"
	"pdf-golang-zip/internal/observability"
	"pdf-golang-zip/internal/storage"
	"pdf-golang-zip/internal/worker"
)

type Server struct {
	Config    config.Config
	Store     storage.Store
	Processor Processor
	Ready     func(context.Context) error
	Logger    *slog.Logger
	slots     chan struct{}
	ctx       context.Context
	stop      context.CancelFunc
}

func New(c config.Config, store storage.Store, processor Processor, logger *slog.Logger) *Server {
	ctx, stop := context.WithCancel(context.Background())
	s := &Server{Config: c, Store: store, Processor: processor, Logger: logger, slots: make(chan struct{}, c.MaxWorkers), ctx: ctx, stop: stop}
	s.Ready = s.checkReady
	return s
}
func (s *Server) Stop() { s.stop() }
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200); _, _ = io.WriteString(w, "ok\n") })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		id := observability.RequestID()
		w.Header().Set("X-Request-ID", id)
		if s.Ready(r.Context()) != nil {
			writeError(w, id, &APIError{503, "not_ready"})
			return
		}
		_, _ = io.WriteString(w, "ready\n")
	})
	mux.HandleFunc("POST /v1/compress", s.compress)
	return mux
}
func (s *Server) checkReady(ctx context.Context) error {
	if s.ctx.Err() != nil {
		return s.ctx.Err()
	}
	if err := s.Store.Writable(); err != nil {
		return err
	}
	if f, err := os.Stat(s.Config.WorkerPath); err != nil || !f.Mode().IsRegular() || f.Mode().Perm()&0111 == 0 {
		return errors.New("worker unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	tools := []struct{ path, version string }{{s.Config.QPDFPath, "12.4.2"}}
	if s.Config.EnableLossy {
		tools = append(tools, struct{ path, version string }{s.Config.GSPath, "10.08.0"})
	}
	for _, tool := range tools {
		res, err := worker.NewLinuxRunner(true).Run(ctx, tool.path, "--version")
		if err != nil || res.Truncated || !hasVersion(string(res.Stdout), tool.version) {
			return errors.New("tool version unavailable")
		}
	}
	return nil
}
func hasVersion(out, version string) bool {
	for _, v := range strings.Fields(out) {
		if v == version {
			return true
		}
	}
	return false
}
func writeError(w http.ResponseWriter, id string, e *APIError) {
	if e.Status < 400 || e.Status > 599 {
		e = &APIError{500, "internal_error"}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(e.Status)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": e.Code, "message": strings.ReplaceAll(e.Code, "_", " "), "request_id": id})
}
func uploadError(err error) *APIError {
	var max *http.MaxBytesError
	var path *os.PathError
	if errors.As(err, &max) {
		return &APIError{413, "request_too_large"}
	}
	if errors.As(err, &path) {
		return &APIError{500, "storage_failed"}
	}
	return &APIError{400, "invalid_multipart"}
}
func (s *Server) upload(w http.ResponseWriter, r *http.Request, job *storage.Job) (compression.Profile, error) {
	r.Body = http.MaxBytesReader(w, r.Body, s.Config.MaxRequestBytes)
	defer r.Body.Close()
	mr, err := r.MultipartReader()
	if err != nil {
		return "", &APIError{400, "invalid_multipart"}
	}
	seen := map[string]bool{}
	profile := compression.ProfileBalanced
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", uploadError(err)
		}
		name := part.FormName()
		if seen[name] || (name != "file" && name != "profile") {
			return "", &APIError{400, "invalid_fields"}
		}
		seen[name] = true
		switch name {
		case "profile":
			if part.FileName() != "" {
				return "", &APIError{400, "invalid_fields"}
			}
			val, err := io.ReadAll(io.LimitReader(part, 1025))
			if err != nil {
				return "", uploadError(err)
			}
			if len(val) > 1024 {
				return "", &APIError{400, "field_too_large"}
			}
			switch string(val) {
			case "balanced":
				profile = compression.ProfileBalanced
			case "compact":
				profile = compression.ProfileCompact
			case "lossless":
				profile = compression.ProfileLossless
			default:
				return "", &APIError{400, "invalid_profile"}
			}
		case "file":
			// Extension, MIME and original filename never determine format or paths.
			f, err := os.OpenFile(filepath.Join(job.Dir, "input.pdf"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if err != nil {
				return "", &APIError{500, "storage_failed"}
			}
			n, copyErr := io.Copy(f, io.LimitReader(part, s.Config.MaxFileBytes+1))
			closeErr := f.Close()
			if copyErr != nil {
				return "", uploadError(copyErr)
			}
			if closeErr != nil {
				return "", &APIError{500, "storage_failed"}
			}
			if n > s.Config.MaxFileBytes {
				return "", &APIError{413, "file_too_large"}
			}
			if n == 0 {
				return "", &APIError{400, "empty_file"}
			}
		}
		if err := part.Close(); err != nil {
			return "", uploadError(err)
		}
	}
	// Account for any epilogue as part of the request byte budget.
	if _, err := io.Copy(io.Discard, r.Body); err != nil {
		return "", uploadError(err)
	}
	if !seen["file"] {
		return "", &APIError{400, "missing_file"}
	}
	if !s.Config.EnableLossy && profile != compression.ProfileLossless {
		return "", &APIError{400, "profile_disabled"}
	}
	f, err := os.Open(filepath.Join(job.Dir, "input.pdf"))
	if err != nil {
		return "", &APIError{500, "storage_failed"}
	}
	head := make([]byte, 1024)
	n, readErr := f.Read(head)
	_ = f.Close()
	if readErr != nil && readErr != io.EOF {
		return "", &APIError{500, "storage_failed"}
	}
	if !bytes.Contains(head[:n], []byte("%PDF-")) {
		return "", &APIError{415, "not_pdf"}
	}
	if err := job.CheckSize(s.Config.MaxJobBytes); err != nil {
		return "", &APIError{500, "job_storage_limit"}
	}
	return profile, nil
}
func (s *Server) compress(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	id := observability.RequestID()
	w.Header().Set("X-Request-ID", id)
	status := 200
	code := ""
	var result *compression.Result
	defer func() {
		if s.Logger != nil {
			args := []any{"request_id", id, "status", status, "code", code, "duration_ms", time.Since(start).Milliseconds()}
			if result != nil {
				args = append(args, "input_bytes", result.OriginalBytes, "output_bytes", result.OutputBytes, "engine", result.Engine, "warnings", result.Warnings)
			}
			s.Logger.Info("compress", args...)
		}
	}()
	fail := func(e *APIError) { status = e.Status; code = e.Code; writeError(w, id, e) }
	if s.ctx.Err() != nil {
		fail(&APIError{503, "shutting_down"})
		return
	}
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		w.Header().Set("Retry-After", "1")
		fail(&APIError{503, "worker_busy"})
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	stop := context.AfterFunc(s.ctx, cancel)
	defer stop()
	r = r.WithContext(ctx)
	// Shutdown must interrupt incomplete uploads and stalled downloads too.
	interruptIO := context.AfterFunc(ctx, func() {
		controller := http.NewResponseController(w)
		_ = controller.SetReadDeadline(time.Now())
		_ = controller.SetWriteDeadline(time.Now())
	})
	defer interruptIO()
	if s.Ready(ctx) != nil {
		fail(&APIError{503, "tool_unavailable"})
		return
	}
	job, err := s.Store.NewJob()
	if err != nil {
		fail(&APIError{500, "storage_failed"})
		return
	}
	defer func() {
		if err := job.Cleanup(); err != nil && s.Logger != nil {
			s.Logger.Error("cleanup_failed", "request_id", id)
		}
	}()
	profile, err := s.upload(w, r, job)
	if err != nil {
		var api *APIError
		if errors.As(err, &api) {
			fail(api)
		} else {
			fail(&APIError{500, "internal_error"})
		}
		return
	}
	// Processing deadline starts only once the entire upload has been validated.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(s.Config.WriteTimeout))
	processCtx, processCancel := context.WithTimeout(ctx, s.Config.ProcessingTimeout)
	defer processCancel()
	result, err = s.Processor.Process(processCtx, job, profile)
	if err != nil {
		var api *APIError
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			fail(&APIError{504, "processing_timeout"})
		case errors.Is(err, context.Canceled):
			status = 499
			code = "client_canceled"
			return
		case errors.As(err, &api):
			fail(api)
		default:
			fail(&APIError{500, "processing_failed"})
		}
		return
	}
	if ctx.Err() != nil {
		status = 499
		code = "client_canceled"
		return
	}
	f, err := os.Open(result.OutputPath)
	if err != nil {
		fail(&APIError{500, "storage_failed"})
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.Size() != result.OutputBytes {
		fail(&APIError{500, "output_validation"})
		return
	}
	h := w.Header()
	h.Set("Content-Type", "application/pdf")
	h.Set("Content-Disposition", `attachment; filename="compressed.pdf"`)
	h.Set("Cache-Control", "no-store")
	h.Set("Content-Length", strconv.FormatInt(result.OutputBytes, 10))
	h.Set("X-Compression-Status", string(result.Status))
	h.Set("X-Optimization-Engine", string(result.Engine))
	h.Set("X-Original-Bytes", strconv.FormatInt(result.OriginalBytes, 10))
	h.Set("X-Output-Bytes", strconv.FormatInt(result.OutputBytes, 10))
	h.Set("X-Saved-Percent", fmt.Sprintf("%.2f", result.SavedPercent))
	h.Set("X-Input-Image-Objects", strconv.Itoa(result.InputImageObjects))
	h.Set("X-Output-Image-Objects", strconv.Itoa(result.OutputImageObjects))
	warnings := strings.Join(compression.DeduplicateWarnings(result.Warnings), ",")
	if len(warnings) > 1024 {
		warnings = warnings[:1024]
	}
	h.Set("X-Compression-Warnings", warnings)
	if _, err := io.Copy(w, f); err != nil {
		status = 499
		code = "response_interrupted"
	}
}
