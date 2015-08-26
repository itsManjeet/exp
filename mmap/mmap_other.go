// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !linux

// Package mmap provides a way to memory-map a file.
package mmap

import (
	"fmt"
	"os"
)

// Reader reads a memory-mapped file.
//
// Like any io.ReaderAt, clients can execute parallel ReadAt calls, but it is
// not safe to call Close and ReadAt concurrently.
type Reader struct {
	f   *os.File
	len int
}

// Close closes the reader.
func (r *Reader) Close() error {
	return r.f.Close()
}

// Len returns the length of the underlying memory-mapped file.
func (r *Reader) Len() int {
	return r.len
}

// At returns the byte at index i.
func (r *Reader) At(i int) byte {
	if i < 0 || r.len <= i {
		panic("index out of range")
	}
	var b [1]byte
	r.ReadAt(b[:], int64(i))
	return b[0]
}

// ReadAt implements the io.ReaderAt interface.
func (r *Reader) ReadAt(p []byte, off int64) (int, error) {
	return r.f.ReadAt(p, off)
}

// Open memory-maps the named file for reading.
func Open(filename string) (*Reader, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}

	size := fi.Size()
	if size < 0 {
		f.Close()
		return nil, fmt.Errorf("mmap: file %q has negative size", filename)
	}
	if size != int64(int(size)) {
		f.Close()
		return nil, fmt.Errorf("mmap: file %q is too large", filename)
	}

	return &Reader{
		f:   f,
		len: int(fi.Size()),
	}, nil
}
