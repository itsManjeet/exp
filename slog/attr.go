// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slog

import (
	"fmt"
	"math"
	"strconv"
	"time"
)

// Kind is the kind of an Attr's value.
type Kind int

// The following list is sorted alphabetically, but it's also important that
// AnyKind is 0 so that a zero Attr's value is nil.

const (
	AnyKind Kind = iota
	BoolKind
	DurationKind
	Float64Kind
	Int64Kind
	StringKind
	TimeKind
	Uint64Kind
)

var kindStrings = []string{
	"Any",
	"Bool",
	"Duration",
	"Float64",
	"Int64",
	"String",
	"Time",
	"Uint64",
}

func (k Kind) String() string {
	if k >= 0 && int(k) < len(kindStrings) {
		return kindStrings[k]
	}
	return "<unknown slog.Kind>"
}

//////////////// Constructors

// Int64 returns an Attr for an int64.
func Int64(key string, value int64) Attr {
	return Attr{key: key, num: uint64(value), any: Int64Kind}
}

// Int converts an int to an int64 and returns
// an Attr with that value.
func Int(key string, value int) Attr {
	return Int64(key, int64(value))
}

// Uint64 returns an Attr for a uint64.
func Uint64(key string, value uint64) Attr {
	return Attr{key: key, num: value, any: Uint64Kind}
}

// Time returns an Attr for a time.Time.
// It discards the monotonic portion.
func Time(key string, value time.Time) Attr {
	return Attr{key: key, num: uint64(value.UnixNano()), any: value.Location()}
}

// Duration returns an Attr for a time.Duration.
func Duration(key string, value time.Duration) Attr {
	return Attr{key: key, num: uint64(value.Nanoseconds()), any: DurationKind}
}

// Any returns an Attr for the supplied value.
//
// Given a value of one of Go's predeclared string, bool, or
// (non-complex) numeric types, Any returns an Attr of kind
// String, Bool, Uint64, Int64, or Float64. The width of the
// original numeric type is not preserved.
//
// Given a time.Time or time.Duration value, Any returns an Attr of kind
// TimeKind or DurationKind. The monotonic time is not preserved.
//
// For nil, or values of all other types, including named types whose
// underlying type is numeric, Any returns a value of kind AnyKind.

func Any[T any](key string, val T) Attr {
	a := Attr{key: key}
	setAttrValue(&a, val)
	return a
}

func setAttrValue[T any](a *Attr, val T) {
	switch any((*T)(nil)).(type) {
	case *string:
		a.setString(any(val).(string))
	case *int:
		a.any = Int64Kind
		a.num = uint64(any(val).(int))
	case *any:
		a.setAnyValue(any(val))
	case *int64:
		a.any = Int64Kind
		a.num = uint64(any(val).(int64))
	case *uint64:
		a.any = Uint64Kind
		a.num = any(val).(uint64)
	case *time.Duration:
		a.any = DurationKind
		a.num = uint64(any(val).(time.Duration).Nanoseconds())
	case *bool:
		a.any = BoolKind
		a.num = 0
		if any(val).(bool) {
			a.num = 1
		}
	case *time.Time:
		a.any = TimeKind
		tm := any(val).(time.Time)
		a.num = uint64(tm.UnixNano())
		a.any = tm.Location()
	case *int8:
		a.any = Int64Kind
		a.num = uint64(any(val).(int8))
	case *int16:
		a.any = Int64Kind
		a.num = uint64(any(val).(int16))
	case *int32:
		a.any = Int64Kind
		a.num = uint64(any(val).(int32))
	case *uint8:
		a.any = Uint64Kind
		a.num = uint64(any(val).(uint8))
	case *uint16:
		a.any = Uint64Kind
		a.num = uint64(any(val).(uint16))
	case *uint32:
		a.any = Uint64Kind
		a.num = uint64(any(val).(uint32))
	case *uintptr:
		a.any = Uint64Kind
		a.num = uint64(any(val).(uintptr))
	case *float32:
		a.any = Float64Kind
		a.num = math.Float64bits(float64(any(val).(float32)))
	case *float64:
		a.any = Float64Kind
		a.num = math.Float64bits(any(val).(float64))
	case Kind:
		panic("cannot store a slog.Kind in an Attr")
	case *time.Location:
		panic("cannot store a *time.Location in an Attr")
	default:
		a.any = any(val)
	}
}

func (a *Attr) setAnyValue(value any) {
	switch v := value.(type) {
	case string:
		a.setString(v)
	case int:
		setAttrValue(a, v)
	case int64:
		setAttrValue(a, v)
	case uint64:
		setAttrValue(a, v)
	case bool:
		setAttrValue(a, v)
	case time.Duration:
		setAttrValue(a, v)
	case time.Time:
		setAttrValue(a, v)
	case uint8:
		setAttrValue(a, v)
	case uint16:
		setAttrValue(a, v)
	case uint32:
		setAttrValue(a, v)
	case uintptr:
		setAttrValue(a, v)
	case int8:
		setAttrValue(a, v)
	case int16:
		setAttrValue(a, v)
	case int32:
		setAttrValue(a, v)
	case float64:
		setAttrValue(a, v)
	case float32:
		setAttrValue(a, v)
	case Kind:
		panic("cannot store a slog.Kind in an Attr")
	case *time.Location:
		panic("cannot store a *time.Location in an Attr")
	default:
		a.any = v
	}
}

//////////////// Accessors

// Key returns the Attr's key.
func (a Attr) Key() string { return a.key }

// Value returns the Attr's value as an any.
func (a Attr) Value() any {
	switch a.Kind() {
	case AnyKind:
		return a.any
	case Int64Kind:
		return int64(a.num)
	case Uint64Kind:
		return a.num
	case Float64Kind:
		return a.float()
	case StringKind:
		return a.str()
	case BoolKind:
		return a.bool()
	case DurationKind:
		return a.duration()
	case TimeKind:
		return a.time()
	default:
		panic("bad kind")
	}
}

// Int64 returns the Attr's value as an int64. It panics
// if the value is not a signed integer.
func (a Attr) Int64() int64 {
	if g, w := a.Kind(), Int64Kind; g != w {
		panic(fmt.Sprintf("Attr kind is %s, not %s", g, w))
	}
	return int64(a.num)
}

// Uint64 returns the Attr's value as a uint64. It panics
// if the value is not an unsigned integer.
func (a Attr) Uint64() uint64 {
	if g, w := a.Kind(), Uint64Kind; g != w {
		panic(fmt.Sprintf("Attr kind is %s, not %s", g, w))
	}
	return a.num
}

// Bool returns the Attr's value as a bool. It panics
// if the value is not a bool.
func (a Attr) Bool() bool {
	if g, w := a.Kind(), BoolKind; g != w {
		panic(fmt.Sprintf("Attr kind is %s, not %s", g, w))
	}
	return a.bool()
}

func (a Attr) bool() bool {
	return a.num == 1
}

// Duration returns the Attr's value as a time.Duration. It panics
// if the value is not a time.Duration.
func (a Attr) Duration() time.Duration {
	if g, w := a.Kind(), DurationKind; g != w {
		panic(fmt.Sprintf("Attr kind is %s, not %s", g, w))
	}

	return a.duration()
}

func (a Attr) duration() time.Duration {
	return time.Duration(int64(a.num))
}

// Float64 returns the Attr's value as a float64. It panics
// if the value is not a float64.
func (a Attr) Float64() float64 {
	if g, w := a.Kind(), Float64Kind; g != w {
		panic(fmt.Sprintf("Attr kind is %s, not %s", g, w))
	}
	return a.float()
}

func (a Attr) float() float64 {
	return math.Float64frombits(a.num)
}

// Time returns the Attr's value as a time.Time. It panics
// if the value is not a time.Time.
func (a Attr) Time() time.Time {
	if g, w := a.Kind(), TimeKind; g != w {
		panic(fmt.Sprintf("Attr kind is %s, not %s", g, w))
	}
	return a.time()
}

func (a Attr) time() time.Time {
	return time.Unix(0, int64(a.num)).In(a.any.(*time.Location))
}

//////////////// Other

// WithKey returns an attr with the given key and the receiver's value.
func (a Attr) WithKey(key string) Attr {
	a.key = key
	return a
}

// Equal reports whether two Attrs have equal keys and values.
func (a1 Attr) Equal(a2 Attr) bool {
	if a1.key != a2.key {
		return false
	}
	k1 := a1.Kind()
	k2 := a2.Kind()
	if k1 != k2 {
		return false
	}
	switch k1 {
	case Int64Kind, Uint64Kind, BoolKind, DurationKind:
		return a1.num == a2.num
	case StringKind:
		return a1.str() == a2.str()
	case Float64Kind:
		return a1.float() == a2.float()
	case TimeKind:
		return a1.time().Equal(a2.time())
	case AnyKind:
		return a1.any == a2.any // may panic if non-comparable
	default:
		panic(fmt.Sprintf("bad kind: %s", k1))
	}
}

// appendValue appends a text representation of the Attr's value to dst.
// The value is formatted as with fmt.Sprint.
func (a Attr) appendValue(dst []byte) []byte {
	switch a.Kind() {
	case StringKind:
		return append(dst, a.str()...)
	case Int64Kind:
		return strconv.AppendInt(dst, int64(a.num), 10)
	case Uint64Kind:
		return strconv.AppendUint(dst, a.num, 10)
	case Float64Kind:
		return strconv.AppendFloat(dst, a.float(), 'g', -1, 64)
	case BoolKind:
		return strconv.AppendBool(dst, a.bool())
	case DurationKind:
		return append(dst, a.duration().String()...)
	case TimeKind:
		return append(dst, a.time().String()...)
	case AnyKind:
		return append(dst, fmt.Sprint(a.any)...)
	default:
		panic(fmt.Sprintf("bad kind: %s", a.Kind()))
	}
}

// Format implements fmt.Formatter.
// It formats an Attr as "KEY=VALUE".
func (a Attr) Format(s fmt.State, verb rune) {
	// TODO: consider verbs and flags
	fmt.Fprintf(s, "%s=%v", a.Key(), a.Value())
}
