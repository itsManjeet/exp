// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package widget

import (
	"image"
	"image/draw"
)

// TODO: mask and maskPoint, not just src and srcRect.

// TODO: be able to specify the draw operator: Src instead of Over.

// TODO: be able to override the natural width and height, e.g. to specify a
// button's image in inches instead of (DPI-independent) pixels? Should that be
// the responsibility of this widget (Image) or of a Sizer shell widget?

// TODO: if the measured size differs from the actual size, specify a
// background color (or tile-able image like a checkerboard)? Specify a
// draw.Scaler from the golang.org/x/image/draw package? Be able to center the
// source image within the widget?

// Image is a leaf widget that paints an image.Image.
type Image struct{ *Node }

// NewImage returns a new Image widget for the part of a source image defined
// by src and srcRect.
func NewImage(src image.Image, srcRect image.Rectangle) Image {
	return Image{
		&Node{
			Class: ImageClass{},
			ClassData: &imageClassData{
				src:     src,
				srcRect: srcRect,
			},
		},
	}
}

func (o Image) Src() image.Image             { return o.classData().src }
func (o Image) SetSrc(v image.Image)         { o.classData().src = v }
func (o Image) SrcRect() image.Rectangle     { return o.classData().srcRect }
func (o Image) SetSrcRect(v image.Rectangle) { o.classData().srcRect = v }

func (o Image) classData() *imageClassData { return o.ClassData.(*imageClassData) }

type imageClassData struct {
	src     image.Image
	srcRect image.Rectangle
}

// ImageClass is the Class for Image nodes.
type ImageClass struct{ LeafClassEmbed }

func (k ImageClass) Measure(n *Node, t *Theme) {
	d := Image{n}.classData()
	n.MeasuredSize = d.srcRect.Size()
}

func (k ImageClass) Paint(n *Node, t *Theme, dst *image.RGBA, origin image.Point) {
	d := Image{n}.classData()
	if d.src == nil {
		return
	}

	// nRect is the node's layout rectangle, in dst's coordinate space.
	nRect := n.Rect.Add(origin)

	// sRect is the source image rectangle, in dst's coordinate space, so that
	// the upper-left corner of the source image rectangle aligns with the
	// upper-left corner of nRect.
	sRect := d.srcRect.Add(nRect.Min.Sub(d.srcRect.Min))

	draw.Draw(dst, nRect.Intersect(sRect), d.src, d.srcRect.Min, draw.Over)
}
