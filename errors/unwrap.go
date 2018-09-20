// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errors

import (
	"reflect"
)

// An Unwrapper provides context around another error.
type Unwrapper interface {
	// Unwrap returns the next error in the error chain.
	// If there is no next error, Unwrap returns nil.
	Unwrap() error
}

// Unwrap returns the next error in err's chain.
// If there is no next error, Unwrap returns nil.
func Unwrap(err error) error {
	if u, ok := err.(Unwrapper); ok {
		return u.Unwrap()
	}
	return nil
}

// An Iser can declare that it is equivalent to another error.
type Iser interface {
	// Is returns true if the receiver is equivalent to the parameter.
	Is(error) bool
}

// Is returns true if any error in err's chain is equivalent to the target.
//
// Two errors are equivalent if they are equal or if either has an Is(error) bool
// method which returs true for the other.
func Is(err, target error) bool {
	tiser, tok := target.(Iser)
	for err != nil {
		if err == target {
			return true
		}
		if iser, ok := err.(Iser); ok && iser.Is(target) {
			return true
		}
		if tok && tiser.Is(err) {
			return true
		}
		wrapper, ok := err.(Unwrapper)
		if !ok {
			return false
		}
		err = wrapper.Unwrap()
	}
	return false
}

// An Aser can represent itself as another type.
type Aser interface {
	// As returns (x, true) if the receiver can represent itself as a
	// value of the same type as the template.
	//
	// If ok is true, the concrete type of the returned error must be the
	// same as that of the template.
	As(template error) (error, bool)
}

// As returns the first error in err's chain that can be represented as the
// the concrete type of the template.
//
// An error can be represented as a type if it is of the same type, or if
// it has an As(error) (error, bool) method that returns true for the type.
func As(err, target error) (error, bool) {
	targetType := reflect.TypeOf(target)
	for err != nil {
		if reflect.TypeOf(err) == targetType {
			return err, true
		}
		if aser, ok := err.(Aser); ok {
			if e, ok := aser.As(target); ok && reflect.TypeOf(e) == targetType {
				return e, true
			}
		}
		wrapper, ok := err.(Unwrapper)
		if !ok {
			return nil, false
		}
		err = wrapper.Unwrap()
	}
	return nil, false
}
