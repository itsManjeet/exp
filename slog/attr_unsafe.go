// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !safe_attrs

package slog

import (
	"reflect"
	"time"
	"unsafe"
)

// An Attr is a key-value pair.
// It can represent some small values without an allocation.
// The zero Attr has a key of "" and a value of nil.
type Attr struct {
	key string
	u   uint64
	a   any
}

// stringptr is used in field `a` when the Value is a string.
type stringptr unsafe.Pointer

// Kind returns the Attr's Kind.
func (a Attr) Kind() Kind {
	switch x := a.a.(type) {
	case Kind:
		return x
	case stringptr:
		return StringKind
	case *time.Location:
		return TimeKind
	default:
		return AnyKind
	}
}

// String returns a new Attr for a string.
func String(key, value string) Attr {
	hdr := (*reflect.StringHeader)(unsafe.Pointer(&value))
	return Attr{key: key, u: uint64(hdr.Len), a: stringptr(hdr.Data)}
}

func (kv Attr) str() string {
	var s string
	hdr := (*reflect.StringHeader)(unsafe.Pointer(&s))
	hdr.Data = uintptr(kv.a.(stringptr))
	hdr.Len = int(kv.u)
	return s
}

// String returns Attr's value as a string, formatted like fmt.Sprint. Unlike
// the methods Int64, Float64, and so on, which panic if the Attr is of the
// wrong kind, String never panics.
func (a Attr) String() string {
	if sp, ok := a.a.(stringptr); ok {
		// Inlining this code makes a huge difference.
		var s string
		hdr := (*reflect.StringHeader)(unsafe.Pointer(&s))
		hdr.Data = uintptr(sp)
		hdr.Len = int(a.u)
		return s
	}
	var buf []byte
	return string(a.AppendValue(buf))
}
