// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin
// +build 386 amd64

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

var initThreadID uint64

func init() {
	// Lock the goroutine responsible for initialization to an OS thread.
	// This means the goroutine running main (and calling runDriver below)
	// is locked to the OS thread that started the program. This is
	// necessary for the correct delivery of Cocoa events to the process.
	//
	// A discussion on this topic:
	// https://groups.google.com/forum/#!msg/golang-nuts/IiWZ2hUuLDA/SNKYYZBelsYJ
	runtime.LockOSThread()
	initThreadID = uint64(C.threadID())
}

var (
	theScreen    *screenImpl
	mainCallback func(screen.Screen)
)

func main(f func(screen.Screen)) (retErr error) {
	if tid := uint64(C.threadID()); tid != initThreadID {
		log.Fatalf("gldriver.Main called on thread %d, but gldriver.init ran on %d", tid, initThreadID)
	}

	theScreen = &screenImpl{
		windows: make(map[uintptr]*windowImpl),
	}
	mainCallback = f
	C.runDriver()
	return nil
}

//export driverStarted
func driverStarted() {
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
	w.draw <- struct{}{}
	<-w.drawDone
}

// loop is the primary drawing loop.
//
// After Cocoa has captured the initial OS thread for processing Cocoa
// events in runApp, it starts loop on another goroutine. It is locked
// to an OS thread for its OpenGL context.
//
// Two Cocoa threads deliver draw signals to loop. The primary source of
// draw events is the CVDisplayLink timer, which is tied to the display
// vsync. Secondary draw events come from [NSView drawRect:] when the
// window is resized.
func (w *windowImpl) drawLoop(ctx uintptr) {
	runtime.LockOSThread()
	C.makeCurrentContext(C.uintptr_t(ctx))

	for range w.draw {
		w.eventsIn <- paint.Event{}
	loop1:
		for {
			select {
			case <-gl.WorkAvailable:
				gl.DoWork()
			case <-w.endPaint:
				C.CGLFlushDrawable(C.CGLGetCurrentContext())
				break loop1
			}
		}
		w.drawDone <- struct{}{}
	}
}

//export setGeom
func setGeom(id uintptr, ppp float32, widthPx, heightPx int) {
	theScreen.mu.Lock()
	w := theScreen.windows[id]
	theScreen.mu.Unlock()
	w.eventsIn <- config.Event{
		WidthPx:     widthPx,
		HeightPx:    heightPx,
		WidthPt:     geom.Pt(float32(widthPx) / ppp),
		HeightPt:    geom.Pt(float32(heightPx) / ppp),
		PixelsPerPt: ppp,
	}
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
