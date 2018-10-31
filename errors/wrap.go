// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errors

import (
	"reflect"
)

// An Wrapper provides context around another error.
type Wrapper interface {
	// Unwrap returns the next error in the error chain.
	// If there is no next error, Unwrap returns nil.
	Unwrap() error
}

// Opaque returns an error that will not show up in a chain of wrapped errors.
func Opaque(err error) error {
	return noWrapper{err}
}

type noWrapper struct {
	error
}

func (e noWrapper) Format(p Printer) (next error) {
	if f, ok := e.error.(Formatter); ok {
		return f.Format(p)
	}
	p.Print(e.error)
	return nil
}

// Unwrap returns the next error in err's chain.
// If there is no next error, Unwrap returns nil.
func Unwrap(err error) error {
	u, ok := err.(Wrapper)
	if !ok {
		return nil
	}
	err = u.Unwrap()
	if err == nil {
		return nil
	}
	if _, ok := err.(noWrapper); ok {
		return nil
	}
	return err
}

// An isser can declare that it is equivalent to another error.
type isser interface {
	// Is returns true if the receiver is equivalent to the parameter.
	Is(error) bool
}

// Is returns true if any error in err's chain is equal to target.
func Is(err, target error) bool {
	for err != nil {
		if _, ok := err.(noWrapper); ok {
			return false
		}
		if err == target {
			return true
		}
		if isser, ok := err.(isser); ok && isser.Is(target) {
			return true
		}
		u, ok := err.(Wrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

// An asser can represent itself as another type.
type asser interface {
	// As returns (x, true) if the receiver can represent itself as a
	// value of the same type as the template.
	//
	// If ok is true, the concrete type of the returned error must be the
	// same as that of the template.
	As(target interface{}) bool
}

// As finds the first error in err's chain that matches a type to which target
// points, and if so, sets the target to its value and reports success.
//
// As will panic if target is nil.
func As(err error, target interface{}) bool {
	targetType := reflect.TypeOf(target).Elem()
	for err != nil {
		if _, ok := err.(noWrapper); ok {
			return false
		}
		if reflect.TypeOf(err) == targetType {
			reflect.ValueOf(target).Elem().Set(reflect.ValueOf(err))
			return true
		}
		if asser, ok := err.(asser); ok {
			if asser.As(target) {
				return true
			}
		}
		u, ok := err.(Wrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
