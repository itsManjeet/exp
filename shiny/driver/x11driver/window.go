// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x11driver

import (
	"image"
	"image/draw"
	"log"
	"sync"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/render"
	"github.com/BurntSushi/xgb/xproto"

	"golang.org/x/exp/shiny/driver/internal/pump"
	"golang.org/x/exp/shiny/screen"
	"golang.org/x/image/math/f64"
)

type windowImpl struct {
	s *screenImpl

	xw xproto.Window
	xg xproto.Gcontext
	xp render.Picture

	pump    pump.Pump
	xevents chan xgb.Event

	mu       sync.Mutex
	released bool
}

func (w *windowImpl) run() {
	for {
		select {
		// TODO: things other than X11 events.

		case ev, ok := <-w.xevents:
			if !ok {
				return
			}
			switch ev := ev.(type) {
			default:
				// TODO: implement.
				log.Println(ev)
			}
		}
	}
}

func (w *windowImpl) Events() <-chan interface{} { return w.pump.Events() }
func (w *windowImpl) Send(event interface{})     { w.pump.Send(event) }

func (w *windowImpl) Release() {
	w.mu.Lock()
	released := w.released
	w.released = true
	w.mu.Unlock()

	if released {
		return
	}
	render.FreePicture(w.s.xc, w.xp)
	xproto.FreeGC(w.s.xc, w.xg)
	xproto.DestroyWindow(w.s.xc, w.xw)
	w.pump.Release()
}

func (w *windowImpl) Upload(dp image.Point, src screen.Buffer, sr image.Rectangle, sender screen.Sender) {
	upload(w.s, w, xproto.Drawable(w.xw), w.xg, w.s.xsi.RootDepth, dp, src.(*bufferImpl), sr, sender)
}

func (w *windowImpl) Draw(src2dst f64.Aff3, src screen.Texture, sr image.Rectangle, op draw.Op, opts *screen.DrawOptions) {
	t := src.(*textureImpl)
	renderOp := uint8(render.PictOpOver)
	if op == draw.Src {
		renderOp = render.PictOpSrc
	}

	// TODO: honor all of src2dst, not just the translation.
	dstX := int(src2dst[2]) - sr.Min.X
	dstY := int(src2dst[5]) - sr.Min.Y

	render.Composite(w.s.xc, renderOp, t.xp, 0, w.xp,
		int16(sr.Min.X), int16(sr.Min.Y), // SrcX, SrcY,
		0, 0, // MaskX, MaskY,
		int16(dstX), int16(dstY), // DstX, DstY,
		uint16(sr.Dx()), uint16(sr.Dy()), // Width, Height,
	)
}

func (w *windowImpl) EndPaint() {
	// TODO.
}
