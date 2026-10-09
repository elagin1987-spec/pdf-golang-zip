package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"pdf-golang-zip/internal/compression"
	"pdf-golang-zip/internal/config"
	"pdf-golang-zip/internal/storage"
)

type processFunc func(context.Context, *storage.Job, compression.Profile) (*compression.Result, error)

func (f processFunc) Process(ctx context.Context, j *storage.Job, p compression.Profile) (*compression.Result, error) {
	return f(ctx, j, p)
}
func setup(t *testing.T, proc Processor) (*Server, *bytes.Buffer) {
	t.Helper()
	c := config.Default()
	c.TempDir = t.TempDir()
	var logs bytes.Buffer
	s := New(c, storage.Store{Root: c.TempDir}, proc, slog.New(slog.NewJSONHandler(&logs, nil)))
	s.Ready = func(context.Context) error { return nil }
	t.Cleanup(s.Stop)
	return s, &logs
}

type field struct {
	name, value string
	file        bool
}

func request(t *testing.T, fields ...field) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, f := range fields {
		var w io.Writer
		var err error
		if f.file {
			w, err = writer.CreateFormFile(f.name, "../../private-name.xyz")
		} else {
			w, err = writer.CreateFormField(f.name)
		}
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.WriteString(w, f.value)
	}
	_ = writer.Close()
	r := httptest.NewRequest("POST", "/v1/compress", &body)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	return r
}
func original(ctx context.Context, j *storage.Job, p compression.Profile) (*compression.Result, error) {
	path := filepath.Join(j.Dir, "input.pdf")
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return &compression.Result{OutputPath: path, Status: compression.StatusUnchanged, Engine: compression.EngineOriginal, OriginalBytes: fi.Size(), OutputBytes: fi.Size(), InputImageObjects: 2, OutputImageObjects: 2, Warnings: []string{"complex_color_preserved", "complex_color_preserved"}}, nil
}
func noJobs(t *testing.T, s *Server) {
	t.Helper()
	entries, e := os.ReadDir(s.Config.TempDir)
	if e != nil || len(entries) != 0 {
		t.Fatalf("leaked jobs: %v %v", entries, e)
	}
}
func TestMultipartContract(t *testing.T) {
	pdf := "prefix %PDF-1.7\nprivate PDF content"
	tests := []struct {
		name   string
		fields []field
		status int
		code   string
	}{
		{"file first", []field{{"file", pdf, true}, {"profile", "lossless", false}}, 200, ""},
		{"profile first", []field{{"profile", "compact", false}, {"file", pdf, true}}, 200, ""},
		{"default", []field{{"file", pdf, true}}, 200, ""},
		{"duplicate file", []field{{"file", pdf, true}, {"file", pdf, true}}, 400, "invalid_fields"},
		{"duplicate profile", []field{{"profile", "balanced", false}, {"profile", "compact", false}, {"file", pdf, true}}, 400, "invalid_fields"},
		{"unknown field", []field{{"file", pdf, true}, {"flags", "unsafe", false}}, 400, "invalid_fields"},
		{"profile upload", []field{{"file", pdf, true}, {"profile", "compact", true}}, 400, "invalid_fields"},
		{"empty", []field{{"file", "", true}}, 400, "empty_file"},
		{"missing", []field{{"profile", "lossless", false}}, 400, "missing_file"},
		{"bad profile", []field{{"file", pdf, true}, {"profile", "COMPACT", false}}, 400, "invalid_profile"},
		{"oversized field", []field{{"file", pdf, true}, {"profile", strings.Repeat("x", 1025), false}}, 400, "field_too_large"},
		{"not pdf", []field{{"file", "not a pdf", true}}, 415, "not_pdf"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, logs := setup(t, processFunc(original))
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, request(t, tt.fields...))
			if w.Code != tt.status {
				t.Fatalf("status %d: %s", w.Code, w.Body)
			}
			if tt.status == 200 {
				if w.Body.String() != pdf || w.Header().Get("X-Compression-Status") != "unchanged" || w.Header().Get("X-Optimization-Engine") != "original" || w.Header().Get("X-Saved-Percent") != "0.00" || w.Header().Get("X-Compression-Warnings") != "complex_color_preserved" || w.Header().Get("X-Input-Image-Objects") != "2" || w.Header().Get("X-Output-Image-Objects") != "2" || w.Header().Get("Content-Type") != "application/pdf" || w.Header().Get("Content-Disposition") != `attachment; filename="compressed.pdf"` {
					t.Fatal("wrong PDF response", w.Header())
				}
			} else {
				var body map[string]string
				_ = json.Unmarshal(w.Body.Bytes(), &body)
				if body["code"] != tt.code || body["request_id"] != w.Header().Get("X-Request-ID") {
					t.Fatal(body)
				}
			}
			if logs.Len() == 0 || strings.Contains(logs.String(), "private PDF content") || strings.Contains(logs.String(), "private-name") {
				t.Fatal("unsafe logs", logs)
			}
			noJobs(t, s)
		})
	}
}
func TestLimitsAndDisabled(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*config.Config)
		status int
	}{
		{"file bytes", func(c *config.Config) { c.MaxFileBytes = 5 }, 413},
		{"request bytes", func(c *config.Config) { c.MaxRequestBytes = 40 }, 413},
		{"disabled default", func(c *config.Config) { c.EnableLossy = false }, 400},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := setup(t, processFunc(original))
			tt.change(&s.Config)
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, request(t, field{"file", "%PDF-1.7\nx", true}))
			if w.Code != tt.status {
				t.Fatal(w.Code, w.Body)
			}
			noJobs(t, s)
		})
	}
	s, _ := setup(t, processFunc(original))
	s.Config.EnableLossy = false
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, request(t, field{"profile", "lossless", false}, field{"file", "%PDF-1.7\nx", true}))
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
}

type unreadBody struct{ read bool }

func (b *unreadBody) Read(p []byte) (int, error) { b.read = true; return 0, io.EOF }
func (b *unreadBody) Close() error               { return nil }
func TestBusyBeforeUpload(t *testing.T) {
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	s, _ := setup(t, processFunc(func(ctx context.Context, j *storage.Job, p compression.Profile) (*compression.Result, error) {
		entered <- struct{}{}
		<-release
		return original(ctx, j, p)
	}))
	done := make(chan struct{}, 2)
	for i := 0; i < 2; i++ {
		go func() {
			s.Handler().ServeHTTP(httptest.NewRecorder(), request(t, field{"file", "%PDF-1.7\nx", true}))
			done <- struct{}{}
		}()
	}
	for i := 0; i < 2; i++ {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("not admitted")
		}
	}
	body := &unreadBody{}
	r := httptest.NewRequest("POST", "/v1/compress", body)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 503 || body.read || w.Header().Get("Retry-After") != "1" {
		t.Fatal("busy consumed body", w.Code, body.read)
	}
	close(release)
	<-done
	<-done
	noJobs(t, s)
}
func TestTimeoutCancelShutdownAndErrors(t *testing.T) {
	for _, mode := range []string{"timeout", "disconnect", "shutdown"} {
		t.Run(mode, func(t *testing.T) {
			entered := make(chan struct{})
			s, _ := setup(t, processFunc(func(ctx context.Context, j *storage.Job, p compression.Profile) (*compression.Result, error) {
				close(entered)
				<-ctx.Done()
				return nil, ctx.Err()
			}))
			s.Config.ProcessingTimeout = 30 * time.Millisecond
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			r := request(t, field{"file", "%PDF-1.7\nx", true}).WithContext(ctx)
			w := httptest.NewRecorder()
			done := make(chan struct{})
			go func() { s.Handler().ServeHTTP(w, r); close(done) }()
			<-entered
			if mode == "disconnect" {
				cancel()
			}
			if mode == "shutdown" {
				s.Stop()
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("not canceled")
			}
			if mode == "timeout" && w.Code != 504 {
				t.Fatal(w.Code, w.Body)
			}
			if w.Header().Get("Content-Type") == "application/pdf" {
				t.Fatal("premature binary")
			}
			noJobs(t, s)
		})
	}
	for _, status := range []int{422, 503, 500} {
		s, _ := setup(t, processFunc(func(context.Context, *storage.Job, compression.Profile) (*compression.Result, error) {
			return nil, &APIError{status, "safe_code"}
		}))
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, request(t, field{"file", "%PDF-1.7\nx", true}))
		if w.Code != status {
			t.Fatal(w.Code)
		}
		noJobs(t, s)
	}
	s, _ := setup(t, processFunc(original))
	s.Ready = func(context.Context) error { return errors.New("private stderr") }
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/readyz", nil))
	if w.Code != 503 || strings.Contains(w.Body.String(), "private stderr") {
		t.Fatal(w.Body)
	}
}
