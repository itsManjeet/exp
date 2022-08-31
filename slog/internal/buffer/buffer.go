// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package buffer provides a pool-allocated byte buffer.
package buffer

import (
	"sync"
)

// buffer adapted from go/src/fmt/print.go
type Buffer []byte

// Having an initial size gives a dramatic speedup.
var bufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 1024)
		return (*Buffer)(&b)
	},
}

func New() *Buffer {
	return bufPool.Get().(*Buffer)
}

const maxBufferSize = 16 << 10

func (b *Buffer) Free() {
	if cap(*b) <= maxBufferSize {
		*b = (*b)[:0]
		bufPool.Put(b)
	}
}
func (b *Buffer) Write(p []byte) {
	*b = append(*b, p...)
}

func (b *Buffer) WriteString(s string) {
	*b = append(*b, s...)
}

func (b *Buffer) WriteByte(c byte) {
	*b = append(*b, c)
}
