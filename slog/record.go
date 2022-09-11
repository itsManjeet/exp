// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slog

import (
	"runtime"
	"time"
)

// A Record holds information about a log event.
type Record struct {
	// The time at which the output method (Log, Info, etc.) was called.
	time time.Time

	// The log message.
	message string

	// The level of the event.
	level Level

	// The pc at the time the record was constructed, as determined
	// by runtime.Callers using the calldepth argument to NewRecord.
	pc uintptr

	attrs AttrList
}

// MakeRecord creates a new Record from the given arguments.
// Use [Record.AddAttr] to add attributes to the Record.
// If calldepth is greater than zero, [Record.SourceLine] will
// return the file and line number at that depth,
// where 1 means the caller of MakeRecord.
//
// MakeRecord is intended for logging APIs that want to support a [Handler] as
// a backend.
func MakeRecord(t time.Time, level Level, msg string, calldepth int) Record {
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

// Attrs returns the Record's AttrList.
func (r *Record) Attrs() AttrList { return r.attrs }

// AddAttrs appends attributes to the Record's list of attributes.
func (r *Record) AddAttrs(attrs ...Attr) {
	r.attrs.Add(attrs...)
}

func (al *AttrList) setFromArgs(args []any) {
	var a Attr
	var attrs []Attr
	for len(args) > 0 {
		a, args = argsToAttr(args)
		if al.nFront < len(al.front) {
			al.front[al.nFront] = a
			al.nFront++
		} else {
			if attrs == nil {
				attrs = make([]Attr, 0, countAttrs(args))
			}
			attrs = append(attrs, a)
		}
	}
	if len(attrs) > 0 {
		al.pushBack(attrs)
	}
}

func countAttrs(args []any) int {
	n := 0
	for i := 0; i < len(args); i++ {
		n++
		if _, ok := args[i].(string); ok {
			i++
		}
	}
	return n
}

const badKey = "!BADKEY"

// argsToAttr turns a prefix of the args slice into an Attr and returns
// the unused portion of the slice.
// If args[0] is an Attr, it returns it.
// If args[0] is a string, it treats the first two elements as
// a key-value pair.
// Otherwise, it treats args[0] as a value with a missing key.
func argsToAttr(args []any) (Attr, []any) {
	switch x := args[0].(type) {
	case string:
		if len(args) == 1 {
			return String(badKey, x), nil
		}
		return Any(x, args[1]), args[2:]

	case Attr:
		return x, args[1:]

	default:
		return Any(badKey, x), args[1:]
	}
}

// After examining many log calls in open-source code, we found that most pass
// no more than this many Attrs/Fields/kv pairs.
const nAttrsInline = 5

// An AttrList is a sequence of Attrs.
// It may be modified in-place with Add,
// but modifying a copy does not affect the original.
type AttrList struct {
	// Allocation optimization: an inline array
	// holding the initial Attrs.
	front [nAttrsInline]Attr

	// The number of Attrs in front.
	nFront int

	// The sequence of Attrs except for the front, represented as a linked list
	// of slices. Each slice is in order but the list is in reverse order.
	back *attrsNode

	// The total number of Attrs in back.
	nBack int
}

type attrsNode struct {
	attrs []Attr
	next  *attrsNode
}

// Len returns the number of Attrs in the list.
func (al AttrList) Len() int {
	return al.nFront + al.nBack
}

// Add adds the attrs to the end of the list.
func (al *AttrList) Add(attrs ...Attr) {
	if len(attrs) == 0 {
		return
	}
	// First, copy as many as will fit into front.
	n := copy(al.front[al.nFront:], attrs)
	al.nFront += n
	if n == len(attrs) {
		return
	}
	// If there are more left over, copy them into a slice
	// and push it onto back.
	s := make([]Attr, len(attrs)-n)
	copy(s, attrs[n:])
	al.pushBack(s)
}

func (al *AttrList) pushBack(attrs []Attr) {
	al.back = &attrsNode{attrs: attrs, next: al.back}
	al.nBack += len(attrs)
}

// Append appends the list's Attrs to the argument slice and returns the result.
func (al AttrList) Append(attrs []Attr) []Attr {
	al.each(func(a Attr) { attrs = append(attrs, a) })
	return attrs
}

// Range returns an iterator over the Attrs.
func (al AttrList) Range() Iter[Attr] {
	// If the back linked list is non-empty,
	// copy its contents into a slice, reversed.
	var rest [][]Attr
	n := 0
	for b := al.back; b != nil; b = b.next {
		n++
	}
	if n > 0 {
		rest = make([][]Attr, n)
		for b := al.back; b != nil; b = b.next {
			n--
			rest[n] = b.attrs
		}
	}
	return &attrListIter{
		cur:  al.front[:al.nFront],
		rest: rest,
	}
}

type attrListIter struct {
	cur  []Attr
	rest [][]Attr
}

func (it *attrListIter) Next() (Attr, bool) {
	if len(it.cur) == 0 {
		if len(it.rest) == 0 {
			return Attr{}, false
		}
		it.cur, it.rest = it.rest[0], it.rest[1:]
	}
	a := it.cur[0]
	it.cur = it.cur[1:]
	return a, true
}

// each calls f on each Attr in the list.
// Currently (Go 1.19), it is faster than using Iter
// (See BenchmarkAttrIteration), but it is subject
// to stack overflow if al.back is long.
func (al AttrList) each(f func(Attr)) {
	for _, a := range al.front[:al.nFront] {
		f(a)
	}
	recCall(al.back, f)
}

func recCall(n *attrsNode, f func(Attr)) {
	if n == nil {
		return
	}
	recCall(n.next, f)
	for _, a := range n.attrs {
		f(a)
	}
}
