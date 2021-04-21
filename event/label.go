// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package event

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"unsafe"
)

// Value holds any value in an efficient way that avoids allocations for
// most types.
type Value struct {
	packed  uint64
	untyped interface{}
}

// Label holds a key and value pair.
type Label struct {
	Key   string
	Value Value
}

// stringptr is used in untyped when the Value is a string
type stringptr unsafe.Pointer

// int64Kind is used in untyped when the Value is a signed integer
type int64Kind struct{}

// uint64Kind is used in untyped when the Value is an unsigned integer
type uint64Kind struct{}

// float64Kind is used in untyped when the Value is a floating point number
type float64Kind struct{}

// boolKind is used in untyped when the Value is a boolean
type boolKind struct{}

// Format prints the value in a standard form.
func (l *Label) Format(f fmt.State, verb rune) {
	buf := bufPool.Get().(*buffer)
	l.format(f.(writer), verb, buf.data[:0])
	bufPool.Put(buf)
}

func (l *Label) format(w writer, verb rune, buf []byte) {
	w.Write(strconv.AppendQuote(buf[:0], l.Key))
	w.WriteString(":")
	l.Value.format(w, verb, buf)
}

// Valid returns true if the Label is a valid one (it has a key).
func (l *Label) Valid() bool { return l.Key != "" }

// Format prints the value in a standard form.
func (v *Value) Format(f fmt.State, verb rune) {
	buf := bufPool.Get().(*buffer)
	v.format(f.(writer), verb, buf.data[:0])
	bufPool.Put(buf)
}

func (v *Value) format(w writer, verb rune, buf []byte) {
	switch {
	case v.IsString():
		w.Write(strconv.AppendQuote(buf[:0], v.String()))
	case v.IsInt64():
		w.Write(strconv.AppendInt(buf[:0], v.Int64(), 10))
	case v.IsUint64():
		w.Write(strconv.AppendUint(buf[:0], v.Uint64(), 10))
	case v.IsFloat64():
		w.Write(strconv.AppendFloat(buf[:0], v.Float64(), 'E', -1, 32))
	case v.IsBool():
		if v.Bool() {
			w.WriteString("true")
		} else {
			w.WriteString("false")
		}
	default:
		fmt.Fprint(w, v.Interface())
	}
}

func (v *Value) HasValue() bool { return v.untyped != nil }

// SetInterface the value to an interface{} value.
func (v *Value) SetInterface(value interface{}) {
	v.untyped = value
}

func (v *Value) Interface() interface{} {
	//TODO: should we check it is not one of the other types here?
	return v.untyped
}

// SetString sets the value to a string form.
func (v *Value) SetString(s string) {
	hdr := (*reflect.StringHeader)(unsafe.Pointer(&s))
	v.packed = uint64(hdr.Len)
	v.untyped = stringptr(hdr.Data)
}

// String returns the value as a string.
// It panics if the value was not built with SetString.
func (v Value) String() string {
	var s string
	hdr := (*reflect.StringHeader)(unsafe.Pointer(&s))
	hdr.Data = uintptr(v.untyped.(stringptr))
	hdr.Len = int(v.packed)
	return s
}

func (v Value) IsString() bool {
	_, ok := v.untyped.(stringptr)
	return ok
}

// SetInt64 sets the value to a signed integer.
func (v *Value) SetInt64(u int64) {
	v.packed = uint64(u)
	v.untyped = int64Kind{}
}

// Int64
func (v Value) Int64() int64 {
	//TODO: panic if v.untyped is not uintKind
	return int64(v.packed)
}

func (v Value) IsInt64() bool {
	_, ok := v.untyped.(int64Kind)
	return ok
}

// SetUint64 sets the value to an unsigned integer.
func (v *Value) SetUint64(u uint64) {
	v.packed = u
	v.untyped = uint64Kind{}
}

// Uint64
func (v Value) Uint64() uint64 {
	//TODO: panic if v.untyped is not uintKind
	return v.packed
}

func (v Value) IsUint64() bool {
	_, ok := v.untyped.(uint64Kind)
	return ok
}

// SetFloat64 sets the value to an unsigned integer.
func (v *Value) SetFloat64(f float64) {
	v.packed = math.Float64bits(f)
	v.untyped = float64Kind{}
}

// Float64
func (v Value) Float64() float64 {
	//TODO: panic if v.untyped is not floatKind
	return math.Float64frombits(v.packed)
}

func (v Value) IsFloat64() bool {
	_, ok := v.untyped.(float64Kind)
	return ok
}

// SetBool sets the value to an unsigned integer.
func (v *Value) SetBool(b bool) {
	if b {
		v.packed = 1
	} else {
		v.packed = 0
	}
	v.untyped = boolKind{}
}

// Bool
func (v Value) Bool() bool {
	//TODO: panic if v.untyped is not boolKind
	if v.packed != 0 {
		return true
	}
	return false
}

func (v Value) IsBool() bool {
	_, ok := v.untyped.(boolKind)
	return ok
}
