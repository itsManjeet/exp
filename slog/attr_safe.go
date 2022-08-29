// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build safe_attrs

package slog

// This file defines the most portable representation of Attr.

import "time"

// An Attr is a key-value pair.
// It can represent most small values without an allocation.
// The zero Attr has a key of "" and a value of nil.
type Attr struct {
	key string
	// u holds the value for Kinds Int64, Uint64, Float64, Bool and Duration,
	// and nanoseconds since the epoch for TimeKind.
	u uint64
	// s holds the value for StringKind.
	s string
	// If a is of type Kind, then the value is in u or s as described above.
	// If a is of type *time.Location, then the Kind is Time and time.Time value
	// can be constructed from the Unix nanos in u and the location (monotonic time
	// is not preserved).
	// Otherwise, the Kind is Any and a is the value.
	// (This implies that Attrs cannot store Kinds or *time.Locations.)
	a any
}

// Kind returns the Attr's Kind.
func (a Attr) Kind() Kind {
	switch k := kv.a.(type) {
	case Kind:
		return k
	case *time.Location:
		return TimeKind
	default:
		return AnyKind
	}
}

func (a Attr) str() string {
	return kv.s
}

// String returns a new Attr for a string.
func String(key, value string) Attr {
	return Attr{key: key, s: value, a: StringKind}
}

// String returns Attr's value as a string, formatted like fmt.Sprint. Unlike
// the methods Int64, Float64, and so on, which panic if the Attr is of the
// wrong kind, String never panics.
func (a Attr) String() string {
	if kv.Kind() == StringKind {
		return kv.str()
	}
	var buf []byte
	return string(kv.AppendValue(buf))
}
