// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flex

import (
	"image"
	"image/color"
	"testing"

	"golang.org/x/exp/shiny/unit"
	"golang.org/x/exp/shiny/widget"
	"golang.org/x/exp/shiny/widget/node"
)

type layoutTest struct {
	direction    Direction
	wrap         FlexWrap
	alignContent AlignContent
	size         image.Point       // size of container
	measured     [][2]float64      // MeasuredSize of child elements
	layoutData   []LayoutData      // LayoutData of child elements
	want         []image.Rectangle // final Rect of child elements
}

var tileColors = []color.RGBA{
	color.RGBA{0x00, 0x7f, 0x7f, 0xff}, // Cyan
	color.RGBA{0x7f, 0x00, 0x7f, 0xff}, // Magenta
	color.RGBA{0x7f, 0x7f, 0x00, 0xff}, // Yellow
	color.RGBA{0xff, 0x00, 0x00, 0xff},
	color.RGBA{0x00, 0xff, 0x00, 0xff},
	color.RGBA{0x00, 0x00, 0xff, 0xff},
}

var layoutTests = []layoutTest{
	{
		size:     image.Point{100, 100},
		measured: [][2]float64{{100, 100}},
		want: []image.Rectangle{
			{size(0, 0), size(100, 100)},
		},
	},
}

func size(x, y int) image.Point { return image.Pt(x, y) }
func sizeptr(x, y int) *image.Point {
	s := size(x, y)
	return &s
}

func TestLayout(t *testing.T) {
	for testNum, test := range layoutTests {
		fl := NewFlex()
		fl.Direction = test.direction
		fl.Wrap = test.wrap
		fl.AlignContent = test.alignContent

		var children []node.Node
		for i, sz := range test.measured {
			n := widget.NewUniform(tileColors[i], unit.Pixels(sz[0]), unit.Pixels(sz[1]))
			if test.layoutData != nil {
				n.LayoutData = test.layoutData[i]
			}
			fl.AppendChild(n)
			children = append(children, n)
		}

		fl.Measure(nil)
		fl.Rect = image.Rectangle{Max: test.size}
		fl.Layout(nil)

		bad := false
		for i, n := range children {
			if n.Wrappee().Rect != test.want[i] {
				bad = true
				break
			}
		}
		if bad {
			t.Logf("Bad testNum %d", testNum)
			// TODO print html so we can see the correct layout
		}
		for i, n := range children {
			if got := n.Wrappee().Rect; got != test.want[i] {
				t.Errorf("\t[%d].Rect=%v, want %v", i, got, test.want[i])
			}
		}
	}
}
