// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package widget

import (
	"image"
	"image/draw"
	"sync"

	"golang.org/x/exp/shiny/screen"
	"golang.org/x/exp/shiny/widget/node"
	"golang.org/x/exp/shiny/widget/theme"
	"golang.org/x/image/math/f64"
	"golang.org/x/mobile/event/lifecycle"
	"golang.org/x/mobile/event/mouse"
)

const (
	tileLength = 256
	tileMask   = tileLength - 1
)

// TODO: make this configurable? Theme DPI dependent? OS dependent?
//
// TODO: do things like a touchpad's synthetic 'wheel' events have richer delta
// information we should use instead?
const buttonWheelDelta = 16

type tile struct {
	buf  screen.Buffer
	tex  screen.Texture
	used bool
}

func (t *tile) release() {
	t.buf.Release()
	t.tex.Release()
}

// Sheet is a shell widget that provides *image.RGBA pixel buffers (analogous
// to blank sheets of paper) for its descendent widgets to paint on, via their
// PaintBase methods. Such buffers may be cached and their contents re-used for
// multiple paints, which can make scrolling and animation smoother and more
// efficient.
//
// A simple app may have only one Sheet, near the root of its widget tree. A
// more complicated app may have multiple Sheets. For example, consider a text
// editor consisting of a small header bar and a large text widget. Those two
// nodes may be backed by two separate Sheets, since scrolling the latter
// should not scroll the former.
type Sheet struct {
	node.ShellEmbed

	scroll      Axis
	scrollStart image.Point // origin of scroll gesture
	origin      image.Point // current sheet scroll offset
	lastRect    image.Rectangle

	m map[image.Point]*tile
}

// NewSheet returns a new Sheet widget.
func NewSheet(scroll Axis, inner node.Node) *Sheet {
	w := &Sheet{
		scroll: scroll,
		m:      make(map[image.Point]*tile),
	}
	w.Wrapper = w
	if inner != nil {
		w.Insert(inner, nil)
	}
	return w
}

func (m *Sheet) Layout(t *theme.Theme) {
	if c := m.FirstChild; c != nil {
		c.Rect = m.Rect.Sub(m.Rect.Min)
		if m.scroll.Horizontal() && c.MeasuredSize.X > c.Rect.Max.X {
			c.Rect.Max.X = c.MeasuredSize.X
		}
		if m.scroll.Vertical() && c.MeasuredSize.Y > c.Rect.Max.Y {
			c.Rect.Max.Y = c.MeasuredSize.Y
		}
		c.Wrapper.Layout(t)
	}
}

func (w *Sheet) release() {
	for offset, t := range w.m {
		t.release()
		delete(w.m, offset)
	}
}

func (w *Sheet) clampOrigin() {
	c := w.FirstChild
	if c == nil {
		return
	}
	if w.origin.X < 0 {
		w.origin.X = 0
	}
	if w.origin.Y < 0 {
		w.origin.Y = 0
	}
	if m := c.MeasuredSize.X - w.Rect.Dx(); w.origin.X > m {
		w.origin.X = m
	}
	if m := c.MeasuredSize.Y - w.Rect.Dy(); w.origin.Y > m {
		w.origin.Y = m
	}
}

func (w *Sheet) Paint(ctx *node.PaintContext, origin image.Point) (err error) {
	w.Marks.UnmarkNeedsPaint()
	c := w.FirstChild
	if c == nil {
		w.release()
		return nil
	}

	// Pool to reuse sheet tiles.
	// Catches the common case that the child needs repainting.
	var tilePool []*tile
	newTile := func() (*tile, error) {
		if len(tilePool) > 0 {
			t := tilePool[0]
			tilePool = tilePool[1:]
			return t, err
		}
		buf, err := ctx.Screen.NewBuffer(image.Point{X: tileLength, Y: tileLength})
		if err != nil {
			return nil, err
		}
		tex, err := ctx.Screen.NewTexture(image.Point{X: tileLength, Y: tileLength})
		if err != nil {
			return nil, err
		}
		return &tile{
			buf: buf,
			tex: tex,
		}, nil
	}

	if c.Marks.NeedsPaintBase() || w.Rect != w.lastRect {
		tilePool = make([]*tile, 0, len(w.m))
		for p, t := range w.m {
			tilePool = append(tilePool, t)
			delete(w.m, p)
		}
	}
	for _, t := range w.m {
		t.used = false
	}
	w.lastRect = w.Rect

	var wg sync.WaitGroup
	xMax := w.Rect.Max.X + w.origin.X
	yMax := w.Rect.Max.Y + w.origin.Y
	for y := w.origin.Y &^ tileMask; y < yMax; y += tileLength {
		for x := w.origin.X &^ tileMask; x < xMax; x += tileLength {
			offset := image.Point{
				X: x,
				Y: y,
			}
			t, found := w.m[offset]
			if found {
				t.used = true
				continue
			}

			t, err = newTile()
			if err != nil {
				return err
			}
			w.m[offset] = t

			c.Wrapper.PaintBase(&node.PaintBaseContext{
				Theme: ctx.Theme,
				Dst:   t.buf.RGBA(),
			}, image.Point{X: -x, Y: -y})
			t.used = true

			wg.Add(1)
			go func() {
				// TODO: if we required PaintBase to be safe for
				// concurrent calls, we could move it here.
				t.tex.Upload(image.Point{}, t.buf, t.buf.Bounds())
				wg.Done()
			}()
		}
	}
	for p, t := range w.m {
		if !t.used {
			t.release()
			delete(w.m, p)
		}
	}
	for _, t := range tilePool {
		t.release()
	}
	wg.Wait()

	for offset, t := range w.m {
		src2dst := ctx.Src2Dst
		b := t.tex.Bounds()
		if offset.X < w.origin.X {
			b.Min.X += w.origin.X - offset.X
		}
		if offset.Y < w.origin.Y {
			b.Min.Y += w.origin.Y - offset.Y
		}
		if maxX := offset.X + tileLength; maxX > w.Rect.Max.X {
			b.Max.X -= w.Rect.Max.X - maxX
		}
		if maxY := offset.Y + tileLength; maxY > w.Rect.Max.Y {
			b.Max.Y -= w.Rect.Max.Y - maxY
		}
		translate(&src2dst,
			float64(origin.X+w.Rect.Min.X+offset.X-w.origin.X),
			float64(origin.Y+w.Rect.Min.Y+offset.Y-w.origin.Y),
		)
		// TODO: should draw.Over be configurable?
		ctx.Drawer.Draw(src2dst, t.tex, b, draw.Over, nil)
	}

	return c.Wrapper.Paint(ctx, origin.Add(w.Rect.Min))
}

func translate(a *f64.Aff3, tx, ty float64) {
	a[2] += a[0]*tx + a[1]*ty
	a[5] += a[3]*tx + a[4]*ty
}

func (w *Sheet) PaintBase(ctx *node.PaintBaseContext, origin image.Point) error {
	w.Marks.UnmarkNeedsPaintBase()
	// Do not recursively call PaintBase on our children. We create our own
	// buffers, and Sheet.Paint will call PaintBase with our PaintBaseContext
	// instead of our ancestor's.
	return nil
}

func (w *Sheet) OnChildMarked(child node.Node, newMarks node.Marks) {
	if newMarks&node.MarkNeedsPaintBase != 0 {
		newMarks &^= node.MarkNeedsPaintBase
		newMarks |= node.MarkNeedsPaint
	}
	w.Mark(newMarks)
}

func (w *Sheet) OnLifecycleEvent(e lifecycle.Event) {
	if e.Crosses(lifecycle.StageVisible) == lifecycle.CrossOff {
		w.release()
	}
}

func (w *Sheet) OnInputEvent(e interface{}, origin image.Point) node.EventHandled {
	if w.ShellEmbed.OnInputEvent(e, origin.Sub(w.origin)) == node.Handled {
		return node.Handled
	}
	if w.scroll != AxisNone {
		switch e := e.(type) {
		// TODO: gesture.Event
		case mouse.Event:
			if !e.Button.IsWheel() {
				break
			}
			switch e.Button {
			case mouse.ButtonWheelUp:
				if w.scroll&AxisVertical != 0 {
					w.origin.Y -= buttonWheelDelta
				}
			case mouse.ButtonWheelDown:
				if w.scroll&AxisVertical != 0 {
					w.origin.Y += buttonWheelDelta
				}
			case mouse.ButtonWheelLeft:
				if w.scroll&AxisHorizontal != 0 {
					w.origin.X -= buttonWheelDelta
				}
			case mouse.ButtonWheelRight:
				if w.scroll&AxisHorizontal != 0 {
					w.origin.X += buttonWheelDelta
				}
			}
			w.clampOrigin()
			w.Mark(node.MarkNeedsPaint)
			return node.Handled
		}
	}
	return node.NotHandled
}
