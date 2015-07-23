// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package shiny defines a basic Graphical User Interface programming model.
//
// This package is experimental, and its API is not guaranteed to be stable.
package shiny

import (
	"errors"
	"image"

	"golang.org/x/image/math/f64"
)

// TODO: say how to make a Window.

// TODO: specify image format (Alpha or Gray, not just RGBA) for NewBuffer
// and/or NewTexture?

// Window is a top-level GUI window.
type Window interface {
	// Close closes the window and its event channel.
	Close() error

	// Events returns the window's event channel, which carries key, mouse,
	// paint and other events.
	//
	// TODO: define and describe these events.
	Events() <-chan interface{}

	// Send sends an event on the window's event channel.
	Send(event interface{})

	// NewBuffer returns a new Buffer that is compatible with this Window.
	NewBuffer(size image.Point) (Buffer, error)

	// NewTexture returns a new Texture that is compatible with this Window.
	NewTexture(size image.Point) (Texture, error)

	// Upload uploads the sub-Buffer defined by src and sr to the Window, such
	// that sr.Min in src-space aligns with dp in the Window's space.
	//
	// The src Buffer is re-usable, but only after an UploadedEvent for that
	// Buffer is received on the event channel.
	//
	// There might not be any visible effect until EndPaint is called.
	Upload(dp image.Point, src Buffer, sr image.Rectangle)

	// Copy copies the sub-Texture defined by src and sr to the Window, such
	// that sr.Min in src-space aligns with dp in the Window's space.
	//
	// There might not be any visible effect until EndPaint is called.
	Copy(dp image.Point, src Texture, sr image.Rectangle, op Op, opts *DrawOptions)

	// Scale scales the sub-Texture defined by src and sr to the Window, such
	// that sr in src-space is mapped to dr in the Window's space.
	//
	// There might not be any visible effect until EndPaint is called.
	Scale(dr image.Rectangle, src Texture, sr image.Rectangle, op Op, opts *DrawOptions)

	// Transform transforms the sub-Texture defined by src and sr to the
	// Window, such that src2dst defines how to transform src coordinates to
	// Window coordinates.
	//
	// There might not be any visible effect until EndPaint is called.
	Transform(src2dst *f64.Aff3, src Texture, sr image.Rectangle, op Op, opts *DrawOptions)

	// EndPaint flushes any pending Upload, Copy, Scale and Transform calls to
	// the screen.
	EndPaint()
}

// Buffer is an in-memory pixel buffer. Its pixels can be modified by any Go
// code that takes an *image.RGBA, such as the standard library's image/draw
// package.
//
// To see a Buffer's contents on a screen, upload it to a Texture (and then
// draw the Texture on a Window) or upload it directly to a Window.
//
// When specifying a sub-Buffer via Upload, a Buffer's top-left pixel is always
// (0, 0) in its own coordinate space.
type Buffer interface {
	// Release releases any resources associated with the Buffer. The Buffer
	// should not be used after Release is called.
	Release()

	// Size returns the size of the Buffer's image.
	Size() image.Point

	// RGBA returns the pixel buffer as an *image.RGBA.
	RGBA() *image.RGBA
}

// Texture is a pixel buffer, but not one that is directly accessible as a
// []byte. Conceptually, it could live on a GPU, in another process or even be
// across a network, instead of on a CPU in this process.
//
// Buffers can be uploaded to Textures, and Textures can be drawn (whether
// copied, scaled or arbitrarily affine transformed) to Windows.
//
// When specifying a sub-Texture via Copy, Scale or Transform, a Texture's
// top-left pixel is always (0, 0) in its own coordinate space.
type Texture interface {
	// Release releases any resources associated with the Texture. The Texture
	// should not be used after Release is called.
	Release()

	// Size returns the size of the Texture's image.
	Size() image.Point

	// Upload uploads the sub-Buffer defined by src and sr to the Texture, such
	// that sr.Min in src-space aligns with dp in the Texture's space.
	//
	// The src Buffer is re-usable, but only after an UploadedEvent for that
	// Buffer is received on the event channel.
	Upload(dp image.Point, src Buffer, sr image.Rectangle)
}

// Op is a drawing operator.
type Op uint32

const (
	OpOver Op = iota
	OpSrc
)

// DrawOptions are optional arguments to draw methods.
type DrawOptions struct {
	// TODO: transparency in [0x0000, 0xffff]?
	// TODO: scaler (nearest neighbor vs linear)?
}

// UploadedEvent records that a Buffer was uploaded to either a Texture or a
// Window.
type UploadedEvent struct {
	Buf Buffer
	Tex Texture
	Win Window
}
