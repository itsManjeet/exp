// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errors

import (
	"fmt"
	"runtime"
	"strings"
)

// A Stack contains part of a call stack.
type Stack struct {
	// Make room for three PCs: the one we were asked for, what it called,
	// and possibly a PC for skipPleaseUseCallersFrames. See:
	// https://go.googlesource.com/go/+/032678e0fb/src/runtime/extern.go#169
	frames [3]uintptr
}

// NewStack returns a Stack containing the call frame of the caller of the
// caller of NewStack.
func NewStack() Stack {
	var s Stack
	runtime.Callers(2, s.frames[:])
	return s
}

func (s Stack) String() string {
	frames := runtime.CallersFrames(s.frames[:])
	if _, ok := frames.Next(); !ok {
		return ""
	}
	fr, ok := frames.Next()
	if !ok {
		return ""
	}
	file := fr.File
	if i := strings.LastIndex(file, "/"); i >= 0 {
		file = file[i+1:]
	}
	return fmt.Sprintf("%s:%d", file, fr.Line)
}

// FormatError prints the stack as error detail.
func (s Stack) FormatError(p Printer) {
	if p.Detail() {
		p.Print(s.String())
	}
}
