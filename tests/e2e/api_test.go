package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

type apiProcess struct {
	url, jobs string
	cmd       *exec.Cmd
	done      chan error
}

func startAPI(t *testing.T, extra ...string) *apiProcess {
	t.Helper()
	bin := os.Getenv("PDF_SERVER_BINARY")
	if bin == "" {
		t.Skip("set PDF_SERVER_BINARY inside runtime test container")
	}
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	jobs := t.TempDir()
	logfile, e := os.Create(filepath.Join(t.TempDir(), "server.log"))
	if e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(), "LISTEN_ADDR="+addr, "TEMP_DIR="+jobs, "WORKER_PATH=/usr/local/bin/pdfworker", "ALLOW_SPECIAL_TYPES=false")
	cmd.Env = append(cmd.Env, extra...)
	cmd.Stdout = logfile
	cmd.Stderr = logfile
	if e := cmd.Start(); e != nil {
		t.Fatal(e)
	}
	p := &apiProcess{"http://" + addr, jobs, cmd, make(chan error, 1)}
	go func() { p.done <- cmd.Wait() }()
	t.Cleanup(func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		select {
		case err := <-p.done:
			if err != nil {
				t.Error("server exit", err)
			}
		case <-time.After(8 * time.Second):
			_ = cmd.Process.Kill()
			t.Error("shutdown stuck")
		}
		_ = logfile.Close()
		data, _ := os.ReadFile(logfile.Name())
		t.Log("server log:", string(data))
	})
	wait(t, func() bool {
		r, e := http.Get(p.url + "/readyz")
		if e != nil {
			return false
		}
		defer r.Body.Close()
		return r.StatusCode == 200
	})
	return p
}
func wait(t *testing.T, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition timed out")
}
func jobsCount(p *apiProcess) int {
	e, _ := os.ReadDir(p.jobs)
	n := 0
	for _, v := range e {
		if strings.HasPrefix(v.Name(), "pdfjob-") {
			n++
		}
	}
	return n
}
func post(t *testing.T, p *apiProcess, data []byte, profile string) *http.Response {
	t.Helper()
	req := upload(t, p, data, profile)
	client := http.Client{Timeout: 10 * time.Second}
	r, e := client.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func upload(t *testing.T, p *apiProcess, data []byte, profile string) *http.Request {
	t.Helper()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	if profile != "" {
		_ = w.WriteField("profile", profile)
	}
	part, e := w.CreateFormFile("file", "ignored.pdf")
	if e != nil {
		t.Fatal(e)
	}
	_, _ = part.Write(data)
	_ = w.Close()
	r, e := http.NewRequest("POST", p.url+"/v1/compress", &b)
	if e != nil {
		t.Fatal(e)
	}
	r.Header.Set("Content-Type", w.FormDataContentType())
	return r
}
func minimalPDF(cmyk bool) []byte {
	objects := []string{"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [3 0 R] /Count 1 >>", "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] /Resources << >> >>"}
	if cmyk {
		objects[2] = "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] /Resources << /XObject << /Im0 4 0 R >> >> /Contents 5 0 R >>"
		objects = append(objects, "<< /Type /XObject /Subtype /Image /Width 1 /Height 1 /BitsPerComponent 8 /ColorSpace /DeviceCMYK /Length 4 >>\nstream\n\x00\x00\x00\x00\nendstream", "<< /Length 8 >>\nstream\n/Im0 Do\n\nendstream")
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.7\n")
	offsets := []int{0}
	for i, obj := range objects {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, off := range offsets[1:] {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	return b.Bytes()
}
func TestRealRuntimeAPI(t *testing.T) {
	p := startAPI(t)
	for _, tt := range []struct {
		name, profile   string
		data            []byte
		warning, status string
	}{
		{"unchanged", "lossless", minimalPDF(false), "", "unchanged"},
		{"complex fallback", "balanced", minimalPDF(true), "complex_color_preserved", "unchanged"},
		{"balanced", "balanced", nil, "", "compressed"},
		{"compact", "compact", nil, "", "compressed"},
		{"lossless", "lossless", nil, "", "compressed"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.data
			if input == nil {
				var e error
				input, e = os.ReadFile("../../testdata/pdf/flate-rgb-600dpi.pdf")
				if e != nil {
					t.Fatal(e)
				}
			}
			r := post(t, p, input, tt.profile)
			output, e := io.ReadAll(r.Body)
			_ = r.Body.Close()
			if e != nil {
				t.Fatal(e)
			}
			if r.StatusCode != 200 {
				t.Fatalf("%d: %s", r.StatusCode, output)
			}
			if len(output) > len(input) || r.Header.Get("X-Compression-Status") != tt.status {
				t.Fatal("size/status", r.Header)
			}
			if tt.status == "unchanged" && !bytes.Equal(input, output) {
				t.Fatal("unchanged bytes differ")
			}
			if tt.warning != "" && !strings.Contains(r.Header.Get("X-Compression-Warnings"), tt.warning) {
				t.Fatal("missing warning", r.Header)
			}
			for _, h := range []string{"X-Request-ID", "X-Optimization-Engine", "X-Original-Bytes", "X-Output-Bytes", "X-Saved-Percent", "X-Input-Image-Objects", "X-Output-Image-Objects"} {
				if r.Header.Get(h) == "" {
					t.Fatal("missing", h)
				}
			}
			path := filepath.Join(t.TempDir(), "output.pdf")
			_ = os.WriteFile(path, output, 0600)
			cmd := exec.Command("qpdf", "--check", path)
			if data, e := cmd.CombinedOutput(); e != nil {
				t.Fatal(e, string(data))
			}
			t.Log("headers", r.Header, "bytes", len(input), len(output))
			wait(t, func() bool { return jobsCount(p) == 0 })
		})
	}
	for _, tt := range []struct {
		data    []byte
		profile string
		status  int
	}{{[]byte("not pdf"), "lossless", 415}, {[]byte("%PDF-invalid"), "lossless", 422}, {minimalPDF(false), "unknown", 400}} {
		r := post(t, p, tt.data, tt.profile)
		_ = r.Body.Close()
		if r.StatusCode != tt.status {
			t.Fatal(r.StatusCode)
		}
		wait(t, func() bool { return jobsCount(p) == 0 })
	}
}
func TestRuntimeBusyBeforeUploadAndDisconnect(t *testing.T) {
	p := startAPI(t)
	addr := strings.TrimPrefix(p.url, "http://")
	var conns []net.Conn
	for i := 0; i < 2; i++ {
		conn, e := net.Dial("tcp", addr)
		if e != nil {
			t.Fatal(e)
		}
		conns = append(conns, conn)
		defer conn.Close()
		_, e = fmt.Fprintf(conn, "POST /v1/compress HTTP/1.1\r\nHost: localhost\r\nContent-Type: multipart/form-data; boundary=slow\r\nContent-Length: 100000\r\n\r\n")
		if e != nil {
			t.Fatal(e)
		}
	}
	wait(t, func() bool { return jobsCount(p) == 2 })
	r := post(t, p, minimalPDF(false), "lossless")
	_ = r.Body.Close()
	if r.StatusCode != 503 || r.Header.Get("Retry-After") != "1" {
		t.Fatal(r.StatusCode, r.Header)
	}
	for _, conn := range conns {
		_ = conn.Close()
	}
	wait(t, func() bool { return jobsCount(p) == 0 })
}
func TestRuntimeTimeoutAndLossyDisabled(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		p := startAPI(t, "PROCESSING_TIMEOUT=1ns")
		r := post(t, p, minimalPDF(false), "lossless")
		_ = r.Body.Close()
		if r.StatusCode != 504 {
			t.Fatal(r.StatusCode)
		}
		wait(t, func() bool { return jobsCount(p) == 0 })
	})
	t.Run("qpdf only", func(t *testing.T) {
		p := startAPI(t, "ENABLE_LOSSY=false", "GS_PATH=/missing-gs")
		r := post(t, p, minimalPDF(false), "")
		_ = r.Body.Close()
		if r.StatusCode != 400 {
			t.Fatal(r.StatusCode)
		}
		r = post(t, p, minimalPDF(false), "lossless")
		_ = r.Body.Close()
		if r.StatusCode != 200 {
			t.Fatal(r.StatusCode)
		}
	})
}
func TestRuntimeProcessingDisconnect(t *testing.T) {
	for _, tool := range []string{"qpdf", "gs"} {
		t.Run(tool, func(t *testing.T) { runtimeProcessingDisconnect(t, tool) })
	}
}
func runtimeProcessingDisconnect(t *testing.T, toolName string) {
	p := startAPI(t)
	data, e := os.ReadFile("../../testdata/pdf/flate-rgb-600dpi.pdf")
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := upload(t, p, data, "compact").WithContext(ctx)
	done := make(chan error, 1)
	go func() {
		r, e := http.DefaultClient.Do(req)
		if r != nil {
			_ = r.Body.Close()
		}
		done <- e
	}()
	var pids []int
	wait(t, func() bool {
		for _, v := range processChildren(p.cmd.Process.Pid) {
			pid, _ := strconv.Atoi(v)
			comm, _ := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
			if strings.TrimSpace(string(comm)) != "pdfworker" {
				continue
			}
			for _, child := range processChildren(pid) {
				childPID, _ := strconv.Atoi(child)
				comm, _ := os.ReadFile(fmt.Sprintf("/proc/%d/comm", childPID))
				if strings.TrimSpace(string(comm)) != toolName {
					continue
				}
				stat, e := os.ReadFile(fmt.Sprintf("/proc/%d/stat", childPID))
				if e != nil {
					continue
				}
				pos := strings.LastIndex(string(stat), ")")
				if pos < 0 {
					continue
				}
				fields := strings.Fields(string(stat)[pos+1:])
				if len(fields) < 3 {
					continue
				}
				if fields[2] != strconv.Itoa(pid) {
					t.Fatalf("tool %d group %s differs from worker %d", childPID, fields[2], pid)
				}
				pids = []int{pid, childPID}
				t.Logf("real worker %d, tool %d, shared PGID %s", pid, childPID, fields[2])
				return true
			}
		}
		return false
	})
	cancel()
	if e := <-done; e == nil {
		t.Fatal("disconnect not canceled")
	}
	wait(t, func() bool { return jobsCount(p) == 0 })
	for _, pid := range pids {
		wait(t, func() bool { return syscall.Kill(pid, 0) != nil })
	}
	t.Log("canceled/reaped worker IDs", pids)
}

func TestRuntimeShutdownInterruptsUpload(t *testing.T) {
	p := startAPI(t)
	conn, e := net.Dial("tcp", strings.TrimPrefix(p.url, "http://"))
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	_, e = fmt.Fprintf(conn, "POST /v1/compress HTTP/1.1\r\nHost: localhost\r\nContent-Type: multipart/form-data; boundary=slow\r\nContent-Length: 100000\r\n\r\n")
	if e != nil {
		t.Fatal(e)
	}
	wait(t, func() bool { return jobsCount(p) == 1 })
	_ = p.cmd.Process.Signal(syscall.SIGTERM)
	wait(t, func() bool { return jobsCount(p) == 0 && syscall.Kill(p.cmd.Process.Pid, 0) != nil })
}

// Linux attaches children to the thread that called exec, which need not be Go's leader thread.
func processChildren(pid int) []string {
	paths, _ := filepath.Glob(fmt.Sprintf("/proc/%d/task/*/children", pid))
	var children []string
	for _, path := range paths {
		b, _ := os.ReadFile(path)
		children = append(children, strings.Fields(string(b))...)
	}
	return children
}

func TestRuntimeQualityFixtureMatrix(t *testing.T) {
	p := startAPI(t)
	data, e := os.ReadFile("../../testdata/pdf/quality-manifest.json")
	if e != nil {
		t.Fatal(e)
	}
	var fixtures []struct {
		File       string `json:"file"`
		HTTPStatus int    `json:"http_status"`
		Warning    string `json:"warning"`
	}
	if e := json.Unmarshal(data, &fixtures); e != nil {
		t.Fatal(e)
	}
	if len(fixtures) < 25 {
		t.Fatal("incomplete matrix")
	}
	for _, fixture := range fixtures {
		for _, profile := range []string{"lossless", "balanced", "compact"} {
			t.Run(fixture.File+"/"+profile, func(t *testing.T) {
				input, e := os.ReadFile("../../testdata/pdf/" + fixture.File)
				if e != nil {
					t.Fatal(e)
				}
				r := post(t, p, input, profile)
				output, e := io.ReadAll(r.Body)
				_ = r.Body.Close()
				if e != nil {
					t.Fatal(e)
				}
				expected := 200
				if fixture.HTTPStatus != 0 {
					expected = fixture.HTTPStatus
				}
				if r.StatusCode != expected {
					t.Fatalf("status %d: %s", r.StatusCode, output)
				}
				if expected == 200 {
					if len(output) > len(input) {
						t.Fatal("output grew")
					}
					if r.Header.Get("X-Compression-Status") == "unchanged" && !bytes.Equal(input, output) {
						t.Fatal("unchanged bytes differ")
					}
					if fixture.Warning != "" && profile != "lossless" && (!strings.Contains(r.Header.Get("X-Compression-Warnings"), fixture.Warning) || r.Header.Get("X-Optimization-Engine") == "ghostscript") {
						t.Fatal("false fallback", r.Header)
					}
					path := filepath.Join(t.TempDir(), "output.pdf")
					if e := os.WriteFile(path, output, 0600); e != nil {
						t.Fatal(e)
					}
					if data, e := exec.Command("qpdf", "--check", path).CombinedOutput(); e != nil {
						t.Fatal(e, string(data))
					}
				}
				wait(t, func() bool { return jobsCount(p) == 0 })
			})
		}
	}
}

// Heavy active processing cancellation, in addition to the existing slow-upload case.
func TestRuntimeLoadBusyAndCancel(t *testing.T) {
	dir := os.Getenv("PDF_LOAD_FIXTURES")
	if dir == "" {
		t.Skip("set PDF_LOAD_FIXTURES for generated Stage 07 max-load fixture")
	}
	data, e := os.ReadFile(filepath.Join(dir, "pages-500-50mib.pdf"))
	if e != nil {
		t.Fatal(e)
	}
	if len(data) != 50<<20 {
		t.Fatal("not the exact byte boundary")
	}
	for _, workers := range []int{1, 2} {
		t.Run(strconv.Itoa(workers)+" workers", func(t *testing.T) {
			p := startAPI(t, "MAX_WORKERS="+strconv.Itoa(workers))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, workers)
			for i := 0; i < workers; i++ {
				req := upload(t, p, data, "balanced").WithContext(ctx)
				go func() {
					r, e := http.DefaultClient.Do(req)
					if r != nil {
						_ = r.Body.Close()
					}
					done <- e
				}()
			}
			wait(t, func() bool { return jobsCount(p) == workers })
			r := post(t, p, minimalPDF(false), "lossless")
			_ = r.Body.Close()
			if r.StatusCode != 503 || r.Header.Get("Retry-After") != "1" {
				t.Fatal("overload", r.StatusCode, r.Header)
			}
			var observed []int
			wait(t, func() bool {
				observed = nil
				for _, v := range processChildren(p.cmd.Process.Pid) {
					pid, _ := strconv.Atoi(v)
					comm, _ := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
					if strings.TrimSpace(string(comm)) != "pdfworker" {
						continue
					}
					found := false
					for _, child := range processChildren(pid) {
						cid, _ := strconv.Atoi(child)
						comm, _ := os.ReadFile(fmt.Sprintf("/proc/%d/comm", cid))
						if strings.TrimSpace(string(comm)) == "gs" {
							observed = append(observed, cid)
							found = true
						}
					}
					if found {
						observed = append(observed, pid)
					}
				}
				return len(observed) >= 2*workers
			})
			cancel()
			for i := 0; i < workers; i++ {
				if e := <-done; e == nil {
					t.Fatal("request finished before cancel")
				}
			}
			wait(t, func() bool { return jobsCount(p) == 0 })
			for _, pid := range observed {
				wait(t, func() bool { return syscall.Kill(pid, 0) != nil })
			}
			t.Log("max-load canceled/reaped", observed, "workers", workers)
		})
	}
}
