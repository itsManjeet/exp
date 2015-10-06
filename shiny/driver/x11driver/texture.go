// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x11driver

import (
	"image"
	"image/color"
	"image/draw"
	"log"
	"sync"

	"github.com/BurntSushi/xgb/render"
	"github.com/BurntSushi/xgb/xproto"

	"golang.org/x/exp/shiny/screen"
	"golang.org/x/image/math/f64"
)

const textureDepth = 32

type textureImpl struct {
	s *screenImpl

	size image.Point
	xm   xproto.Pixmap
	xp   render.Picture

	mu       sync.Mutex
	released bool
}

func (t *textureImpl) Size() image.Point       { return t.size }
func (t *textureImpl) Bounds() image.Rectangle { return image.Rectangle{Max: t.size} }

func (t *textureImpl) Release() {
	t.mu.Lock()
	released := t.released
	t.released = true
	t.mu.Unlock()

	if released {
		return
	}
	render.FreePicture(t.s.xc, t.xp)
	xproto.FreePixmap(t.s.xc, t.xm)
}

func (t *textureImpl) Upload(dp image.Point, src screen.Buffer, sr image.Rectangle, sender screen.Sender) {
	src.(*bufferImpl).upload(t, xproto.Drawable(t.xm), t.s.gcontext32, textureDepth, dp, sr, sender)
}

func (t *textureImpl) Fill(dr image.Rectangle, src color.Color, op draw.Op) {
	fill(t.s.xc, t.xp, dr, src, op)
}

func f64ToFixed(x float64) render.Fixed {
	return render.Fixed(x * 65536)
}

func inv(x *f64.Aff3) *f64.Aff3 {
	return &f64.Aff3{
		x[4] / (x[0]*x[4] - x[1]*x[3]),
		x[1] / (x[1]*x[3] - x[0]*x[4]),
		(x[2]*x[4] - x[1]*x[5]) / (x[1]*x[3] - x[0]*x[4]),
		x[3] / (x[1]*x[3] - x[0]*x[4]),
		x[0] / (x[0]*x[4] - x[1]*x[3]),
		(x[2]*x[3] - x[0]*x[5]) / (x[0]*x[4] - x[1]*x[3]),
	}
}

func (t *textureImpl) draw(xp render.Picture, src2dst *f64.Aff3, sr image.Rectangle, op draw.Op, w, h int, opts *screen.DrawOptions) {
	pictOp := byte(render.PictOpOver)
	dstX, dstY := 0, 0

	if src2dst[1] == 0 && src2dst[3] == 0 {
		// Drawing a square onto a square, we can honor the draw op.
		pictOp = renderOp(op)

		// We do the translation component of the transform by clipping
		// the composition region in render.Composite below.
		w, h = sr.Dx(), sr.Dy()
		dstX = int(src2dst[2]) - sr.Min.X
		dstY = int(src2dst[5]) - sr.Min.Y
		src2dst[2] = 0
		src2dst[5] = 0
	} else {
		// Drawing a square onto an aribtrary region of xp.
		//
		// As the composite extension maps from dst to src and we don't
		// know what pixels on dst we are going to touch, we cannot honor
		// the draw op.
		if sr.Max != t.size {
			// TODO: support sr.Max. We cannot do so with SetPictureClipRectangles
			// to do it as that applies before the affine transform. We probably
			// have to generate an intermediate picture.
			log.Printf("x11driver.Draw: sr.Max not supported for arbitrary affine transforms")
		}
	}

	// The XTransform matrix maps from destination pixels to source
	// pixels, so we invert src2dst.
	dst2src := inv(src2dst)
	render.SetPictureTransform(t.s.xc, t.xp, render.Transform{
		f64ToFixed(dst2src[0]), f64ToFixed(dst2src[1]), f64ToFixed(dst2src[2]),
		f64ToFixed(dst2src[3]), f64ToFixed(dst2src[4]), f64ToFixed(dst2src[5]),
		f64ToFixed(0), f64ToFixed(0), f64ToFixed(1),
	})
	render.SetPictureFilter(t.s.xc, t.xp, uint16(len("bilinear")), "bilinear", nil)

	render.Composite(t.s.xc, pictOp, t.xp, 0, xp,
		int16(sr.Min.X), int16(sr.Min.Y), // SrcX, SrcY,
		0, 0, // MaskX, MaskY,
		int16(dstX), int16(dstY), // DstX, DstY,
		uint16(w), uint16(h), // Width, Height,
	)
}

func renderOp(op draw.Op) byte {
	if op == draw.Src {
		return render.PictOpSrc
	}
	return render.PictOpOver
}
