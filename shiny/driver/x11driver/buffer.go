// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x11driver

import (
	"image"
	"log"
	"sync"
	"unsafe"

	"github.com/BurntSushi/xgb/shm"
	"github.com/BurntSushi/xgb/xproto"

	"golang.org/x/exp/shiny/screen"
)

type bufferImpl struct {
	s *screenImpl

	addr unsafe.Pointer
	buf  []byte
	rgba image.RGBA
	size image.Point
	xs   shm.Seg

	mu        sync.Mutex
	nUpload   uint32
	released  bool
	cleanedUp bool
}

func (b *bufferImpl) Size() image.Point { return b.size }
func (b *bufferImpl) RGBA() *image.RGBA { return &b.rgba }

func (b *bufferImpl) preUpload() {
	b.mu.Lock()
	if b.released {
		b.mu.Unlock()
		panic("x11driver: Buffer.Upload called after Buffer.Release")
	}
	needsSwizzle := b.nUpload == 0
	b.nUpload++
	b.mu.Unlock()

	if needsSwizzle {
		swizzle(b.buf)
	}
}

// swizzle converts a pixel buffer between Go's RGBA byte order and X11's BGRA
// byte order.
//
// TODO: optimize this.
func swizzle(p []byte) {
	if len(p)%4 != 0 {
		return
	}
	for i := 0; i < len(p); i += 4 {
		p[i+0], p[i+2] = p[i+2], p[i+0]
	}
}

func (b *bufferImpl) postUpload() {
	b.mu.Lock()
	b.nUpload--
	more := b.nUpload != 0
	released := b.released
	b.mu.Unlock()

	if more {
		return
	}
	if released {
		b.cleanUp()
	} else {
		swizzle(b.buf)
	}
}

func (b *bufferImpl) Release() {
	b.mu.Lock()
	cleanUp := !b.released && b.nUpload == 0
	b.released = true
	b.mu.Unlock()

	if cleanUp {
		b.cleanUp()
	}
}

func (b *bufferImpl) cleanUp() {
	b.mu.Lock()
	alreadyCleanedUp := b.cleanedUp
	b.cleanedUp = true
	b.mu.Unlock()

	if alreadyCleanedUp {
		panic("x11driver: Buffer clean-up occurred twice")
	}

	b.s.mu.Lock()
	delete(b.s.buffers, b.xs)
	b.s.mu.Unlock()

	shm.Detach(b.s.xc, b.xs)
	if err := shmClose(b.addr); err != nil {
		log.Printf("x11driver: shmClose: %v", err)
	}
}

func upload(s *screenImpl, u screen.Uploader, xd xproto.Drawable, xg xproto.Gcontext, depth uint8,
	dp image.Point, src *bufferImpl, sr image.Rectangle, sender screen.Sender) {

	src.preUpload()

	// TODO: adjust if dp is outside dst bounds, or sr is outside src bounds.
	dr := sr.Sub(sr.Min).Add(dp)

	cookie := shm.PutImage(
		s.xc, xd, xg,
		uint16(src.size.X), uint16(src.size.Y), // TotalWidth, TotalHeight,
		uint16(sr.Min.X), uint16(sr.Min.Y), // SrcX, SrcY,
		uint16(dr.Dx()), uint16(dr.Dy()), // SrcWidth, SrcHeight,
		int16(dr.Min.X), int16(dr.Min.Y), // DstX, DstY,
		depth, xproto.ImageFormatZPixmap,
		1, src.xs, 0, // 1 means send a completion event, 0 means a zero offset.
	)

	s.mu.Lock()
	s.uploads[cookie.Sequence] = completion{
		sender: sender,
		event: screen.UploadedEvent{
			Buffer:   src,
			Uploader: u,
		},
	}
	s.mu.Unlock()
}
