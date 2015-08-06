// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin
// +build 386 amd64
// +build !ios

package gldriver

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework OpenGL -framework QuartzCore
#import <Cocoa/Cocoa.h>
#include <pthread.h>
#include "cocoa.h"
*/
import "C"

import (
	"log"
	"runtime"

	"golang.org/x/exp/shiny/screen"
	"golang.org/x/mobile/event/config"
	"golang.org/x/mobile/event/lifecycle"
	"golang.org/x/mobile/event/paint"
	"golang.org/x/mobile/geom"
	"golang.org/x/mobile/gl"
)

var initThreadID C.uint64_t

func init() {
	// Lock the goroutine responsible for initialization to an OS thread.
	// This means the goroutine running main (and calling startDriver below)
	// is locked to the OS thread that started the program. This is
	// necessary for the correct delivery of Cocoa events to the process.
	//
	// A discussion on this topic:
	// https://groups.google.com/forum/#!msg/golang-nuts/IiWZ2hUuLDA/SNKYYZBelsYJ
	runtime.LockOSThread()
	initThreadID = C.threadID()
}

var (
	theScreen = &screenImpl{
		windowCreated: make(chan *windowImpl),
		draw:          make(chan *windowImpl),
		drawDone:      make(chan struct{}),
		windows:       make(map[uintptr]*windowImpl),
	}
	mainCallback func(screen.Screen)
)

func main(f func(screen.Screen)) error {
	if tid := C.threadID(); tid != initThreadID {
		log.Fatalf("gldriver.Main called on thread %d, but gldriver.init ran on %d", tid, initThreadID)
	}

	mainCallback = f
	C.startDriver()
	return nil
}

//export driverStarted
func driverStarted() {
	go drawLoop()
	go func() {
		mainCallback(theScreen)
		C.stopDriver()
	}()
}

//export drawgl
func drawgl(id uintptr) {
	theScreen.mu.Lock()
	w := theScreen.windows[id]
	theScreen.mu.Unlock()

	theScreen.draw <- w
	<-theScreen.drawDone
}

// drawLoop is the primary drawing loop.
//
// After Cocoa has created an NSWindow on the initial OS thread for
// processing Cocoa events in newWindow, it starts drawLoop on another
// goroutine. It is locked to an OS thread for its OpenGL context.
//
// Two Cocoa threads deliver draw signals to drawLoop. The primary
// source of draw events is the CVDisplayLink timer, which is tied to
// the display vsync. Secondary draw events come from [NSView drawRect:]
// when the window is resized.
func drawLoop() {
	runtime.LockOSThread()

	w := <-theScreen.windowCreated
	C.makeCurrentContext(C.uintptr_t(w.glctx))

	// TODO(crawshaw): exit this goroutine on Release.
	for {
		select {
		case <-gl.WorkAvailable:
			gl.DoWork()
		case w := <-theScreen.draw:
			C.makeCurrentContext(C.uintptr_t(w.glctx))
			w.eventsIn <- paint.Event{}
		loop:
			for {
				select {
				case <-gl.WorkAvailable:
					gl.DoWork()
				case <-w.endPaint:
					C.CGLFlushDrawable(C.CGLGetCurrentContext())
					break loop
				}
			}
			theScreen.drawDone <- struct{}{}
		}
	}
}

//export setGeom
func setGeom(id uintptr, ppp float32, widthPx, heightPx int) {
	theScreen.mu.Lock()
	w := theScreen.windows[id]
	theScreen.mu.Unlock()

	cfg := config.Event{
		WidthPx:     widthPx,
		HeightPx:    heightPx,
		WidthPt:     geom.Pt(float32(widthPx) / ppp),
		HeightPt:    geom.Pt(float32(heightPx) / ppp),
		PixelsPerPt: ppp,
	}

	w.mu.Lock()
	w.cfg = cfg
	w.mu.Unlock()

	w.eventsIn <- cfg
}

//export eventMouseDown
func eventMouseDown(id uintptr, x, y float32) { log.Printf("eventMouseDown") } // TODO

//export eventMouseDragged
func eventMouseDragged(id uintptr, x, y float32) { log.Printf("eventMouseDragged") } // TODO

//export eventMouseEnd
func eventMouseEnd(id uintptr, x, y float32) { log.Printf("eventMouseEnd") } // TODO

func sendLifecycle(to lifecycle.Stage) {
	log.Printf("sendLifecycle: %v", to) // TODO
}

//export lifecycleDead
func lifecycleDead() { sendLifecycle(lifecycle.StageDead) }

//export lifecycleAlive
func lifecycleAlive() { sendLifecycle(lifecycle.StageAlive) }

//export lifecycleVisible
func lifecycleVisible() { sendLifecycle(lifecycle.StageVisible) }

//export lifecycleFocused
func lifecycleFocused() { sendLifecycle(lifecycle.StageFocused) }
