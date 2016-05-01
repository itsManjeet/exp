// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flex

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"testing"

	"golang.org/x/exp/shiny/unit"
	"golang.org/x/exp/shiny/widget"
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

func (test *layoutTest) html() string {
	buf := new(bytes.Buffer)
	fmt.Fprintf(buf, `<style>
#container {
	display: flex;
	width:   %dpx;
	height:  %dpx;
`, test.size.X, test.size.Y)

	switch test.direction {
	case Row:
	case RowReverse:
		fmt.Fprintf(buf, "\tflex-direction: row-reverse;\n")
	case Column:
		fmt.Fprintf(buf, "\tflex-direction: column;\n")
	case ColumnReverse:
		fmt.Fprintf(buf, "\tflex-direction: column-reverse;\n")
	}
	switch test.wrap {
	case NoWrap:
	case Wrap:
		fmt.Fprintf(buf, "\tflex-wrap: wrap;\n")
	case WrapReverse:
		fmt.Fprintf(buf, "\tflex-wrap: wrap-reverse;\n")
	}
	switch test.alignContent {
	case AlignContentStart:
	case AlignContentEnd:
		fmt.Fprintf(buf, "\talign-content: flex-end;\n")
	case AlignContentCenter:
		fmt.Fprintf(buf, "\talign-content: center;\n")
	case AlignContentSpaceBetween:
		fmt.Fprintf(buf, "\talign-content: space-between;\n")
	case AlignContentSpaceAround:
		fmt.Fprintf(buf, "\talign-content: space-around;\n")
	case AlignContentStretch:
		fmt.Fprintf(buf, "\talign-content: stretch;\n")
	}
	fmt.Fprintf(buf, "}\n")

	for i, m := range test.measured {
		fmt.Fprintf(buf, `#child%d {
	width: %.2fpx;
	height: %.2fpx;
`, i, m[0], m[1])
		c := tileColors[i]
		fmt.Fprintf(buf, "\tbackground-color: rgb(%d, %d, %d);\n", c.R, c.G, c.B)
		if test.layoutData != nil {
			d := test.layoutData[i]
			if d.MinSize.X != 0 {
				fmt.Fprintf(buf, "\tmin-width: %dpx;\n", d.MinSize.X)
			}
			if d.MinSize.Y != 0 {
				fmt.Fprintf(buf, "\tmin-height: %dpx;\n", d.MinSize.Y)
			}
			if d.MaxSize != nil {
				fmt.Fprintf(buf, "\tmax-width: %dpx;\n", d.MaxSize.X)
				fmt.Fprintf(buf, "\tmax-height: %dpx;\n", d.MaxSize.Y)
			}
			if d.Grow != 0 {
				fmt.Fprintf(buf, "\tflex-grow: %f;\n", d.Grow)
			}
			if d.Shrink != nil {
				fmt.Fprintf(buf, "\tflex-shrink: %f;\n", *d.Shrink)
			}
			// TODO: Basis, Align, BreakAfter
		}
		fmt.Fprintf(buf, "}\n")
	}
	fmt.Fprintf(buf, `</style>
<div id="container">
`)
	for i := range test.measured {
		fmt.Fprintf(buf, "\t<div id=\"child%d\"></div>\n", i)
	}
	fmt.Fprintf(buf, `</div>
<pre id="out"></pre>
<script>
var out = document.getElementById("out");
var container = document.getElementById("container");
for (var i = 0; i < container.children.length; i++) {
	var c = container.children[i];
	var ctop = c.offsetTop - container.offsetTop;
	var cleft = c.offsetLeft - container.offsetLeft;
	var cbottom = ctop + c.offsetHeight;
	var cright = cleft + c.offsetWidth;

	out.innerHTML += "\t{size(" + cleft + ", " + ctop + "), size(" + cright + ", " + cbottom + ")},\n";
}
</script>
`)

	return buf.String()
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
		size:     image.Point{350, 100},
		measured: [][2]float64{{100, 100}, {100, 100}, {100, 100}},
		want: []image.Rectangle{
			{size(0, 0), size(100, 100)},
			{size(100, 0), size(200, 100)},
			{size(200, 0), size(300, 100)},
		},
	},
	{
		size:     image.Point{300, 100},
		measured: [][2]float64{{100, 100}, {100, 100}},
		want: []image.Rectangle{
			{size(0, 0), size(100, 100)},
			{size(100, 0), size(300, 100)},
		},
		layoutData: []LayoutData{{}, {Grow: 1}},
	},
	{
		size:     image.Point{300, 100},
		measured: [][2]float64{{50, 50}, {100, 100}, {100, 100}},
		want: []image.Rectangle{
			{size(0, 0), size(50, 50)},
			{size(50, 0), size(175, 100)},
			{size(175, 0), size(300, 100)},
		},
		layoutData: []LayoutData{{}, {Grow: 1}, {Grow: 1}},
	},
	{
		size:     image.Point{300, 100},
		measured: [][2]float64{{20, 100}, {20, 100}, {20, 100}},
		want: []image.Rectangle{
			{size(0, 0), size(30, 100)},
			{size(30, 0), size(130, 100)},
			{size(130, 0), size(300, 100)},
		},
		layoutData: []LayoutData{
			{MaxSize: sizeptr(30, 100), Grow: 1},
			{MinSize: size(100, 0), Grow: 1},
			{Grow: 4},
		},
	},
	{
		size:     image.Point{300, 200},
		wrap:     Wrap,
		measured: [][2]float64{{150, 100}, {160, 100}, {20, 100}},
		want: []image.Rectangle{
			{size(0, 0), size(30, 100)},
			{size(0, 100), size(220, 200)},
			{size(220, 100), size(300, 200)},
		},
		layoutData: []LayoutData{
			{MaxSize: sizeptr(30, 100), Grow: 1},
			{MinSize: size(100, 0), Grow: 1},
			{Grow: 1},
		},
	},
	{
		size:         image.Point{300, 200},
		direction:    Column,
		wrap:         Wrap,
		alignContent: AlignContentSpaceBetween,
		measured:     [][2]float64{{150, 100}, {160, 100}, {20, 100}, {300, 300}},
		want: []image.Rectangle{
			{size(0, 0), size(30, 100)},
			{size(0, 100), size(160, 200)},
			{size(218, 0), size(238, 200)},
			{size(295, 0), size(300, 5)},
		},
		layoutData: []LayoutData{
			{MaxSize: sizeptr(30, 100), Grow: 1},
			{MinSize: size(100, 0), Grow: 1},
			{Grow: 1},
			{MaxSize: sizeptr(5, 5)},
		},
	},
	{
		size:      image.Point{300, 60},
		direction: ColumnReverse,
		wrap:      Wrap,
		measured:  [][2]float64{{25, 25}, {25, 25}, {25, 25}, {25, 25}, {25, 25}},
		want: []image.Rectangle{
			{size(0, 35), size(25, 60)},
			{size(0, 0), size(25, 35)},
			{size(100, 35), size(125, 60)},
			{size(100, 10), size(125, 35)},
			{size(200, 0), size(225, 60)},
		},
		layoutData: []LayoutData{
			{},
			{Grow: 1},
			{},
			{},
			{Grow: 1},
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

		var children []*widget.Node
		for i, sz := range test.measured {
			n := widget.NewUniform(tileColors[i], unit.Pixels(sz[0]), unit.Pixels(sz[1])).Node
			if test.layoutData != nil {
				n.LayoutData = test.layoutData[i]
			}
			fl.AppendChild(n)
			children = append(children, n)
		}

		fl.Node.Class.Measure(&fl.Node, nil)
		fl.Node.Rect = image.Rectangle{Max: test.size}
		fl.Node.Class.Layout(&fl.Node, nil)

		bad := false
		for i, n := range children {
			if n.Rect != test.want[i] {
				bad = true
				break
			}
		}
		if bad {
			t.Logf("Bad testNum %d:\n%s", testNum, test.html())
		}
		for i, n := range children {
			if n.Rect != test.want[i] {
				t.Errorf("\t[%d].Rect=%v, want %v", i, n.Rect, test.want[i])
			}
		}
	}
}
