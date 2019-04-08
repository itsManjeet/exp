// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin

package mtldriver

import (
	"image"
	"unsafe"

	"dmitri.shuralyov.com/gpu/mtl"
	"github.com/go-gl/glfw/v3.2/glfw"
	"golang.org/x/exp/shiny/driver/mtldriver/internal/ca"
	"golang.org/x/exp/shiny/driver/mtldriver/internal/ns"
	"golang.org/x/exp/shiny/screen"
	"golang.org/x/mobile/event/key"
	"golang.org/x/mobile/event/mouse"
)

// screenImpl implements screen.Screen.
type screenImpl struct {
	device mtl.Device
}

func (*screenImpl) NewBuffer(size image.Point) (screen.Buffer, error) {
	return &bufferImpl{
		rgba: image.NewRGBA(image.Rectangle{Max: size}),
	}, nil
}

func (*screenImpl) NewTexture(size image.Point) (screen.Texture, error) {
	return &textureImpl{
		rgba: image.NewRGBA(image.Rectangle{Max: size}),
	}, nil
}

func (s *screenImpl) NewWindow(opts *screen.NewWindowOptions) (screen.Window, error) {
	glfw.WindowHint(glfw.ClientAPI, glfw.NoAPI)
	width, height := optsSize(opts)
	window, err := glfw.CreateWindow(width, height, opts.GetTitle(), nil, nil)
	if err != nil {
		return nil, err
	}

	ml := ca.MakeMetalLayer()
	ml.SetDevice(s.device)
	ml.SetPixelFormat(mtl.PixelFormatBGRA8UNorm)
	ml.SetMaximumDrawableCount(2)
	ml.SetDisplaySyncEnabled(true)
	cv := ns.NewWindow(unsafe.Pointer(window.GetCocoaWindow())).ContentView()
	cv.SetLayer(ml)
	cv.SetWantsLayer(true)

	w := &windowImpl{
		device:  s.device,
		window:  window,
		ml:      ml,
		cq:      s.device.MakeCommandQueue(),
		eventCh: make(chan interface{}, 8),
	}

	// Set callbacks.
	window.SetFramebufferSizeCallback(func(_ *glfw.Window, width, height int) {
		w.size = &image.Point{X: width, Y: height}
	})
	window.SetCursorPosCallback(func(_ *glfw.Window, x, y float64) {
		const scale = 2 // TODO: compute dynamically
		w.eventCh <- mouse.Event{X: float32(x * scale), Y: float32(y * scale)}
	})
	window.SetMouseButtonCallback(func(_ *glfw.Window, b glfw.MouseButton, a glfw.Action, mods glfw.ModifierKey) {
		btn, dir := glfwMouseButton(b), glfwMouseDirection(a)
		if btn == mouse.ButtonNone || dir == mouse.DirNone {
			return
		}
		const scale = 2 // TODO: compute dynamically
		x, y := window.GetCursorPos()
		w.eventCh <- mouse.Event{
			X: float32(x * scale), Y: float32(y * scale),
			Button:    btn,
			Direction: dir,
			// TODO: set Modifiers
		}
	})
	window.SetKeyCallback(func(_ *glfw.Window, k glfw.Key, _ int, a glfw.Action, mods glfw.ModifierKey) {
		// TODO: support more keys
		if k == glfw.KeyEscape && a != glfw.Release {
			w.eventCh <- key.Event{Code: key.CodeEscape}
		}
	})
	window.SetCloseCallback(func(*glfw.Window) {
		w.lifecycler.SetDead(true)
		w.lifecycler.SendEvent(w, nil)
	})

	width, height = window.GetFramebufferSize()
	w.size = &image.Point{X: width, Y: height}

	// TODO: more fine-grained tracking of whether window is visible and/or focused
	w.lifecycler.SetDead(false)
	w.lifecycler.SetVisible(true)
	w.lifecycler.SetFocused(true)
	w.lifecycler.SendEvent(w, nil)

	return w, nil
}

func optsSize(opts *screen.NewWindowOptions) (width, height int) {
	width, height = 1024/2, 768/2
	if opts != nil {
		if opts.Width > 0 {
			width = opts.Width
		}
		if opts.Height > 0 {
			height = opts.Height
		}
	}
	return width, height
}

func glfwMouseButton(button glfw.MouseButton) mouse.Button {
	switch button {
	case glfw.MouseButtonLeft:
		return mouse.ButtonLeft
	case glfw.MouseButtonRight:
		return mouse.ButtonRight
	case glfw.MouseButtonMiddle:
		return mouse.ButtonMiddle
	default:
		return mouse.ButtonNone
	}
}

func glfwMouseDirection(action glfw.Action) mouse.Direction {
	switch action {
	case glfw.Press:
		return mouse.DirPress
	case glfw.Release:
		return mouse.DirRelease
	default:
		return mouse.DirNone
	}
}
