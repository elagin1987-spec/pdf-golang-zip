package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateJobsCleanupAndLimits(t *testing.T) {
	s := Store{Root: t.TempDir()}
	if err := s.Startup(); err != nil {
		t.Fatal(err)
	}
	job, err := s.NewJob()
	if err != nil {
		t.Fatal(err)
	}
	defer job.Cleanup()
	info, _ := os.Stat(job.Dir)
	if info.Mode().Perm() != 0700 {
		t.Fatal(info.Mode())
	}
	path := filepath.Join(job.Dir, "input.pdf")
	if err := os.WriteFile(path, []byte("12345"), 0600); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(job.CheckSize(4), ErrJobLimit) {
		t.Fatal("limit not applied")
	}
	if err := job.CheckSize(5); err != nil {
		t.Fatal(err)
	}
	unrelated := filepath.Join(s.Root, "pdfjob-unrelated")
	_ = os.Mkdir(unrelated, 0700)
	_ = os.WriteFile(filepath.Join(unrelated, "keep"), nil, 0600)
	if err := s.Startup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(job.Dir); !os.IsNotExist(err) {
		t.Fatal("owned job survived")
	}
	if _, err := os.Stat(unrelated); err != nil {
		t.Fatal("unrelated directory removed")
	}
}

func TestLostJobIsStorageFailure(t *testing.T) {
	j, err := (Store{Root: t.TempDir()}).NewJob()
	if err != nil {
		t.Fatal(err)
	}
	if err := j.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if err := j.CheckSize(100); !os.IsNotExist(err) {
		t.Fatal("lost job was ignored", err)
	}
}
