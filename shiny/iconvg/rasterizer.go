// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iconvg

import (
	"image"
	"image/color"
	"image/draw"

	"golang.org/x/image/math/f32"
	"golang.org/x/image/vector"
)

// Rasterizer is a Destination that draws an IconVG graphic onto a raster
// image.
//
// The zero value is usable, in that it has no raster image to draw onto, so
// that calling Decode with this Destination is a no-op (other than checking
// the encoded form for errors in the byte code). Call SetDstImage to change
// the raster image, before calling Decode or between calls to Decode.
type Rasterizer struct {
	z vector.Rasterizer

	dst      draw.Image
	r        image.Rectangle
	drawOp   draw.Op
	metadata Metadata

	firstStartPath  bool
	prevSmoothPoint f32.Vec2

	cSel uint32
	nSel uint32
	lod0 float32
	lod1 float32

	// scale and bias transforms the metadata.ViewBox rectangle to the (0, 0) -
	// (r.Dx(), r.Dy()) rectangle.
	scaleX float32
	biasX  float32
	scaleY float32
	biasY  float32

	creg [64]color.RGBA
	nreg [64]float32
}

// SetDstImage sets the Rasterizer to draw onto the given destination image,
// given by dst and r, with the given compositing operator.
//
// The IconVG graphic (which does not have a fixed size in pixels) will be
// scaled in the X and Y dimensions to fit the rectangle r. The scaling factors
// may differ in the two dimensions.
func (z *Rasterizer) SetDstImage(dst draw.Image, r image.Rectangle, drawOp draw.Op) {
	z.dst = dst
	if r.Empty() {
		r = image.Rectangle{}
	}
	z.r = r
	z.drawOp = drawOp
	z.recalcTransform()
}

func (z *Rasterizer) Reset(m Metadata) {
	z.metadata = m
	z.firstStartPath = true
	z.prevSmoothPoint = f32.Vec2{}
	z.cSel = 0
	z.nSel = 0
	z.lod0 = 0
	z.lod1 = positiveInfinity
	z.creg = [64]color.RGBA{}
	z.nreg = [64]float32{}
	z.recalcTransform()
}

func (z *Rasterizer) recalcTransform() {
	z.scaleX = float32(z.r.Dx()) / (z.metadata.ViewBox.Max[0] - z.metadata.ViewBox.Min[0])
	z.biasX = -z.metadata.ViewBox.Min[0]
	z.scaleY = float32(z.r.Dy()) / (z.metadata.ViewBox.Max[1] - z.metadata.ViewBox.Min[1])
	z.biasY = -z.metadata.ViewBox.Min[1]
}

func (z *Rasterizer) absX(x float32) float32 { return z.scaleX * (x + z.biasX) }
func (z *Rasterizer) absY(y float32) float32 { return z.scaleY * (y + z.biasY) }
func (z *Rasterizer) relX(x float32) float32 { return z.scaleX * x }
func (z *Rasterizer) relY(y float32) float32 { return z.scaleY * y }

func (z *Rasterizer) absVec2(x, y float32) f32.Vec2 {
	return f32.Vec2{z.absX(x), z.absY(y)}
}

func (z *Rasterizer) relVec2(x, y float32) f32.Vec2 {
	pen := z.z.Pen()
	return f32.Vec2{pen[0] + z.relX(x), pen[1] + z.relY(y)}
}

func (z *Rasterizer) implicitSmoothPoint() f32.Vec2 {
	pen := z.z.Pen()
	return f32.Vec2{
		2*pen[0] - z.prevSmoothPoint[0],
		2*pen[1] - z.prevSmoothPoint[1],
	}
}

func (z *Rasterizer) StartPath(adj int, x, y float32) {
	// TODO: note adj, use it in ClosePathEndPath.

	z.z.Reset(z.r.Dx(), z.r.Dy())
	if z.firstStartPath {
		z.firstStartPath = false
		z.z.DrawOp = z.drawOp
	}
	z.z.MoveTo(z.absVec2(x, y))
}

func (z *Rasterizer) ClosePathEndPath() {
	z.z.ClosePath()
	if z.dst == nil {
		return
	}
	// TODO: don't assume image.Opaque.
	z.z.Draw(z.dst, z.r, image.Opaque, image.Point{})
}

func (z *Rasterizer) ClosePathAbsMoveTo(x, y float32) {
	z.z.ClosePath()
	z.z.MoveTo(z.absVec2(x, y))
}

func (z *Rasterizer) ClosePathRelMoveTo(x, y float32) {
	z.z.ClosePath()
	z.z.MoveTo(z.relVec2(x, y))
}

func (z *Rasterizer) AbsHLineTo(x float32) {
	pen := z.z.Pen()
	z.z.LineTo(f32.Vec2{z.absX(x), pen[1]})
}

func (z *Rasterizer) RelHLineTo(x float32) {
	pen := z.z.Pen()
	z.z.LineTo(f32.Vec2{pen[0] + z.relX(x), pen[1]})
}

func (z *Rasterizer) AbsVLineTo(y float32) {
	pen := z.z.Pen()
	z.z.LineTo(f32.Vec2{pen[0], z.absY(y)})
}

func (z *Rasterizer) RelVLineTo(y float32) {
	pen := z.z.Pen()
	z.z.LineTo(f32.Vec2{pen[0], pen[1] + z.relY(y)})
}

func (z *Rasterizer) AbsLineTo(x, y float32) {
	z.z.LineTo(z.absVec2(x, y))
}

func (z *Rasterizer) RelLineTo(x, y float32) {
	z.z.LineTo(z.relVec2(x, y))
}

func (z *Rasterizer) AbsSmoothQuadTo(x, y float32) {
	z.prevSmoothPoint = z.implicitSmoothPoint()
	z.z.QuadTo(z.prevSmoothPoint, z.absVec2(x, y))
}

func (z *Rasterizer) RelSmoothQuadTo(x, y float32) {
	z.prevSmoothPoint = z.implicitSmoothPoint()
	z.z.QuadTo(z.prevSmoothPoint, z.relVec2(x, y))
}

func (z *Rasterizer) AbsQuadTo(x1, y1, x, y float32) {
	z.prevSmoothPoint = z.absVec2(x1, y1)
	z.z.QuadTo(z.prevSmoothPoint, z.absVec2(x, y))
}

func (z *Rasterizer) RelQuadTo(x1, y1, x, y float32) {
	z.prevSmoothPoint = z.relVec2(x1, y1)
	z.z.QuadTo(z.prevSmoothPoint, z.relVec2(x, y))
}

func (z *Rasterizer) AbsSmoothCubeTo(x2, y2, x, y float32) {
	p1 := z.implicitSmoothPoint()
	z.prevSmoothPoint = z.absVec2(x2, y2)
	z.z.CubeTo(p1, z.prevSmoothPoint, z.absVec2(x, y))
}

func (z *Rasterizer) RelSmoothCubeTo(x2, y2, x, y float32) {
	p1 := z.implicitSmoothPoint()
	z.prevSmoothPoint = z.relVec2(x2, y2)
	z.z.CubeTo(p1, z.prevSmoothPoint, z.relVec2(x, y))
}

func (z *Rasterizer) AbsCubeTo(x1, y1, x2, y2, x, y float32) {
	z.prevSmoothPoint = z.absVec2(x2, y2)
	z.z.CubeTo(z.absVec2(x1, y1), z.prevSmoothPoint, z.absVec2(x, y))
}

func (z *Rasterizer) RelCubeTo(x1, y1, x2, y2, x, y float32) {
	z.prevSmoothPoint = z.relVec2(x2, y2)
	z.z.CubeTo(z.relVec2(x1, y1), z.prevSmoothPoint, z.relVec2(x, y))
}

func (z *Rasterizer) AbsArcTo(rx, ry, xAxisRotation float32, largeArc, sweep bool, x, y float32) {
	// TODO: implement.
}

func (z *Rasterizer) RelArcTo(rx, ry, xAxisRotation float32, largeArc, sweep bool, x, y float32) {
	// TODO: implement.
}
