// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore
//
// This build tag means that "go install golang.org/x/exp/shiny/..." doesn't
// install this example program. Use "go run main.go board.go xy.go" to run it.

// Basic is a basic example of a graphical application.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	stdDraw "image/draw"
	"math/rand"
	"time"

	"log"

	"golang.org/x/exp/shiny/driver"
	"golang.org/x/exp/shiny/screen"
	"golang.org/x/mobile/event/key"
	"golang.org/x/mobile/event/mouse"
	"golang.org/x/mobile/event/paint"
	"golang.org/x/mobile/event/size"
)

var dirty bool
var uploading bool

var scale = flag.Int("scale", 35, "`percent` to scale images (TODO: a poor design)")

func main() {
	flag.Parse()

	rand.Seed(int64(time.Now().Nanosecond()))
	board := NewBoard(9, *scale)

	driver.Main(func(s screen.Screen) {
		w, err := s.NewWindow(nil)
		if err != nil {
			log.Fatal(err)
		}
		defer w.Release()

		r := board.image.Bounds()
		winSize := image.Point{r.Dx(), r.Dy()}
		var b screen.Buffer
		defer func() {
			if b != nil {
				b.Release()
			}
		}()

		var sz size.Event

		for e := range w.Events() {
			// fmt.Printf("%T\n", e)
			switch e := e.(type) {
			default:
				// TODO: be more interesting.
				fmt.Printf("got event %#v\n", e)
				render(b.RGBA(), board)

			case mouse.Event:
				if e.Direction == mouse.DirRelease && e.Button != 0 {
					// Invert y.
					y := b.RGBA().Bounds().Dy() - int(e.Y)
					board.click(b.RGBA(), int(e.X), y, int(e.Button))
					dirty = true
				}

			case key.Event:
				if e.Rune >= 0 {
					fmt.Printf("key.Event: %q (%v), %v, %v\n", e.Rune, e.Code, e.Modifiers, e.Direction)
				} else {
					fmt.Printf("key.Event: (%v), %v, %v\n", e.Code, e.Modifiers, e.Direction)
				}
				if e.Code == key.CodeEscape {
					return
				}
				render(b.RGBA(), board)

			case paint.Event:
				//if dirty && !uploading {
				wBounds := image.Rectangle{Max: image.Point{sz.WidthPx, sz.HeightPx}}
				w.Fill(wBounds, color.RGBA{0x00, 0x00, 0x3f, 0xff}, stdDraw.Src)
				w.Upload(image.Point{0, 0}, b, b.Bounds(), w) // TODO: On Darwin always writes to 0,0, ignoring first arg.
				dirty = false
				uploading = true
				//}
				w.EndPaint(e)

			case screen.UploadedEvent:
				// No-op.
				uploading = false

			case size.Event:
				// TODO: Set board size.
				sz = e
				// fmt.Printf("%#v\n", e)
				if b != nil {
					b.Release()
				}
				winSize = image.Point{sz.WidthPx, sz.HeightPx}
				b, err = s.NewBuffer(winSize)
				if err != nil {
					log.Fatal(err)
				}
				render(b.RGBA(), board)

			case error:
				log.Print(e)
			}
		}
	})
}

func tf(x bool) byte {
	if x {
		return 'T'
	}
	return 'F'
}

func render(m *image.RGBA, board *Board) {
	board.Draw(m)
	dirty = true
}
