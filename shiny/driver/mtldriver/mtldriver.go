// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin

// Package mtldriver provides a Metal driver for accessing a screen.
//
// At this time, the Metal API is used only to present the final pixels
// to the screen. All rendering is performed on the CPU via the image/draw
// algorithms. Future work is to use mtl.Buffer, mtl.Texture, etc., and
// do more of the rendering work on the GPU.
package mtldriver

import (
	"runtime"

	"dmitri.shuralyov.com/gpu/mtl"
	"github.com/go-gl/glfw/v3.2/glfw"
	"golang.org/x/exp/shiny/driver/internal/errscreen"
	"golang.org/x/exp/shiny/screen"
)

func init() {
	runtime.LockOSThread()
}

// Main is called by the program's main function to run the graphical
// application.
//
// It calls f on the Screen, possibly in a separate goroutine, as some OS-
// specific libraries require being on 'the main thread'. It returns when f
// returns.
func Main(f func(screen.Screen)) {
	if err := main(f); err != nil {
		f(errscreen.Stub(err))
	}
}

func main(f func(screen.Screen)) error {
	device, err := mtl.CreateSystemDefaultDevice()
	if err != nil {
		return err
	}
	err = glfw.Init()
	if err != nil {
		return err
	}
	defer glfw.Terminate()
	f(&screenImpl{device: device})
	return nil
}
