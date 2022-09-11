// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Temporarily define Iter here in anticipation of it
// becoming part of the standard library.

package slog

// Iter supports iterating over a sequence of values of type `E`.
type Iter[E any] interface {
	// Next returns the next value in the iteration if there is one,
	// and reports whether the returned value is valid.
	// Once Next returns ok==false, the iteration is over,
	// and all subsequent calls will return ok==false.
	Next() (elem E, ok bool)
}
