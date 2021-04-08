// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package event

import (
	"reflect"
	"unsafe"
)

// Key is used as the identity of a Label.
// Keys are intended to be compared by pointer only, the name should be unique
// for communicating with external systems, but it is not required or enforced.
type Key interface {
	// Name returns the key name.
	Name() string

	// Print is used in printing to append the value of the label to the
	// supplied printer.
	Print(p Printer, l Label)
}

// Label holds a key and value pair.
// It is normally used when passing around lists of labels.
type Label struct {
	key     Key
	packed  uint64
	untyped interface{}
}

// OfValue creates a new label from the key and value.
// This method is for implementing new key types, label creation should
// normally be done with the Of method of the key.
func OfValue(k Key, value interface{}) Label { return Label{key: k, untyped: value} }

// UnpackValue assumes the label was built using LabelOfValue and returns the value
// that was passed to that constructor.
// This method is for implementing new key types, for type safety normal
// access should be done with the From method of the key.
func (l Label) UnpackValue() interface{} { return l.untyped }

// Of64 creates a new label from a key and a uint64. This is often
// used for non uint64 values that can be packed into a uint64.
// This method is for implementing new key types, label creation should
// normally be done with the Of method of the key.
func Of64(k Key, v uint64) Label { return Label{key: k, packed: v} }

// Unpack64 assumes the label was built using LabelOf64 and returns the value that
// was passed to that constructor.
// This method is for implementing new key types, for type safety normal
// access should be done with the From method of the key.
func (l Label) Unpack64() uint64 { return l.packed }

type stringptr unsafe.Pointer

// OfString creates a new label from a key and a string.
// This method is for implementing new key types, label creation should
// normally be done with the Of method of the key.
func OfString(k Key, v string) Label {
	hdr := (*reflect.StringHeader)(unsafe.Pointer(&v))
	return Label{
		key:     k,
		packed:  uint64(hdr.Len),
		untyped: stringptr(hdr.Data),
	}
}

// UnpackString assumes the label was built using LabelOfString and returns the
// value that was passed to that constructor.
// This method is for implementing new key types, for type safety normal
// access should be done with the From method of the key.
func (l Label) UnpackString() string {
	var v string
	hdr := (*reflect.StringHeader)(unsafe.Pointer(&v))
	hdr.Data = uintptr(l.untyped.(stringptr))
	hdr.Len = int(l.packed)
	return v
}

// Valid returns true if the Label is a valid one (it has a key).
func (l Label) Valid() bool { return l.key != nil }

// Key returns the key of this Label.
func (l Label) Key() Key { return l.key }
