// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build windows

package gldriver

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/exp/shiny/driver/internal/win32"
	"golang.org/x/exp/shiny/screen"
	"golang.org/x/mobile/event/lifecycle"
	"golang.org/x/mobile/event/mouse"
	"golang.org/x/mobile/event/paint"
	"golang.org/x/mobile/event/size"
	"golang.org/x/mobile/gl"
)

func main(f func(screen.Screen)) error {
	return win32.Main(func() { f(theScreen) })
}

func init() {
	err := libEGL.Load()
	if err == nil {
		return
	}

	// Look for libEGL.dll in GOPATH.
	gopath := os.Getenv("GOPATH")
	for _, seg := range filepath.SplitList(gopath) {
		path := filepath.Join(seg, "src/golang.org/x/exp/shiny/driver/gldriver")
		if _, err := os.Stat(path); err == nil {
			libEGL.Name = filepath.Join(path, "libEGL.dll")
			if err := libEGL.Load(); err == nil {
				return // done
			}
		}
	}

	panic(fmt.Sprintf("could not load libEGL.dll (also missing from GOPATH): %v", err))
}

var (
	libEGL = syscall.NewLazyDLL("libEGL.dll")

	eglGetPlatformDisplayEXT = libEGL.NewProc("eglGetPlatformDisplayEXT")
	eglInitialize            = libEGL.NewProc("eglInitialize")
	eglChooseConfig          = libEGL.NewProc("eglChooseConfig")
	eglGetError              = libEGL.NewProc("eglGetError")
	eglBindAPI               = libEGL.NewProc("eglBindAPI")
	eglGetConfigAttrib       = libEGL.NewProc("eglGetConfigAttrib")
	eglCreateWindowSurface   = libEGL.NewProc("eglCreateWindowSurface")
	eglCreateContext         = libEGL.NewProc("eglCreateContext")
	eglMakeCurrent           = libEGL.NewProc("eglMakeCurrent")
	eglSwapInterval          = libEGL.NewProc("eglSwapInterval")
	eglDestroySurface        = libEGL.NewProc("eglDestroySurface")
	eglSwapBuffers           = libEGL.NewProc("eglSwapBuffers")
)

type eglConfig uintptr // void*

type eglInt int32

var rgb888 = [...]int{
	_EGL_RENDERABLE_TYPE, _EGL_OPENGL_ES2_BIT,
	_EGL_SURFACE_TYPE, _EGL_WINDOW_BIT,
	_EGL_BLUE_SIZE, 8,
	_EGL_GREEN_SIZE, 8,
	_EGL_RED_SIZE, 8,
	_EGL_DEPTH_SIZE, 24,
	_EGL_STENCIL_SIZE, 8,
	_EGL_NONE,
}

type ctxWin32 struct {
	ctx     uintptr
	display uintptr // EGLDisplay
	surface uintptr // EGLSurface
}

func newWindow(opts *screen.NewWindowOptions) (uintptr, error) {
	w, err := win32.NewWindow(opts)
	if err != nil {
		return 0, err
	}
	return uintptr(w), nil
}

func showWindow(w *windowImpl) {
	w.glctxMu.Lock()
	w.glctx, w.worker = gl.NewContext()
	w.glctxMu.Unlock()

	// Show makes an initial call to sizeEvent (via win32.SizeEvent), where
	// we setup the EGL surface and GL context.
	win32.Show(win32.HWND(w.id))
}

func closeWindow(id uintptr) {} // TODO

func drawLoop(w *windowImpl) {
	runtime.LockOSThread()

	display := w.ctx.(ctxWin32).display
	surface := w.ctx.(ctxWin32).surface
	ctx := w.ctx.(ctxWin32).ctx

	if ret, _, _ := eglMakeCurrent.Call(display, surface, surface, ctx); ret == 0 {
		panic(fmt.Sprintf("eglMakeCurrent failed: %v", eglErr()))
	}

	// TODO(crawshaw): exit this goroutine on Release.
	workAvailable := w.worker.WorkAvailable()
	for {
		select {
		case <-workAvailable:
			w.worker.DoWork()
		case <-w.publish:
		loop:
			for {
				select {
				case <-workAvailable:
					w.worker.DoWork()
				default:
					break loop
				}
			}
			if ret, _, _ := eglSwapBuffers.Call(display, surface); ret == 0 {
				panic(fmt.Sprintf("eglSwapBuffers failed: %v", eglErr()))
			}
			select {
			case w.drawDone <- struct{}{}:
			default:
			}
		}
	}
}

func init() {
	win32.SizeEvent = sizeEvent
	win32.PaintEvent = paintEvent
	win32.MouseEvent = mouseEvent
	win32.LifecycleEvent = lifecycleEvent
}

func lifecycleEvent(hwnd win32.HWND, to lifecycle.Stage) {
	theScreen.mu.Lock()
	w := theScreen.windows[uintptr(hwnd)]
	theScreen.mu.Unlock()

	if w.lifecycleStage == to {
		return
	}
	w.Send(lifecycle.Event{
		From:        w.lifecycleStage,
		To:          to,
		DrawContext: w.glctx,
	})
	w.lifecycleStage = to
}

func mouseEvent(hwnd win32.HWND, e mouse.Event) {
	theScreen.mu.Lock()
	w := theScreen.windows[uintptr(hwnd)]
	theScreen.mu.Unlock()

	w.Send(e)
}

func paintEvent(hwnd win32.HWND, e paint.Event) {
	theScreen.mu.Lock()
	w := theScreen.windows[uintptr(hwnd)]
	theScreen.mu.Unlock()

	if w.ctx == nil {
		// Sometimes a paint event comes in before initial
		// window size is set. Ignore it.
		return
	}

	w.Send(paint.Event{})
}

func sizeEvent(hwnd win32.HWND, e size.Event) {
	theScreen.mu.Lock()
	w := theScreen.windows[uintptr(hwnd)]
	theScreen.mu.Unlock()

	if w.ctx == nil {
		// This is the initial size event on window creation.
		// Create an EGL surface and spin up a GL context.
		if err := createEGLSurface(hwnd, w); err != nil {
			panic(err)
		}
	}

	w.szMu.Lock()
	w.sz = e
	w.szMu.Unlock()

	w.Send(e)

	// Screen is dirty, generate a paint event.
	//
	// GL calls on a separate goroutine to avoid a work queue
	// deadlock when compiling with -tags gldebug.
	go func() {
		// TODO this call to Viewport is not right, but is very hard to
		// do correctly with ur async events channel model. We want the
		// call to Viewport to be made the instant before the
		// paint.Event is receieved.
		w.glctxMu.Lock()
		w.glctx.Viewport(0, 0, e.WidthPx, e.HeightPx)
		w.glctx.ClearColor(0, 0, 0, 1)
		w.glctx.Clear(gl.COLOR_BUFFER_BIT)
		w.glctxMu.Unlock()

		w.Send(paint.Event{})
	}()
}

func eglErr() error {
	if ret, _, _ := eglGetError.Call(); ret != _EGL_SUCCESS {
		return errors.New(eglErrString(ret))
	}
	return nil
}

func createEGLSurface(hwnd win32.HWND, w *windowImpl) error {
	fmt.Printf("createEGLSurface called\n")
	var display uintptr

	displayAttrib := [...]eglInt{
		_EGL_PLATFORM_ANGLE_TYPE_ANGLE,
		_EGL_PLATFORM_ANGLE_TYPE_DEFAULT_ANGLE,
		_EGL_PLATFORM_ANGLE_MAX_VERSION_MAJOR_ANGLE, _EGL_DONT_CARE,
		_EGL_PLATFORM_ANGLE_MAX_VERSION_MINOR_ANGLE, _EGL_DONT_CARE,
		_EGL_NONE,
	}

	dc, err := win32.GetDC(hwnd)
	if err != nil {
		return fmt.Errorf("win32.GetDC failed: %v", err)
	}
	display, _, _ = eglGetPlatformDisplayEXT.Call(
		_EGL_PLATFORM_ANGLE_ANGLE,
		uintptr(dc),
		uintptr(unsafe.Pointer(&displayAttrib)),
	)
	if display == _EGL_NO_DISPLAY {
		return fmt.Errorf("eglGetPlatformDisplayEXT failed: %v", eglErr())
	}
	if ret, _, _ := eglInitialize.Call(display, 0, 0); ret == 0 {
		return fmt.Errorf("eglInitialize failed: %v", eglErr())
	}

	eglBindAPI.Call(_EGL_OPENGL_ES_API)
	if err := eglErr(); err != nil {
		return err
	}

	var numConfigs eglInt
	var config eglConfig
	ret, _, _ := eglChooseConfig.Call(
		display,
		uintptr(unsafe.Pointer(&rgb888[0])),
		uintptr(unsafe.Pointer(&config)),
		1,
		uintptr(unsafe.Pointer(&numConfigs)),
	)
	if ret == 0 {
		return fmt.Errorf("eglChooseConfig failed: %v", eglErr())
	}
	if numConfigs <= 0 {
		return errors.New("eglChooseConfig found no valid config")
	}

	const _EGL_POST_SUB_BUFFER_SUPPORTED_NV = 0x30BE
	surfaceAttrib := [...]eglInt{
		_EGL_POST_SUB_BUFFER_SUPPORTED_NV, 1,
		_EGL_NONE,
	}
	surface, _, _ := eglCreateWindowSurface.Call(
		display,
		uintptr(config),
		uintptr(hwnd),
		0,
		uintptr(unsafe.Pointer(&surfaceAttrib[0])),
	)
	if surface == _EGL_NO_SURFACE {
		return fmt.Errorf("eglCreateWindowSurface failed: %v", eglErr())
	}

	contextAttribs := [...]eglInt{
		_EGL_CONTEXT_CLIENT_VERSION, 2, _EGL_NONE,
	}
	context, _, _ := eglCreateContext.Call(
		display,
		uintptr(config),
		_EGL_NO_CONTEXT,
		uintptr(unsafe.Pointer(&contextAttribs[0])),
	)
	if context == _EGL_NO_CONTEXT {
		return fmt.Errorf("eglCreateContext failed: %v", eglErr())
	}

	eglSwapInterval.Call(display, 1)

	w.ctx = ctxWin32{
		ctx:     context,
		display: display,
		surface: surface,
	}

	return nil
}
