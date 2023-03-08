// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gobinary

import (
	"errors"
)

var (
	// ErrInlined is returned from Func methods only defined for physical
	// functions if called on an inlined function.
	ErrInlined = errors.New("function is inlined")

	// ErrPhysical is returned from Func methods only defined for inlined
	// functions if called on a physical function.
	ErrPhysical = errors.New("function is physical")
)

// Func represents a function. This may either be a physical function in the
// binary, or representation of an inlined function.
type Func struct{}

// Name returns the symbol name of the function, including the package name.
func (*Func) Name() string {
	return "unimplemented"
}

// Inlined returns true if this is a representation of an inlined function.
func (*Func) Inlined() bool {
	// TODO: Unimplemented
	return false
}

// StartLine returns the source line number (potentially adjusted by //line
// directives) of the start of this function (i.e., the line containing the
// func keyword).
func (*Func) StartLine() (int32, error) {
	return 0, errors.New("unimplemented")
}

// Entry returns the entry point of a physical function.
//
// Preconditions:
//
// * Function is not inlined.
func (f *Func) Entry() (uint64, error) {
	if f.Inlined() {
		return 0, ErrInlined
	}

	return 0, errors.New("unimplemented")
}

// Parent returns the parent Func containing a call to this inlined function.
// Note that because there may be multiple levels of inlining, the returned
// parent may also be inlined. Each parent chain eventually ends at a physical
// function.
//
// Preconditions:
//
// * Function is inlined.
func (f *Func) Parent() (*Func, error) {
	if !f.Inlined() {
		return nil, ErrPhysical
	}

	return nil, errors.New("unimplemented")
}
