// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package font defines an interface for font faces, for drawing text on an
// image.
//
// Other packages provide font face implementations. For example, a truetype
// package would provide one based on .ttf font files.
package font

// TODO: move this from golang.org/x/exp to golang.org/x/image ??

import (
	"image"
	"image/draw"
	"io"

	"golang.org/x/image/math/fixed"
)

// TODO: who is responsible for caches (glyph images, glyph indices, kerns)?
// The Drawer or the Face?

// Face is a font face. Its glyphs are often derived from a font file, such as
// "Comic_Sans_MS.ttf", but a face has a specific size, style, weight and
// hinting. For example, the 12pt and 18pt versions of Comic Sans are two
// different faces, even if derived from the same font file.
//
// A Face is not safe for concurrent use by multiple goroutines, as its methods
// may re-use implementation-specific caches and mask image buffers.
//
// To create a Face, look to other packages that implement specific font file
// formats.
type Face interface {
	io.Closer

	// Glyph returns the draw.DrawMask parameters (dr, mask, maskp) to draw r's
	// glyph at the sub-pixel destination location dot. It also returns the new
	// dot after adding the glyph's advance width. It returns !ok if the face
	// does not contain a glyph for r.
	//
	// The contents of the mask image returned by one Glyph call may change
	// after the next Glyph call. Callers that want to cache the mask must make
	// a copy.
	Glyph(dot fixed.Point26_6, r rune) (
		newDot fixed.Point26_6, dr image.Rectangle, mask image.Image, maskp image.Point, ok bool)

	// Kern returns the horizontal adjustment for the kerning pair (r0, r1). A
	// positive kern means to move the glyphs further apart.
	Kern(r0, r1 rune) fixed.Int26_6

	// TODO: per-font and per-glyph Metrics.
	// TODO: ColoredGlyph for various emoji?
	// TODO: Ligatures? Shaping?
}

// TODO: let a MultiFace open and close its constituent faces lazily? Or can we
// assume that you can always mmap a font file, and let the OS handle paging?

// MultiFaceElement maps a single rune range [Lo, Hi] to a Face.
type MultiFaceElement struct {
	// Lo and Hi are the low and high rune bounds. Both are inclusive.
	Lo, Hi rune
	// Face is the Face to use for runes in the MultiFaceElement's range.
	Face Face

	// TODO: height/ascent adjustment, so that subfonts' baselines align. This
	// requires the Font interface to have a Metrics method.
	//
	// TODO: do any other font metrics need mentioning here?
}

// MultiFace maps multiple rune ranges to Faces. Rune ranges may overlap; the
// first match wins.
type MultiFace []MultiFaceElement

// Close implements the Face interface.
func (m MultiFace) Close() (retErr error) {
	for _, e := range m {
		if err := e.Face.Close(); retErr == nil {
			retErr = err
		}
	}
	return retErr
}

// Glyph implements the Face interface.
func (m MultiFace) Glyph(dot fixed.Point26_6, r rune) (
	newDot fixed.Point26_6, dr image.Rectangle, mask image.Image, maskp image.Point, ok bool) {

	// TODO: height/ascent adjustment, so that subfonts' baselines align.
	if e := m.find(r); e != nil {
		return e.Face.Glyph(dot, r)
	}
	return fixed.Point26_6{}, image.Rectangle{}, nil, image.Point{}, false
}

// Kern implements the Face interface.
func (m MultiFace) Kern(r0, r1 rune) fixed.Int26_6 {
	e0 := m.find(r0)
	e1 := m.find(r1)
	if e0 == e1 && e0 != nil {
		return e0.Face.Kern(r0, r1)
	}
	return 0
}

// find returns the first MultiFaceElement that contains r, or nil if there is
// no such element.
func (m MultiFace) find(r rune) *MultiFaceElement {
	// We have to do linear, not binary search. plan9port's
	// lucsans/unicode.8.font says:
	//	0x2591  0x2593  ../luc/Altshades.7.0
	//	0x2500  0x25ee  ../luc/FormBlock.7.0
	// and the rune ranges overlap.
	for i, e := range m {
		if e.Lo <= r && r <= e.Hi {
			return &m[i]
		}
	}
	return nil
}

// TODO: Drawer.Layout or Drawer.Measure methods to measure text without
// drawing?

// Drawer draws text on a destination image.
//
// A Drawer is not safe for concurrent use by multiple goroutines, since its
// Face is not.
type Drawer struct {
	// Dst is the destination image.
	Dst draw.Image
	// Src is the source image.
	Src image.Image
	// Face provides the glyph mask images.
	Face Face
	// Dot is the baseline location to draw the next glyph. The majority of the
	// affected pixels will be above and to the right of the dot, but some may
	// be below or to the left. For example, drawing a 'j' in an italic face
	// may affect pixels below and to the left of the dot.
	Dot fixed.Point26_6

	// TODO: Clip image.Image?
	// TODO: SrcP image.Point for Src images other than *image.Uniform? How
	// does it get updated during DrawString?
}

// TODO: should DrawString return the last rune drawn, so the next DrawString
// call can kern beforehand? Or should that be the responsibility of the caller
// if they really want to do that, since they have to explicitly shift d.Dot
// anyway?
//
// In general, we'd have a DrawBytes([]byte) and DrawRuneReader(io.RuneReader)
// and the last case can't assume that you can rewind the stream.
//
// TODO: how does this work with line breaking: drawing text up until a
// vertical line? Should DrawString return the number of runes drawn?

// DrawString draws s at the dot and advances the dot's location.
func (d *Drawer) DrawString(s string) {
	var prevC rune
	for i, c := range s {
		if i != 0 {
			d.Dot.X += d.Face.Kern(prevC, c)
		}
		newDot, dr, mask, maskp, ok := d.Face.Glyph(d.Dot, c)
		if !ok {
			// TODO: is falling back on the U+FFFD glyph the responsibility of
			// the Drawer or the Face?
			continue
		}
		draw.DrawMask(d.Dst, dr, d.Src, image.Point{}, mask, maskp, draw.Over)
		d.Dot, prevC = newDot, c
	}
}
