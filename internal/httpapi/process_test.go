package httpapi

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"pdf-golang-zip/internal/compression"
	"pdf-golang-zip/internal/config"
	"pdf-golang-zip/internal/storage"
)

func script(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0700); err != nil {
		t.Fatal(err)
	}
	return path
}
func TestReadinessPinnedToolsAndLossless(t *testing.T) {
	c := config.Default()
	c.TempDir = t.TempDir()
	c.WorkerPath = script(t, "worker", "exit 0\n")
	c.QPDFPath = script(t, "qpdf", "echo 'qpdf version 12.4.2'\n")
	c.GSPath = script(t, "gs", "echo 10.08.0\n")
	s := New(c, storage.Store{Root: c.TempDir}, nil, nil)
	defer s.Stop()
	if err := s.checkReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	s.Config.GSPath = "/does-not-exist"
	if s.checkReady(context.Background()) == nil {
		t.Fatal("missing gs accepted")
	}
	s.Config.EnableLossy = false
	if err := s.checkReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	s.Config.QPDFPath = script(t, "wrong-version", "echo 'qpdf version 12.4.20'\n")
	if s.checkReady(context.Background()) == nil {
		t.Fatal("wrong qpdf accepted")
	}
}
func TestProcessWorkerStorageWatchAndTimeout(t *testing.T) {
	for _, mode := range []string{"timeout", "storage"} {
		t.Run(mode, func(t *testing.T) {
			c := config.Default()
			c.TempDir = t.TempDir()
			c.MaxJobBytes = 50
			// Tool stubs are confined to the test job and intentionally ignore TERM.
			body := `cat >/dev/null
sh -c 'trap "" TERM; echo $$ > child.pid; exec sleep 30' &
if [ "$TEST_MODE" = storage ]; then head -c 100 /dev/zero > oversized; fi
wait
`
			c.WorkerPath = script(t, "worker", body)
			t.Setenv("TEST_MODE", mode)
			j, err := (storage.Store{Root: c.TempDir}).NewJob()
			if err != nil {
				t.Fatal(err)
			}
			defer j.Cleanup()
			_ = os.WriteFile(filepath.Join(j.Dir, "input.pdf"), []byte("%PDF-1.7"), 0600)
			ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
			defer cancel()
			_, err = (ProcessWorker{Config: c}).Process(ctx, j, compression.ProfileLossless)
			if mode == "timeout" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal(err)
			}
			if mode == "storage" {
				var api *APIError
				if !errors.As(err, &api) || api.Code != "job_storage_limit" {
					t.Fatal(err)
				}
			}
			b, e := os.ReadFile(filepath.Join(j.Dir, "child.pid"))
			if e != nil {
				t.Fatal(e)
			}
			pid, e := strconv.Atoi(strings.TrimSpace(string(b)))
			if e != nil {
				t.Fatal(e)
			}
			// init in the test container reaps orphans; allow its small scheduling delay.
			deadline := time.Now().Add(time.Second)
			for syscall.Kill(pid, 0) == nil && time.Now().Before(deadline) {
				time.Sleep(10 * time.Millisecond)
			}
			if syscall.Kill(pid, 0) == nil {
				t.Fatalf("descendant %d survived/reaping missing", pid)
			}
		})
	}
}
