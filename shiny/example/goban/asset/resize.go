// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

// Custom image resizer. Saved for posterity.

package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"

	"golang.org/x/image/draw"
)

var n = flag.Int("n", 256, "size in pixels")

func main() {
	flag.Parse()
	fmt.Println(flag.Args())
	for _, s := range flag.Args() {
		resize(s)
	}
}

func resize(s string) {
	in, err := os.Open(s)
	ck(err)
	defer in.Close()
	im, _, err := image.Decode(in)
	ck(err)
	res := resizeGray(im.(*image.Gray))
	name := fmt.Sprintf("../%s", s)
	out, err := os.Create(name)
	fmt.Println(name)
	ck(err)
	defer out.Close()
	final := image.NewGray(image.Rect(0, 0, 256, 256))
	draw.Draw(final, final.Bounds(), image.White, image.ZP, draw.Src)
	pt := image.Point{(*n - 256) / 2, (*n - 256) / 2}
	draw.Draw(final, final.Bounds(), res, pt, draw.Src)
	png.Encode(out, final)
}

func resizeGray(src *image.Gray) *image.Gray {
	dst := image.NewGray(image.Rect(0, 0, *n, *n))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
	return dst
}

func ck(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
