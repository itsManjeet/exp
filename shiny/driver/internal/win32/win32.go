// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build windows

package win32

import (
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/exp/shiny/screen"
	"golang.org/x/mobile/event/key"
	"golang.org/x/mobile/event/mouse"
	"golang.org/x/mobile/event/paint"
	"golang.org/x/mobile/event/size"
	"golang.org/x/mobile/geom"
)

// screenHWND is the handle to the "Screen window".
// The Screen window encapsulates all screen.Screen operations
// in an actual Windows window so they all run on the main thread.
// Since any messages sent to a window will be executed on the
// main thread, we can safely use the messages below.
var screenHWND HWND

const (
	msgCreateWindow = _WM_USER + iota
	msgInitialSize
	msgLast
)

var nextWM uint32 = msgLast

func newWindow(opts *screen.NewWindowOptions) (HWND, error) {
	// TODO(brainman): convert windowClass to *uint16 once (in initWindowClass)
	wcname, err := syscall.UTF16PtrFromString(windowClass)
	if err != nil {
		return 0, err
	}
	title, err := syscall.UTF16PtrFromString("Shiny Window")
	if err != nil {
		return 0, err
	}
	h, err := _CreateWindowEx(0,
		wcname, title,
		_WS_OVERLAPPEDWINDOW,
		_CW_USEDEFAULT, _CW_USEDEFAULT,
		_CW_USEDEFAULT, _CW_USEDEFAULT,
		0, 0, hThisInstance, 0)
	if err != nil {
		return 0, err
	}
	// TODO(andlabs): use proper nCmdShow
	_ShowWindow(h, _SW_SHOWDEFAULT)
	// TODO(andlabs): call UpdateWindow()

	return h, nil
}

func Show(hwnd HWND) {
	// Send a fake size event.
	// Windows won't generate the WM_WINDOWPOSCHANGED
	// we trigger a resize on for the initial size, so we have to do
	// it ourselves. The example/basic program assumes it will
	// receive a size.Event for the initial window size that isn't 0x0.
	SendMessage(hwnd, msgInitialSize, 0, 0)
}

func Release(hwnd HWND) {
	// TODO(andlabs): check for errors from this?
	// TODO(andlabs): remove unsafe
	_DestroyWindow(hwnd)
	// TODO(andlabs): what happens if we're still painting?
}

func sendSizeEvent(hwnd HWND, uMsg uint32, wParam, lParam uintptr) bool {
	if uMsg != msgInitialSize {
		wp := (*_WINDOWPOS)(unsafe.Pointer(lParam))
		if wp.Flags&_SWP_NOSIZE != 0 {
			return false
		}
	}
	var r _RECT
	if err := _GetClientRect(hwnd, &r); err != nil {
		panic(err) // TODO(andlabs)
	}

	width := int(r.Right - r.Left)
	height := int(r.Bottom - r.Top)

	// TODO(andlabs): don't assume that PixelsPerPt == 1
	SizeEvent(hwnd, size.Event{
		WidthPx:     width,
		HeightPx:    height,
		WidthPt:     geom.Pt(width),
		HeightPt:    geom.Pt(height),
		PixelsPerPt: 1,
	})
	return false
}

func sendMouseEvent(hwnd HWND, uMsg uint32, wParam, lParam uintptr) bool {
	x := _GET_X_LPARAM(lParam)
	y := _GET_Y_LPARAM(lParam)
	var dir mouse.Direction
	var button mouse.Button

	switch uMsg {
	case _WM_MOUSEMOVE:
		dir = mouse.DirNone
	case _WM_LBUTTONDOWN, _WM_MBUTTONDOWN, _WM_RBUTTONDOWN:
		dir = mouse.DirPress
	case _WM_LBUTTONUP, _WM_MBUTTONUP, _WM_RBUTTONUP:
		dir = mouse.DirRelease
	default:
		panic("sendMouseEvent() called on non-mouse message")
	}

	switch uMsg {
	case _WM_MOUSEMOVE:
		button = mouse.ButtonNone
	case _WM_LBUTTONDOWN, _WM_LBUTTONUP:
		button = mouse.ButtonLeft
	case _WM_MBUTTONDOWN, _WM_MBUTTONUP:
		button = mouse.ButtonMiddle
	case _WM_RBUTTONDOWN, _WM_RBUTTONUP:
		button = mouse.ButtonRight
	}
	// TODO(andlabs): mouse wheel

	MouseEvent(hwnd, mouse.Event{
		X:         float32(x),
		Y:         float32(y),
		Button:    button,
		Modifiers: keyModifiers(),
		Direction: dir,
	})

	return false
}

// Precondition: this is called in immediate response to the message that triggered the event (so not after w.Send).
func keyModifiers() (m key.Modifiers) {
	down := func(x int32) bool {
		// GetKeyState gets the key state at the time of the message, so this is what we want.
		return _GetKeyState(x)&0x80 != 0
	}

	if down(_VK_CONTROL) {
		m |= key.ModControl
	}
	if down(_VK_MENU) {
		m |= key.ModAlt
	}
	if down(_VK_SHIFT) {
		m |= key.ModShift
	}
	if down(_VK_LWIN) || down(_VK_RWIN) {
		m |= key.ModMeta
	}
	return m
}

var (
	MouseEvent func(hwnd HWND, e mouse.Event)
	PaintEvent func(hwnd HWND, e paint.Event)
	SizeEvent  func(hwnd HWND, e size.Event)
)

func sendPaint(hwnd HWND, uMsg uint32, wParam, lParam uintptr) bool {
	PaintEvent(hwnd, paint.Event{})
	return true // defer to DefWindowProc; it will handle validation for us
}

func screenWindowWndProc(hwnd HWND, uMsg uint32, wParam uintptr, lParam uintptr) (lResult uintptr) {
	switch uMsg {
	case msgCreateWindow:
		p := (*newWindowParams)(unsafe.Pointer(lParam))
		p.w, p.err = newWindow(p.opts)
		return 0
	}
	return _DefWindowProc(hwnd, uMsg, wParam, lParam)
}

var windowMsgs = map[uint32]func(hwnd HWND, uMsg uint32, wParam, lParam uintptr) bool{
	_WM_PAINT:            sendPaint,
	msgInitialSize:       sendSizeEvent,
	_WM_WINDOWPOSCHANGED: sendSizeEvent,
	_WM_LBUTTONUP:        sendMouseEvent,
	_WM_MBUTTONDOWN:      sendMouseEvent,
	_WM_MBUTTONUP:        sendMouseEvent,
	_WM_RBUTTONDOWN:      sendMouseEvent,
	_WM_RBUTTONUP:        sendMouseEvent,
	// TODO case _WM_MOUSEMOVE, _WM_LBUTTONDOWN: call SetFocus()?
	// TODO case _WM_KEYDOWN, _WM_KEYUP, _WM_SYSKEYDOWN, _WM_SYSKEYUP:
}

func AddWindowMsg(fn func(hwnd HWND, uMsg uint32, wParam, lParam uintptr) bool) uint32 {
	uMsg := nextWM
	nextWM++
	windowMsgs[uMsg] = fn
	return uMsg
}

func windowWndProc(hwnd HWND, uMsg uint32, wParam uintptr, lParam uintptr) (lResult uintptr) {
	runtime.LockOSThread()
	fn := windowMsgs[uMsg]
	if fn != nil {
		if !fn(hwnd, uMsg, wParam, lParam) {
			return 0
		}
	}
	return _DefWindowProc(hwnd, uMsg, wParam, lParam)
}

type newWindowParams struct {
	opts *screen.NewWindowOptions
	w    HWND
	err  error
}

func NewWindow(opts *screen.NewWindowOptions) (HWND, error) {
	var p newWindowParams
	p.opts = opts
	SendMessage(screenHWND, msgCreateWindow, 0, uintptr(unsafe.Pointer(&p)))
	return p.w, p.err
}

const windowClass = "shiny_Window"

func initWindowClass() (err error) {
	wcname, err := syscall.UTF16PtrFromString(windowClass)
	if err != nil {
		return err
	}
	_, err = _RegisterClass(&_WNDCLASS{
		LpszClassName: wcname,
		LpfnWndProc:   syscall.NewCallback(windowWndProc),
		HIcon:         hDefaultIcon,
		HCursor:       hDefaultCursor,
		HInstance:     hThisInstance,
		// TODO(andlabs): change this to something else? NULL? the hollow brush?
		HbrBackground: syscall.Handle(_COLOR_BTNFACE + 1),
	})
	return err
}

func initScreenWindow() (err error) {
	const screenWindowClass = "shiny_ScreenWindow"
	swc, err := syscall.UTF16PtrFromString(screenWindowClass)
	if err != nil {
		return err
	}
	emptyString, err := syscall.UTF16PtrFromString("")
	if err != nil {
		return err
	}
	wc := _WNDCLASS{
		LpszClassName: swc,
		LpfnWndProc:   syscall.NewCallback(screenWindowWndProc),
		HIcon:         hDefaultIcon,
		HCursor:       hDefaultCursor,
		HInstance:     hThisInstance,
		HbrBackground: syscall.Handle(_COLOR_BTNFACE + 1),
	}
	_, err = _RegisterClass(&wc)
	if err != nil {
		return err
	}
	screenHWND, err = _CreateWindowEx(0,
		swc, emptyString,
		_WS_OVERLAPPEDWINDOW,
		_CW_USEDEFAULT, _CW_USEDEFAULT,
		_CW_USEDEFAULT, _CW_USEDEFAULT,
		_HWND_MESSAGE, 0, hThisInstance, 0)
	if err != nil {
		return err
	}
	return nil
}

var (
	hDefaultIcon   syscall.Handle
	hDefaultCursor syscall.Handle
	hThisInstance  syscall.Handle
)

func initCommon() (err error) {
	hDefaultIcon, err = _LoadIcon(0, _IDI_APPLICATION)
	if err != nil {
		return err
	}
	hDefaultCursor, err = _LoadCursor(0, _IDC_ARROW)
	if err != nil {
		return err
	}
	// TODO(andlabs) hThisInstance
	return nil
}

func mainMessagePump() {
	var m _MSG
	for {
		done, err := _GetMessage(&m, 0, 0, 0)
		if err != nil {
			panic(err)
		}
		if done == 0 { // WM_QUIT
			return
		}
		_TranslateMessage(&m)
		_DispatchMessage(&m)
	}
}

func Main(f func()) (retErr error) {
	// It does not matter which OS thread we are on.
	// All that matters is that we confine all UI operations
	// to the thread that created the respective window.
	runtime.LockOSThread()

	if err := initCommon(); err != nil {
		return err
	}

	if err := initScreenWindow(); err != nil {
		return err
	}
	defer func() {
		// TODO(andlabs): log an error if this fails?
		_DestroyWindow(screenHWND)
		// TODO(andlabs): unregister window class
	}()

	if err := initWindowClass(); err != nil {
		return err
	}
	// TODO(andlabs): uninit

	go f()
	mainMessagePump()
	return nil
}
