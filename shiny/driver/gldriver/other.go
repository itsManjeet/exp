// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !darwin !386,!amd64 ios
// +build !linux android
// +build !windows

package gldriver

import (
	"fmt"
	"runtime"

	"golang.org/x/exp/shiny/screen"
)

type winID int

func newWindow(width, height int32) winID { return 0 }
func showWindow(id *windowImpl)           {}
func closeWindow(id winID)                {}
func drawLoop(w *windowImpl)              {}

func main(f func(screen.Screen)) error {
	return fmt.Errorf("gldriver: unsupported GOOS/GOARCH %s/%s", runtime.GOOS, runtime.GOARCH)
}
