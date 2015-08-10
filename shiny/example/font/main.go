// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore
//
// This build tag means that "go install golang.org/x/exp/shiny/..." doesn't
// install this example program. Use "go run main.go" to explicitly run it.

// Program font is a basic example of fonts.
package main

import (
	"flag"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io/ioutil"
	"log"
	"os"

	"golang.org/x/exp/shiny/font"
	"golang.org/x/exp/shiny/font/plan9font"
	"golang.org/x/image/math/fixed"
)

var (
	subfont   = flag.String("subfont", "", `filename of the Plan 9 subfont file, such as "lucsans/lsr.14"`)
	firstRune = flag.Int("firstrune", 0, "the Unicode code point of the first rune in the subfont file")
)

func pt(p fixed.Point26_6) image.Point {
	return image.Point{
		X: int(p.X+32) >> 6,
		Y: int(p.Y+32) >> 6,
	}
}

func main() {
	flag.Parse()

	// TODO: mmap the file.
	if *subfont == "" {
		flag.Usage()
		log.Fatal("no subfont specified")
	}
	fontData, err := ioutil.ReadFile(*subfont)
	if err != nil {
		log.Fatal(err)
	}
	face, err := plan9font.ParseSubfont(fontData, rune(*firstRune))
	if err != nil {
		log.Fatal(err)
	}

	dst := image.NewRGBA(image.Rect(0, 0, 800, 100))
	draw.Draw(dst, dst.Bounds(), image.Black, image.Point{}, draw.Src)

	d := &font.Drawer{
		Dst:  dst,
		Src:  image.White,
		Face: face,
		Dot: fixed.Point26_6{
			X: 20 << 6,
			Y: 80 << 6,
		},
	}
	dot0 := pt(d.Dot)
	d.DrawString("The quick brown fox jumps over the lazy dog.")
	dot1 := pt(d.Dot)

	dst.SetRGBA(dot0.X, dot0.Y, color.RGBA{0xff, 0x00, 0x00, 0xff})
	dst.SetRGBA(dot1.X, dot1.Y, color.RGBA{0x00, 0x00, 0xff, 0xff})

	out, err := os.Create("out.png")
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()
	if err := png.Encode(out, dst); err != nil {
		log.Fatal(err)
	}
}
