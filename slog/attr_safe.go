// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build safe_attrs

package slog

import "time"

// An Attr is a key-value pair.
// It can represent some small values without an allocation.
// The zero Attr has a key of "" and a value of nil.
type Attr struct {
	key string
	u   uint64
	s   string
	a   any
}

// Kind returns the Attr's Kind.
func (kv Attr) Kind() Kind {
	switch k := kv.a.(type) {
	case Kind:
		return k
	case *time.Location:
		return TimeKind
	default:
		return AnyKind
	}
}

func (kv Attr) str() string {
	return kv.s
}

// String returns a new Attr for a string.
func String(key, value string) Attr {
	return Attr{key: key, s: value, a: StringKind}
}

// String returns Attr's value as a string, formatted like fmt.Sprint. Unlike
// the methods Int64, Float64, and so on, which panic if the Attr is of the
// wrong kind, String never panics.
func (kv Attr) String() string {
	if kv.Kind() == StringKind {
		return kv.str()
	}
	var buf []byte
	return string(kv.AppendValue(buf))
}
