// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package widget

import (
	"testing"

	"golang.org/x/exp/shiny/unit"
	"golang.org/x/image/math/fixed"
)

func TestThemeIsAUnitConverter(t *testing.T) {
	// 1.5 inches (at the default 72 DPI) should be 108 pixels.
	c := unit.Converter(DefaultTheme)
	got := c.Pixels(unit.Inches(1.5))
	want := fixed.I(108)
	if got != want {
		t.Errorf("1 inch in pixels: got %v, want %v", got, want)
	}

	// 3 em (at basicfont.Face7x13's 13 pixel em-height) should be 39 pixels.
	for _, dpi := range []float64{72, 160} {
		c := unit.Converter(&Theme{
			DPI: dpi,
		})
		got := c.Convert(unit.Ems(3), unit.Pt)
		want := unit.Points(39 * unit.PointsPerInch / dpi)
		if got != want {
			t.Errorf("dpi=%v: 3 em in points: got %v, want %v", dpi, got, want)
		}
	}
}
