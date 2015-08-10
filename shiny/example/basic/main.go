// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore
//
// This build tag means that "go install golang.org/x/exp/shiny/..." doesn't
// install this example program. Use "go run main.go" to run it.

// Basic is a basic example of a graphical application.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"

	"golang.org/x/exp/shiny/driver"
	"golang.org/x/exp/shiny/screen"
	"golang.org/x/image/math/f64"
	"golang.org/x/mobile/event/config"
	"golang.org/x/mobile/event/paint"
)

func main() {
	driver.Main(func(s screen.Screen) {
		w, err := s.NewWindow(nil)
		if err != nil {
			log.Fatal(err)
		}
		defer w.Release()

		size := image.Point{256, 256}
		b, err := s.NewBuffer(size)
		if err != nil {
			log.Fatal(err)
		}
		defer b.Release()
		drawGradient(b.RGBA())

		t, err := s.NewTexture(size)
		if err != nil {
			log.Fatal(err)
		}
		defer t.Release()
		t.Upload(image.Point{}, b, b.Bounds(), w)

		var cfg config.Event
		for e := range w.Events() {
			switch e := e.(type) {
			default:
				// TODO: be more interesting.
				fmt.Printf("got event %#v\n", e)

			case config.Event:
				cfg = e

			case paint.Event:
				wBounds := image.Rectangle{Max: image.Point{cfg.WidthPx, cfg.HeightPx}}
				w.Fill(wBounds, color.RGBA{0x00, 0x00, 0x3f, 0xff}, draw.Src)
				w.Upload(image.Point{}, b, b.Bounds(), w)
				w.Draw(f64.Aff3{
					1, 0, 100,
					0, 1, 200,
				}, t, t.Bounds(), screen.Over, nil)
				w.EndPaint(e)

			case screen.UploadedEvent:
				// No-op.

			case error:
				log.Print(e)
			}
		}
	})
}

func drawGradient(m *image.RGBA) {
	b := m.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			m.SetRGBA(x, y, color.RGBA{uint8(x), uint8(y), 0x00, 0xff})
		}
	}
}
