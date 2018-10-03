// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errors

// A Formatter formats error messages.
type Formatter interface {
	// Format is implemented by errors to print a single error message.
	// It should return the next error in the error chain, if any.
	Format(p Printer) (next error)
}

// A Printer creates formatted error messages. It enforces that
// detailed information is written last.
//
// Printer is implemented by fmt. Localization packages may provide
// their own implementation to support localized error messages
// (see for instance golang.org/x/text/message).
type Printer interface {
	// Print appends args to the message output.
	// String arguments are not localized, even within a localized context.
	Print(args ...interface{})

	// Printf writes a formatted string.
	Printf(format string, args ...interface{})

	// Detail reports whether error detail is requested.
	// After the first call to Detail, all text written to the Printer
	// is formatted as additional detail, or ignored when
	// detail has not been requested.
	// If Detail returns false, the caller can avoid printing the detail at all.
	Detail() bool
}
