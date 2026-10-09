package storage

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var ErrJobLimit = errors.New("job storage limit exceeded")

const prefix = "pdfjob-"
const marker = ".pdf-service-owned"

type Store struct{ Root string }
type Job struct{ Dir string }

// Root must be dedicated to this service. Only marked, prefixed directories are removed.
func (s Store) Startup() error {
	if err := os.MkdirAll(s.Root, 0700); err != nil {
		return err
	}
	entries, err := os.ReadDir(s.Root)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), prefix) {
			continue
		}
		dir := filepath.Join(s.Root, e.Name())
		if info, err := os.Lstat(filepath.Join(dir, marker)); err == nil && info.Mode().IsRegular() {
			if err := os.RemoveAll(dir); err != nil {
				return err
			}
		}
	}
	return s.Writable()
}
func (s Store) Writable() error {
	f, err := os.CreateTemp(s.Root, ".write-check-")
	if err != nil {
		return err
	}
	name := f.Name()
	closeErr := f.Close()
	removeErr := os.Remove(name)
	return errors.Join(closeErr, removeErr)
}
func (s Store) NewJob() (*Job, error) {
	dir, err := os.MkdirTemp(s.Root, prefix)
	if err != nil {
		return nil, err
	}
	if err = os.WriteFile(filepath.Join(dir, marker), nil, 0600); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	return &Job{Dir: dir}, nil
}
func (j *Job) Cleanup() error { return os.RemoveAll(j.Dir) }
func (j *Job) CheckSize(limit int64) error {
	var total int64
	return filepath.WalkDir(j.Dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) && path != j.Dir {
				return nil
			}
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return errors.New("symlink in job")
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		total += info.Size()
		if total > limit {
			return ErrJobLimit
		}
		return nil
	})
}
