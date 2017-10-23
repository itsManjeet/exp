// Package write provides a way to atomically create or replace a file.
//
// Caveat: this package requires the file system rename(2) implementation to be
// atomic. Notably, this is not the case when using NFS with multiple clients:
// https://stackoverflow.com/a/41396801
package write

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

func tempDir(dir, dest string) string {
	if dir != "" {
		return dir // caller-specified directory always wins
	}

	// Chose the destination directory as temporary directory so that we
	// definitely can rename the file, for which both temporary and destination
	// file need to point to the same mount point.
	fallback := filepath.Dir(dest)

	// TODO(stapelberg): memoize this rather expensive suitability test under
	// the assumption that TMPDIR is not changed between calls, and that the
	// file system which TMPDIR points to is not replaced with a file system
	// that is no longer suitable.

	// The user might have overridden the os.TempDir() return value by setting
	// the TMPDIR environment variable.
	tmpdir := os.TempDir()

	testsrc, err := ioutil.TempFile(tmpdir, "."+filepath.Base(dest))
	if err != nil {
		return fallback
	}
	cleanup := true
	defer func() {
		if cleanup {
			os.Remove(testsrc.Name())
		}
	}()
	testsrc.Close()

	testdest, err := ioutil.TempFile(filepath.Dir(dest), "."+filepath.Base(dest))
	if err != nil {
		return fallback
	}
	defer os.Remove(testdest.Name())
	testdest.Close()

	if err := os.Rename(testsrc.Name(), testdest.Name()); err != nil {
		return fallback
	}
	cleanup = false // testsrc no longer exists
	return tmpdir
}

// PendingFile is a pending temporary file, waiting to replace the destination
// path in a call to CloseAtomicallyReplace.
type PendingFile struct {
	*os.File

	path   string
	done   bool
	closed bool
}

// Cleanup is a no-op if CloseAtomicallyReplace succeeded, and otherwise closes
// and removes the temporary file.
func (t *PendingFile) Cleanup() error {
	if t.done {
		return nil
	}
	// An error occurred. Close and remove the tempfile. Errors are returned for
	// reporting, there is nothing the caller can recover here.
	var closeErr error
	if !t.closed {
		closeErr = t.Close()
	}
	if err := os.Remove(t.Name()); err != nil {
		return err
	}
	return closeErr
}

// CloseAtomicallyReplace closes the temporary file and atomatically replaces
// the destination file with it, i.e., a concurrent open(2) call will either
// open the file previously located at the destination path (if any), or the
// just written file, but the file will always be present.
func (t *PendingFile) CloseAtomicallyReplace() error {
	// Even on an ordered file system (e.g. ext4 with data=ordered) or file
	// systems with write barriers, we cannot skip the fsync(2) call as per
	// Theodore Ts'o (ext2/3/4 lead developer):
	//
	// > data=ordered only guarantees the avoidance of stale data (e.g., the previous
	// > contents of a data block showing up after a crash, where the previous data
	// > could be someone's love letters, medical records, etc.). Without the fsync(2)
	// > a zero-length file is a valid and possible outcome after the rename.
	if err := t.Sync(); err != nil {
		return err
	}
	t.closed = true
	if err := t.Close(); err != nil {
		return err
	}
	if err := os.Rename(t.Name(), t.path); err != nil {
		return err
	}
	t.done = true
	return nil
}

// TempFile wraps ioutil.TempFile for the use case of atomically creating or
// replacing the destination file at path. If dir is the empty string,
// os.TempDir() is considered, falling back to filepath.Dir(path).
//
// Example:
//     t, err := write.TempFile("/tmp/bar.txt")
//     if err != nil {
//     	return err
//     }
//     defer t.Cleanup()
//     if _, err := t.Write([]byte("foo")); err != nil {
//     	return err
//     }
//     if err := t.CloseAtomicallyReplace(); err != nil {
//     	return err
//     }
func TempFile(dir, path string) (*PendingFile, error) {
	f, err := ioutil.TempFile(tempDir(dir, path), "."+filepath.Base(path))
	if err != nil {
		return nil, err
	}

	return &PendingFile{File: f, path: path}, nil
}
