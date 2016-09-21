// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package widget

import (
	"image"
	"image/draw"
	"sort"
	"strings"
	"sync/atomic"

	"golang.org/x/exp/shiny/gesture"
	"golang.org/x/exp/shiny/text"
	"golang.org/x/exp/shiny/unit"
	"golang.org/x/exp/shiny/widget/node"
	"golang.org/x/exp/shiny/widget/theme"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

type TabAlign int

const (
	TabCenter TabAlign = iota
	TabLeft
)

func NewTabs(children ...node.Node) *Tabs {
	w := &Tabs{}
	w.Wrapper = w
	for _, c := range children {
		w.Insert(c, nil)
	}
	return w
}

type Tabs struct {
	node.ContainerEmbed

	Align TabAlign

	PreserveCase bool

	// TODO TabLayout for mobile: TabScrollable, TabFixed, TabDesktop

	// TODO: should we use a widget.Sheet for the tabs?
	headerOffset uintptr // atomic.LoadUintptr, atomic.StoreUintptr
	headerHeight int
	active       int

	tabOffsets []int
	tabWidths  []int
}

func (w *Tabs) OnInputEvent(e interface{}, origin image.Point) node.EventHandled {
	switch e := e.(type) {
	case gesture.Event:
		if e.Type != gesture.TypeTap {
			return node.NotHandled
		}
		headerRect := w.headerRect(origin)
		pos := image.Point{
			X: int(e.InitialPos.X),
			Y: int(e.InitialPos.Y),
		}
		if !pos.In(headerRect) {
			return node.NotHandled
		}
		if len(w.tabWidths) == 0 {
			return node.Handled
		}
		newActive := w.findTab(pos.X)
		if newActive == w.active {
			return node.Handled
		}
		w.active = newActive
		w.Mark(node.MarkNeedsPaintBase)
		return node.Handled
	}
	return node.NotHandled
}

func (w *Tabs) Measure(t *theme.Theme, widthHint, heightHint int) {
	mSize := image.Point{}
	hasText, hasIcons := false, false
	for c := w.FirstChild; c != nil; c = c.NextSibling {
		c.Wrapper.Measure(t, widthHint, heightHint)
		if mSize.Y < c.MeasuredSize.Y {
			mSize.Y = c.MeasuredSize.Y
		}
		if mSize.X < c.MeasuredSize.X {
			mSize.X = c.MeasuredSize.X
		}
		if d, ok := c.LayoutData.(TabsLayoutData); ok {
			if d.Label != "" {
				hasText = true
			}
			// TODO: d.Icon
		}
	}

	var (
		tabHeaderBoth           = unit.DIPs(72)
		tabHeaderJustTextOrIcon = unit.DIPs(48)
	)
	if hasText && hasIcons {
		w.headerHeight = t.Pixels(tabHeaderBoth).Round()
	} else {
		w.headerHeight = t.Pixels(tabHeaderJustTextOrIcon).Round()
	}
	mSize.Y += w.headerHeight

	mSize.X = t.Pixels(unit.DIPs(2*80 + float64(len(w.tabWidths))*2*24)).Round() // padding
	// TODO: mSize.X += text size

	w.MeasuredSize = mSize
}

func (w *Tabs) Layout(t *theme.Theme) {
	// TODO: subtract m.Rect.Min
	r := image.Rectangle{
		Min: image.Point{X: 0, Y: w.headerHeight},
		Max: w.Rect.Max,
	}
	numTabs := 0
	for c := w.FirstChild; c != nil; c = c.NextSibling {
		c.Rect = r
		c.Wrapper.Layout(t)
		numTabs++
	}
	w.tabOffsets = w.tabOffsets[:0]
	w.tabWidths = w.tabWidths[:0]

	maxWidth := t.Pixels(unit.DIPs(246)).Round()
	if w := w.Rect.Dx() - t.Pixels(unit.DIPs(56)).Round(); w < maxWidth {
		maxWidth = w
	}
	// TODO: There is a notion of "smaller views" for which the
	//       minWidth is 72dp. Work out what a smaller view is.
	minWidth := t.Pixels(unit.DIPs(160)).Round()

	fontOpts := theme.FontFaceOptions{
		Weight: font.WeightMedium,
	}
	face := t.GetFontFaceCatalog().AcquireFontFace(fontOpts)
	defer t.GetFontFaceCatalog().ReleaseFontFace(fontOpts, face)

	off := t.Pixels(unit.DIPs(56)).Round()
	for c := w.FirstChild; c != nil; c = c.NextSibling {
		label := ""
		if d, ok := c.LayoutData.(TabsLayoutData); ok {
			// TODO Icon
			label = d.Label
		}
		width := font.MeasureString(face, label).Round()
		if width < minWidth {
			width = minWidth
		}
		if width > maxWidth {
			width = maxWidth
		}
		w.tabOffsets = append(w.tabOffsets, off)
		w.tabWidths = append(w.tabWidths, width)
		off += width
	}
}

func (w *Tabs) findTab(x int) (tabIndex int) {
	// TODO adjust for headerOffset
	tabIndex = sort.SearchInts(w.tabOffsets, x)
	if tabIndex < len(w.tabOffsets) && w.tabOffsets[tabIndex] == x {
		return tabIndex
	}
	if tabIndex == len(w.tabOffsets) {
		if x > w.tabOffsets[tabIndex-1]+w.tabWidths[tabIndex-1] {
			return w.active // beyond end
		}
	}
	return tabIndex - 1
}

func (w *Tabs) headerRect(origin image.Point) image.Rectangle {
	r := w.Rect.Add(origin)
	r.Max.Y = r.Min.Y + w.headerHeight
	return r
}

func (w *Tabs) paintHeader(ctx *node.PaintBaseContext, origin image.Point) {
	headerRect := w.headerRect(origin)

	draw.Draw(ctx.Dst, headerRect, ctx.Theme.GetPalette().Dark(), image.Point{}, draw.Over)
	if len(w.tabWidths) == 0 {
		return
	}

	// Draw indicator.
	// TODO: animate indicator transition.
	headerOffset := int(atomic.LoadUintptr(&w.headerOffset))
	tabHeight := headerRect.Dy()
	indicatorHeight := ctx.Theme.Pixels(unit.DIPs(2))
	indicatorRect := image.Rectangle{
		Min: image.Point{
			X: w.tabOffsets[w.active] - headerOffset,
			Y: tabHeight - indicatorHeight.Round(),
		},
		Max: image.Point{
			X: headerOffset + w.tabOffsets[w.active] + w.tabWidths[w.active] - headerOffset,
			Y: tabHeight,
		},
	}
	indicatorRect = indicatorRect.Add(origin)
	draw.Draw(ctx.Dst, indicatorRect, ctx.Theme.GetPalette().Accent(), headerRect.Min, draw.Over)

	// Paint text.
	// TODO: adjust for scrolling through tab header with headerOffset
	fontOpts := theme.FontFaceOptions{Weight: font.WeightMedium}
	face := ctx.Theme.GetFontFaceCatalog().AcquireFontFace(fontOpts)
	defer ctx.Theme.GetFontFaceCatalog().ReleaseFontFace(fontOpts, face)
	dp24 := ctx.Theme.Pixels(unit.DIPs(24))
	oneLineTextDot := ctx.Theme.Pixels(unit.DIPs(20))
	twoLinesTextDot := ctx.Theme.Pixels(unit.DIPs(12))
	i := 0
	for c := w.FirstChild; c != nil; c = c.NextSibling {
		label := ""
		if d, ok := c.LayoutData.(TabsLayoutData); ok {
			// TODO Icon
			label = d.Label
		}
		if label == "" {
			i++
			continue
		}
		if !w.PreserveCase {
			label = strings.ToUpper(label)
		}
		marginLeft, marginRight := dp24, dp24

		tabRect := image.Rectangle{
			Min: image.Point{
				// TODO headerOffset
				X: w.tabOffsets[i],
				Y: 0,
			},
			Max: image.Point{
				X: w.tabOffsets[i] + w.tabWidths[i],
				Y: tabHeight,
			},
		}
		tabRect = tabRect.Add(origin)

		maxTextWidth := fixed.I(w.tabWidths[i]) - marginLeft - marginRight
		f := new(text.Frame)
		f.SetFace(face)
		f.SetMaxWidth(maxTextWidth)

		// TODO: If more than two lines, replace final visible character with "…".
		c := f.NewCaret()
		c.WriteString(label)
		c.Close()

		p := f.FirstParagraph()
		lineNum := 0
		for l := p.FirstLine(f); l != nil && lineNum < 2; l = l.Next(f) {
			for b := l.FirstBox(f); b != nil && lineNum < 2; b = b.Next(f) {
				text := b.TrimmedText(f)
				extraLeftPad := fixed.Int26_6(0)
				if w.Align == TabCenter {
					width := font.MeasureBytes(face, text)
					extraLeftPad = (maxTextWidth - width) * 64 / fixed.I(2)
				}
				d := font.Drawer{
					Dst:  ctx.Dst.SubImage(tabRect).(*image.RGBA),
					Src:  ctx.Theme.GetPalette().Accent(),
					Face: face,
					Dot: fixed.Point26_6{
						X: fixed.I(w.tabOffsets[i]) + marginLeft + extraLeftPad,
						Y: fixed.I(tabHeight),
					},
				}
				if f.LineCount() == 1 {
					d.Dot.Y -= oneLineTextDot
				} else {
					d.Dot.Y -= twoLinesTextDot
					if lineNum == 0 {
						d.Dot.Y -= face.Metrics().Height
					}
				}
				d.DrawBytes(text)

				if b.Next(f) != nil {
					panic("more than one box on a line")
				}
				lineNum++
			}
		}
		i++
	}

	// (Re-)Paint margin backgrounds (may draw over scrolled buttons).
	marginLeftRect := image.Rectangle{
		Min: image.Point{X: 0, Y: 0},
		Max: image.Point{
			X: ctx.Theme.Pixels(unit.DIPs(56)).Round(),
			Y: tabHeight,
		},
	}
	marginRightRect := image.Rectangle{
		Min: image.Point{
			X: headerRect.Max.X - ctx.Theme.Pixels(unit.DIPs(56)).Round(),
			Y: 0,
		},
		Max: headerRect.Max,
	}
	marginLeftRect.Add(origin)
	marginRightRect.Add(origin)
	draw.Draw(ctx.Dst, marginLeftRect, ctx.Theme.GetPalette().Dark(), image.Point{}, draw.Over)
	draw.Draw(ctx.Dst, marginRightRect, ctx.Theme.GetPalette().Dark(), image.Point{}, draw.Over)

	// Paint scroll arrow buttons.
	// TODO: can this be replaced by a button widget?
	end := w.tabOffsets[len(w.tabOffsets)-1] + w.tabWidths[len(w.tabWidths)-1]
	needsScroll := end > headerRect.Max.X
	if needsScroll {
		// Draw left arrow.
		// TODO: use material design icon "keyboard arrow left".
		color := ctx.Theme.GetPalette().Background()
		if headerOffset > 0 {
			color = ctx.Theme.GetPalette().Accent() // active
		}
		d := font.Drawer{
			Dst:  ctx.Dst.SubImage(marginLeftRect).(*image.RGBA),
			Src:  color,
			Face: face,
			Dot: fixed.Point26_6{
				X: fixed.I(marginLeftRect.Min.X),
				Y: fixed.I(marginLeftRect.Max.Y),
			},
		}
		d.DrawString("<")

		// Draw right arrow.
		// TODO: use material design icon "keyboard arrow right".
		color = ctx.Theme.GetPalette().Background()
		if end-headerOffset > headerRect.Max.X {
			color = ctx.Theme.GetPalette().Accent() // active
		}
		d = font.Drawer{
			Dst:  ctx.Dst.SubImage(marginRightRect).(*image.RGBA),
			Src:  color,
			Face: face,
			Dot: fixed.Point26_6{
				X: fixed.I(marginRightRect.Min.X),
				Y: fixed.I(marginRightRect.Max.Y),
			},
		}
		d.DrawString(">")
	}
}

func (w *Tabs) Paint(ctx *node.PaintContext, origin image.Point) error {
	w.Marks.UnmarkNeedsPaint()
	c := w.FirstChild
	for i := w.active; i > 0; i-- {
		c = c.NextSibling
	}
	c.Wrapper.Paint(ctx, origin.Add(w.Rect.Min))
	return nil
}

func (w *Tabs) PaintBase(ctx *node.PaintBaseContext, origin image.Point) error {
	w.Marks.UnmarkNeedsPaintBase()
	w.paintHeader(ctx, origin)
	c := w.FirstChild
	for i := w.active; i > 0; i-- {
		c = c.NextSibling
	}
	c.Wrapper.PaintBase(ctx, origin.Add(w.Rect.Min))
	return nil
}

type TabsLayoutData struct {
	// TODO Icon *Icon
	Label string
}
