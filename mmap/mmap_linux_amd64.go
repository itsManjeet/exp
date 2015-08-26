// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package mmap provides a way to memory-map a file.
package mmap

// TODO: see if this works for linux_*, not just linux_amd64.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

// debug is whether to print debugging messages for manual testing.
//
// The runtime.SetFinalizer documentation says that, "The finalizer for x is
// scheduled to run at some arbitrary time after x becomes unreachable. There
// is no guarantee that finalizers will run before a program exits", so we
// cannot automatically test that the finalizer runs. Instead, set this to true
// when running the manual test.
const debug = false

// Reader reads a memory-mapped file.
type Reader struct {
	addr uintptr
	b    []byte
	size int64

	mu     sync.Mutex
	closed bool
}

// Close closes the reader.
func (r *Reader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}
	r.closed = true
	runtime.SetFinalizer(r, nil)

	_, _, errno := syscall.Syscall(syscall.SYS_MUNMAP, r.addr, uintptr(r.size), 0)
	if debug {
		println("munmap", r.addr, "errno=", errno)
	}
	if errno != 0 {
		return errno
	}
	return nil
}

// Size returns the size of the underlying memory-mapped file.
func (r *Reader) Size() int64 {
	return r.size
}

// ReadAt implements the io.ReaderAt interface.
func (r *Reader) ReadAt(p []byte, off int64) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return 0, errors.New("mmap: closed")
	}
	if off < 0 || r.size < off {
		return 0, fmt.Errorf("mmap: invalid ReadAt offset %d", off)
	}
	n := copy(p, r.b[int(off):])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// Open memory-maps the named file for reading.
func Open(filename string) (*Reader, error) {
	const maxSize = 2 << 30 // 2 GiB sanity check.

	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := fi.Size()
	if size < 0 || maxSize < size {
		return nil, fmt.Errorf("mmap: file %q is too large", filename)
	}
	addr, _, errno := syscall.Syscall6(
		syscall.SYS_MMAP, 0, uintptr(size), syscall.PROT_READ, syscall.MAP_SHARED, f.Fd(), 0)
	if debug {
		println("mmap", addr, "errno=", errno)
	}
	if errno != 0 {
		return nil, errno
	}
	a := (*[maxSize]byte)(unsafe.Pointer(addr))
	r := &Reader{
		addr: addr,
		b:    (*a)[:size:size],
		size: size,
	}
	runtime.SetFinalizer(r, (*Reader).Close)
	return r, nil
}
