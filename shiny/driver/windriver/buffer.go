// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build windows

package windriver

import (
	"image"

	"golang.org/x/exp/shiny/screen"
)

type buffer struct {
	rgba *image.RGBA
}

func newBuffer(size image.Point) (screen.Buffer, error) {
	b := buffer{
		image.NewRGBA(image.Rectangle{Max: size}),
	}
	return &b, nil
}

func (b *buffer) Release() {
	b.rgba = nil
}

func (b *buffer) Size() image.Point {
	return b.rgba.Rect.Max
}

func (b *buffer) Bounds() image.Rectangle {
	return b.rgba.Rect
}

func (b *buffer) RGBA() *image.RGBA {
	return b.rgba
}
