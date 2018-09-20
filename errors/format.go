// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errors

import (
	"bytes"
	"fmt"
)

// A Formatter formats error messages.
type Formatter interface {
	// Format is implemented by errors to print a single error message.
	// It should return the next error in the error chain, if any.
	FormatError(p Printer) (next error)
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

// Format creates a formatted error message.
func Format(err error, s fmt.State, verb rune) {
	p := &printer{
		detail: verb == 'v' && s.Flag('+'),
		state:  s,
	}
	for {
		p.inDetail = false
		if f, ok := err.(Formatter); ok {
			if err = f.FormatError(p); err == nil {
				return
			}
		} else {
			p.Print(err)
			return
		}
		if p.detail {
			fmt.Fprint(s, "\n--- ")
		} else {
			fmt.Fprint(s, ": ")
		}
	}
}

type printer struct {
	detail   bool
	inDetail bool
	indent   bool
	state    fmt.State
}

func (p *printer) Write(b []byte) (n int, err error) {
	if p.inDetail && !p.detail {
		return len(b), nil
	}
	if p.indent {
		b = bytes.Replace(b, []byte("\n"), []byte("\n    "), -1)
	}
	return p.state.Write(b)
}

func (p *printer) Print(args ...interface{}) {
	fmt.Fprint(p, args...)
}

func (p *printer) Printf(format string, args ...interface{}) {
	fmt.Fprintf(p, format, args...)
}

func (p *printer) Detail() bool {
	p.indent = true
	if !p.inDetail {
		p.inDetail = true
		p.Print("\n")
	}
	return p.detail
}
