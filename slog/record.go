// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slog

import (
	"runtime"
	"time"
)

const nAttrsInline = 5

// A Record holds information about a log event.
type Record struct {
	// The time at which the output method (Print, Fatal, etc.) was called.
	time time.Time

	// The log message.
	message string

	level Level

	pc uintptr

	// Allocation optimization: an inline array sized to hold
	// the majority of log calls (based on examination of open-source
	// code)
	attrs1 [nAttrsInline]Attr
	// The number of Attrs in attrs1.
	n1     int
	attrs2 list[Attr]
}

// NewRecord creates a new Record from the given arguments.
// Use [Record.AddAttr] to add attributes to the Record.
// If calldepth is greater than zero, [Record.SourceLine] will
// return the file and line number at that depth.
//
// NewRecord is intended for logging APIs that want to support a [Handler] as
// a backend. Most users won't need it.
func NewRecord(t time.Time, level Level, msg string, calldepth int) Record {
	var p uintptr
	if calldepth > 0 {
		p = pc(calldepth + 1)
	}
	return Record{
		time:    t,
		message: msg,
		level:   level,
		pc:      p,
	}
}

func pc(depth int) uintptr {
	var pcs [1]uintptr
	runtime.Callers(depth, pcs[:])
	return pcs[0]
}

// Time returns the time of the log event.
func (r *Record) Time() time.Time { return r.time }

// Message returns the log message.
func (r *Record) Message() string { return r.message }

// Level returns the level of the log event.
func (r *Record) Level() Level { return r.level }

// SourceLine returns the file and line of the log event.
// If the Record was created without the necessary information,
// or if the location is unavailable, it returns ("", 0).
func (r *Record) SourceLine() (file string, line int) {
	fs := runtime.CallersFrames([]uintptr{r.pc})
	// TODO: error-checking?
	f, _ := fs.Next()
	return f.File, f.Line
}

// Attrs returns a copy of the sequence of Attrs in r.
func (r *Record) Attrs() []Attr {
	// concat always makes a new slice.
	r.attrs2 = r.attrs2.normalize()
	return concat(r.attrs1[:r.n1], r.attrs2.front)
}

// NumAttrs returns the number of Attrs in r.
func (r *Record) NumAttrs() int {
	return r.n1 + r.attrs2.len()
}

// Attr returns the i'th Attr in r.
func (r *Record) Attr(i int) Attr {
	if i < r.n1 {
		return r.attrs1[i]
	}
	r.attrs2 = r.attrs2.normalize()
	return r.attrs2.at(i - r.n1)
}

// AddAttr appends a to the list of r's attributes.
// It does not check for duplicate keys.
func (r *Record) AddAttr(a Attr) {
	if r.n1 < len(r.attrs1) {
		r.attrs1[r.n1] = a
		r.n1++
	} else {
		r.attrs2 = r.attrs2.append(a)
	}
}
