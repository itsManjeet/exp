// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package value provides the Value type, a non-allocating alternative to
// interface{}.
package value

import (
	"fmt"
	"math"
	"reflect"
	"unsafe"
)

// A Value is a type that can hold any Go value, like interface{}.
// Unlike interface{}, placing a Go value into a Value never
// allocates. Also, once stored in a Value it is not always
// possible to recover the exact type, but the kind is preserved
// (one of int, uint, float, bool, string, or interface{}).
//
// The empty Value represents nil.
type Value struct {
	packed  uint64
	untyped interface{}
}

// A kind is the kind of value stored. For non-interface{} values, kind is
// stored in the untyped field. Since it is an unexported type, outside code
// cannot spoof it, so we can be sure that if v.untyped == intKind, then
// v.packed was created with OfInt.
type kind int

const (
	intKind kind = iota
	uintKind
	floatKind
	boolKind
)

var kindString = map[kind]string{
	intKind:   "int",
	uintKind:  "uint",
	floatKind: "float",
	boolKind:  "bool",
}

func OfInt(x int64) Value {
	return Value{
		packed:  uint64(x),
		untyped: intKind,
	}
}

// This is called AsInt instead of Int, because String has a special meaning in
// Go.

func (v Value) AsInt() int64 {
	v.checkKind(intKind)
	return int64(v.packed)
}

func OfUint(x uint64) Value {
	return Value{
		packed:  x,
		untyped: uintKind,
	}
}

func (v Value) AsUint() uint64 {
	v.checkKind(uintKind)
	return v.packed
}

func OfFloat(x float64) Value {
	return Value{
		packed:  math.Float64bits(x),
		untyped: floatKind,
	}
}

func (v Value) AsFloat() float64 {
	v.checkKind(floatKind)
	return math.Float64frombits(v.packed)
}

func OfBool(x bool) Value {
	var b uint64
	if x {
		b = 1
	}
	return Value{
		packed:  b,
		untyped: boolKind,
	}
}

func (v Value) AsBool() bool {
	v.checkKind(boolKind)
	return unpackBool(v.packed)
}

func unpackBool(u uint64) bool {
	if u == 0 {
		return false
	}
	return true
}

type stringptr unsafe.Pointer

func OfString(x string) Value {
	hdr := (*reflect.StringHeader)(unsafe.Pointer(&x))
	return Value{
		packed:  uint64(hdr.Len),
		untyped: stringptr(hdr.Data),
	}
}

func (v Value) AsString() string {
	sp, ok := v.untyped.(stringptr)
	if !ok {
		panic(kindError("string"))
	}
	return unpackString(v.packed, sp)
}

func unpackString(packed uint64, sp stringptr) string {
	var s string
	hdr := (*reflect.StringHeader)(unsafe.Pointer(&s))
	hdr.Data = uintptr(sp)
	hdr.Len = int(packed)
	return s
}

// OfInterface creates a new Value from an arbitrary Go type.
func OfInterface(x interface{}) Value {
	return Value{untyped: x}
}

// AsInterface returns the Go value stored in v as an interface{}.
// Unlike the other As functions, does not panic if the value was
// not created with OfInterface; it works for any Value.
func (v Value) AsInterface() interface{} {
	switch v.untyped {
	case intKind:
		return int64(v.packed)
	case uintKind:
		return v.packed
	case floatKind:
		return math.Float64frombits(v.packed)
	case boolKind:
		return unpackBool(v.packed)
	default:
		if s, ok := v.untyped.(stringptr); ok {
			return unpackString(v.packed, s)
		}
		return v.untyped
	}
}

func (v Value) checkKind(k kind) {
	if v.untyped != k {
		panic(kindError(kindString[k]))
	}
}

func kindError(s string) string {
	return fmt.Sprintf("bad kind: expected %s", s)
}

// Handler is used with Apply to get a Value's contents.
type Handler interface {
	Int(v int64)
	Uint(v uint64)
	Float(v float64)
	Bool(v bool)
	String(v string)
	Interface(v interface{})
}

func (v Value) Apply(h Handler) {
	switch v.untyped {
	case intKind:
		h.Int(int64(v.packed))
	case uintKind:
		h.Uint(v.packed)
	case floatKind:
		h.Float(math.Float64frombits(v.packed))
	case boolKind:
		h.Bool(unpackBool(v.packed))
	default:
		if s, ok := v.untyped.(stringptr); ok {
			h.String(unpackString(v.packed, s))
			return
		}
		h.Interface(v.untyped)
	}
}

func (v Value) String() string {
	return fmt.Sprint(v.AsInterface())
}
