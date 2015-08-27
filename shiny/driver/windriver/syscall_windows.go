// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run $GOROOT/src/syscall/mksyscall_windows.go -output zsyscall_windows.go syscall_windows.go

package windriver

import "syscall"

type point struct {
	X int32
	Y int32
}

type msg struct {
	Hwnd    syscall.Handle
	Message uint32
	Wparam  uintptr
	Lparam  uintptr
	Time    uint32
	Pt      point
}

type wndclass struct {
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     syscall.Handle
	HIcon         syscall.Handle
	HCursor       syscall.Handle
	HbrBackground syscall.Handle
	LpszMenuName  *uint16
	LpszClassName *uint16
}

const (
	xWM_USER = 0x0400
)

const (
	xWS_OVERLAPPED       = 0x00000000
	xWS_CAPTION          = 0x00C00000
	xWS_SYSMENU          = 0x00080000
	xWS_THICKFRAME       = 0x00040000
	xWS_MINIMIZEBOX      = 0x00020000
	xWS_MAXIMIZEBOX      = 0x00010000
	xWS_OVERLAPPEDWINDOW = xWS_OVERLAPPED | xWS_CAPTION | xWS_SYSMENU | xWS_THICKFRAME | xWS_MINIMIZEBOX | xWS_MAXIMIZEBOX
)

const (
	xCOLOR_BTNFACE = 15
)

const (
	xIDI_APPLICATION = 32512
	xIDC_ARROW       = 32512
)

var (
	// TODO(andlabs): verify these as correct for 64-bit Windows
	baseCW_USEDEFAULT uint32 = 0x80000000
	baseHWND_MESSAGE  int32  = -3

	xCW_USEDEFAULT = int32(baseCW_USEDEFAULT)
	xHWND_MESSAGE  = syscall.Handle(baseHWND_MESSAGE)
)

// notes to self
// UINT = uint32
// callbacks = uintptr
// strings = *uint16

//sys	getMessage(msg *msg, hwnd syscall.Handle, msgfiltermin uint32, msgfiltermax uint32) (ret int32, err error) [failretval==-1] = user32.GetMessageW
//sys	translateMessage(msg *msg) (done bool) = user32.TranslateMessage
//sys	dispatchMessage(msg *msg) (ret int32) = user32.DispatchMessageW
//sys	defWindowProc(hwnd syscall.Handle, uMsg uint32, wParam uintptr, lParam uintptr) (lResult uintptr) = user32.DefWindowProcW
//sys	registerClass(wc *wndclass) (atom uint16, err error) [failretval==0] = user32.RegisterClassW
//sys	createWindowEx(exstyle uint32, className *uint16, windowText *uint16, style uint32, x int32, y int32, width int32, height int32, parent syscall.Handle, menu syscall.Handle, hInstance syscall.Handle, lpParam uintptr) (hwnd syscall.Handle, err error) [failretval==0] = user32.CreateWindowExW
//sys	destroyWindow(hwnd syscall.Handle) (err error) [failretval==0] = user32.DestroyWindow
//sys	sendMessage(hwnd syscall.Handle, uMsg uint32, wParam uintptr, lParam uintptr) (lResult uintptr) = user32.SendMessageW
//sys	loadIcon(hInstance syscall.Handle, resource uintptr) (icon syscall.Handle, err error) [failretval==0] = user32.LoadIconW
//sys	loadCursor(hInstance syscall.Handle, resource uintptr) (cursor syscall.Handle, err error) [failretval==0] = user32.LoadCursorW
