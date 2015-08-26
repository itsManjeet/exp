// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !linux !amd64

// Package mmap provides a way to memory-map a file.
package mmap

import (
	"os"
)

// Reader reads a memory-mapped file.
type Reader struct {
	f    *os.File
	size int64
}

// Close closes the reader.
func (r *Reader) Close() error {
	return r.f.Close()
}

// Size returns the size of the underlying memory-mapped file.
func (r *Reader) Size() int64 {
	return r.size
}

// ReadAt implements the io.ReaderAt interface.
//
// It is safe to call ReadAt multiple times concurrently.
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
	return &Reader{
		f:    f,
		size: fi.Size(),
	}, nil
}
