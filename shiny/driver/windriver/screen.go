// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build windows

package windriver

import (
	"fmt"
	"image"
	"syscall"
	"unsafe"

	"golang.org/x/exp/shiny/screen"
)

// The Screen window encapsulates all screen.Screen operations
// in an actual Windows window so they all run on the main thread.
// Since any messages sent to a window will be executed on the
// main thread, we can safely use the messages below.
var screenhwnd syscall.Handle

const (
	// wParam - pointer to window options
	// lParam - pointer to *screen.Window
	// lResult - pointer to error
	msgCreateWindow = xWM_USER + iota
)

type screenimpl struct{}

func newScreenImpl() screen.Screen {
	return &screenimpl{}
}

func (*screenimpl) NewBuffer(size image.Point) (screen.Buffer, error) {
	return nil, fmt.Errorf("TODO")
}

func (*screenimpl) NewTexture(size image.Point) (screen.Texture, error) {
	return nil, fmt.Errorf("TODO")
}

func (*screenimpl) NewWindow(opts *screen.NewWindowOptions) (screen.Window, error) {
	var w screen.Window
	perr := sendMessage(screenhwnd, msgCreateWindow,
		uintptr(unsafe.Pointer(opts)),
		uintptr(unsafe.Pointer(&w)))
	// TODO this part probably isn't safe
	err := *(*error)(unsafe.Pointer(perr))
	if err != nil {
		return nil, err
	}
	return w, nil
}

func screenWindowWndProc(hwnd syscall.Handle, uMsg uint32, wParam uintptr, lParam uintptr) (lResult uintptr) {
	switch uMsg {
	case msgCreateWindow:
		opts := (*screen.NewWindowOptions)(unsafe.Pointer(wParam))
		pw := (*screen.Window)(unsafe.Pointer(lParam))
		w, err := newWindow(opts)
		*pw = w
		return uintptr(unsafe.Pointer(&err))
	}
	return defWindowProc(hwnd, uMsg, wParam, lParam)
}

const screenWindowClass = "shiny_ScreenWindow"

func initScreenWindow() (err error) {
	swc, err := syscall.UTF16PtrFromString(screenWindowClass)
	if err != nil {
		return err
	}
	emptyString, err := syscall.UTF16PtrFromString("")
	if err != nil {
		return err
	}
	wc := wndclass{
		LpszClassName: swc,
		LpfnWndProc:   syscall.NewCallback(screenWindowWndProc),
		HIcon:         hDefaultIcon,
		HCursor:       hDefaultCursor,
		HInstance:     hThisInstance,
		HbrBackground: syscall.Handle(xCOLOR_BTNFACE + 1),
	}
	_, err = registerClass(&wc)
	if err != nil {
		return err
	}
	screenhwnd, err = createWindowEx(0,
		swc, emptyString,
		xWS_OVERLAPPEDWINDOW,
		xCW_USEDEFAULT, xCW_USEDEFAULT,
		xCW_USEDEFAULT, xCW_USEDEFAULT,
		xHWND_MESSAGE, 0, hThisInstance, 0)
	if err != nil {
		return err
	}
	return nil
}
