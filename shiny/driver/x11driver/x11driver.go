// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package x11driver provides the X11 driver for accessing a screen.
package x11driver

// TODO: figure out what to say about the responsibility for users of this
// package to check any implicit dependencies' LICENSEs. For example, the
// driver might use third party software outside of golang.org/x, like an X11
// or OpenGL library.

import (
	"errors"
	"image"

	"golang.org/x/exp/shiny/screen"
)

// Main is called by the program's main function to run the graphical
// application.
//
// It calls f on the Screen, possibly in a separate goroutine, as some OS-
// specific libraries require being on 'the main thread'. It returns when f
// returns.
func Main(f func(screen.Screen)) {
	f(stub{})
}

type stub struct{}

func (stub) NewBuffer(size image.Point) (screen.Buffer, error) {
	return nil, errTODO
}

func (stub) NewTexture(size image.Point) (screen.Texture, error) {
	return nil, errTODO
}

func (stub) NewWindow(opts *screen.NewWindowOptions) (screen.Window, error) {
	return nil, errTODO
}

var errTODO = errors.New("TODO: write the X11 driver")
