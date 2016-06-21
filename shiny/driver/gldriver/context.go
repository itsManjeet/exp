// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gldriver

import (
	"runtime"

	"golang.org/x/mobile/gl"
)

// NewContext creates an OpenGL ES context with a dedicated processing thread.
func NewContext() gl.Context {
	glctx, worker := gl.NewContext()

	ctxCh := make(chan interface{})
	workAvailable := worker.WorkAvailable()
	go func() {
		runtime.LockOSThread()
		ctx := surfaceCreate()
		ctxCh <- ctx
		if ctx == nil {
			return
		}

		for {
			<-workAvailable
			worker.DoWork()
		}
	}()
	if ctx := <-ctxCh; ctx == nil {
	}
	return glctx
}
