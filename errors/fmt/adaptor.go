// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fmt

// The functionality in this file is to provide adaptors only. It will not
// be included in the standard library.

import "golang.org/x/exp/errors"

// FormatError calls the FormatError method of err with a errors.Printer
// configured according to s and verb and writes the result to s.
func FormatError(s State, verb rune, err ErrorFormatter) {
	p := newPrinter()
	if verb == 'v' {
		if s.Flag('#') {
			p.fmt.sharpV = true
		}
		if s.Flag('+') {
			p.fmt.plusV = true
		}
	}
	fmtError(p, verb, err)
	s.Write(p.buf)
}

// ErrorFormatter is like errors.Formatter, but with different names.
// Packages can implement to provide error formatting functionality for
// older Go versions.
type ErrorFormatter interface {
	error
	FormatError(errors.Printer) error
}
