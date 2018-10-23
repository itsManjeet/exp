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

// Is returns true if any error in err's chain is a target.
//
// An error is considered to be a target if they are equal or if this error
// implements and Is(error) bool method that returns true for target.
func Is(err, target error) bool {
	for err != nil {
		if _, ok := err.(noWrapper); ok {
			return false
		}
		if err == target {
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

// As finds the first error in err's chain that matches a type to which target
// points, and if so, sets the target to its value and reports success.
//
// An error can be represented as a type if it is of the same type, or if
// it has an As(interface{}) bool method that returns true for the given target.
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
		u, ok := err.(Wrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
