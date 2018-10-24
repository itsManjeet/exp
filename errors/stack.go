// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errors

import (
	"runtime"
)

// A Frame contains part of a call stack.
type Frame struct {
	// Make room for three PCs: the one we were asked for, what it called,
	// and possibly a PC for skipPleaseUseCallersFrames. See:
	// https://go.googlesource.com/go/+/032678e0fb/src/runtime/extern.go#169
	frames [3]uintptr
}

// Caller reports a Frame about function invocations on the calling goroutine's
// stack. The argument skip is the number of stack frames to ascend, with 0
// identifying the caller of Caller.
func Caller(skip int) Frame {
	var s Frame
	runtime.Callers(skip+1, s.frames[:])
	return s
}

// Location reports the file, line, and function of a frame.
//
// The returned function may be "" even if file and line are not.
func (f Frame) Location() (function, file string, line int) {
	frames := runtime.CallersFrames(f.frames[:])
	if _, ok := frames.Next(); !ok {
		return "", "", 0
	}
	fr, ok := frames.Next()
	if !ok {
		return "", "", 0
	}
	return fr.Function, fr.File, fr.Line
}

// Format prints the stack as error detail.
func (f Frame) Format(p Printer) {
	if p.Detail() {
		function, file, line := f.Location()
		if function != "" {
			p.Printf("%s\n    ", function)
		}
		if file != "" {
			p.Printf("%s:%d\n", file, line)
		}
	}
}
