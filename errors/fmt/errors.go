// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fmt

import (
	"bytes"
	"strings"

	"golang.org/x/exp/errors"
)

func errorf(format string, a []interface{}) error {
	err := lastError(format, a)
	if err == nil {
		return &simpleErr{Sprintf(format, a...), errors.Caller(1)}
	}

	// TODO: this is not entirely correct. The error value could be
	// printed elsewhere in format if it mixes numbered with unnumbered
	// substitutions. With relatively small changes to doPrintf we can
	// have it optionally ignore extra arguments and pass the argument
	// list in its entirety.
	format = format[:len(format)-len(": %s")]
	return &withChain{
		msg:   Sprintf(format, a[:len(a)-1]...),
		err:   err,
		Frame: errors.Caller(1),
	}
}

func lastError(format string, a []interface{}) error {
	if len(a) == 0 {
		return nil
	}

	p := len(a) - 1
	err, ok := a[p].(error)
	if !ok {
		return nil
	}

	if !strings.HasSuffix(format, ": %s") && !strings.HasSuffix(format, ": %v") {
		return nil
	}

	return err
}

type simpleErr struct {
	msg string
	errors.Frame
}

func (e *simpleErr) Error() string {
	return Sprint(e)
}

func (e *simpleErr) Format(p errors.Printer) (next error) {
	p.Print(e.msg)
	e.Frame.Format(p)
	return nil
}

type withChain struct {
	// TODO: add frame information
	msg string
	err error
	errors.Frame
}

func (e *withChain) Error() string {
	return Sprint(e)
}

func (e *withChain) Format(p errors.Printer) (next error) {
	p.Print(e.msg)
	e.Frame.Format(p)
	return e.err
}

func (e *withChain) Unwrap() error {
	return e.err
}

func fmtError(p *pp, verb rune, err error) (handled bool) {
	var (
		sep = ": "
		w   = p
	)
	switch {
	case p.fmt.sharpV:
		if stringer, ok := p.arg.(GoStringer); ok {
			// Print the result of GoString unadorned.
			p.fmt.fmtS(stringer.GoString())
			return true
		}
		return false

	case p.fmt.plusV:
		sep = "\n--- "
		w.fmt.fmtFlags = fmtFlags{plusV: p.fmt.plusV} // only keep detail flag

		// The width or precision of a detailed view could be the number of
		// errors to print from a list.

	default:
		// Use an intermediate buffer in the rare cases that precision,
		// truncation, or one of the alternative verbs (q, x, and X) are
		// specified.
		switch verb {
		case 's', 'v':
			if (!w.fmt.widPresent || w.fmt.wid == 0) && !w.fmt.precPresent {
				break
			}
			fallthrough
		case 'q', 'x', 'X':
			w = newPrinter()
			defer w.free()
		default:
			w.badVerb(verb)
			return true
		}
	}

loop:
	for {
		w.fmt.inDetail = false
		switch v := err.(type) {
		case errors.Formatter:
			err = v.Format((*errPP)(w))
		// TODO: This case is for supporting old error implementations.
		// It may eventually disappear.
		case interface{ FormatError(errors.Printer) error }:
			err = v.FormatError((*errPP)(w))
		case Formatter:
			// Discard verb, but keep the flags. Discarding the verb prevents
			// nested quoting and other unwanted behavior. Preserving flags
			// recursively signals a request for detail, if interpreted as %+v.
			w.fmt.fmtFlags = p.fmt.fmtFlags
			if w.fmt.plusV {
				v.Format((*errPP)(w), 'v') // indent new lines
			} else {
				v.Format(w, 'v') // do not indent new lines
			}
			break loop
		default:
			w.fmtString(v.Error(), 's')
			break loop
		}
		if err == nil {
			break
		}
		w.buf.WriteString(sep)
	}

	if w != p {
		p.fmtString(string(w.buf), verb)
	}
	return true
}

// errPP wraps a pp to implement an errors.Printer. It keeps the ability to
// implement State so that the indenting functionality can be passed to
// errors that only implement fmt.Formatter.
type errPP pp

func (p *errPP) Width() (wid int, ok bool)      { return (*pp)(p).Width() }
func (p *errPP) Precision() (prec int, ok bool) { return (*pp)(p).Precision() }
func (p *errPP) Flag(c int) bool                { return (*pp)(p).Flag(c) }
func (p *errPP) WriteString(s string) (n int, err error) {
	return (*pp)(p).WriteString(s)
}

func (p *errPP) Write(b []byte) (n int, err error) {
	if !p.fmt.inDetail || p.fmt.plusV {
		if p.fmt.indent {
			b = bytes.Replace(b, []byte("\n"), []byte("\n    "), -1)
		}
		p.buf.Write(b)
	}
	return len(b), nil
}

func (p *errPP) Print(args ...interface{}) {
	if !p.fmt.inDetail || p.fmt.plusV {
		if p.fmt.indent {
			Fprint(p, args...)
		} else {
			(*pp)(p).doPrint(args)
		}
	}
}

func (p *errPP) Printf(format string, args ...interface{}) {
	if !p.fmt.inDetail || p.fmt.plusV {
		if p.fmt.indent {
			Fprintf(p, format, args...)
		} else {
			(*pp)(p).doPrintf(format, args)
		}
	}
}

func (p *errPP) Detail() bool {
	p.fmt.indent = p.fmt.plusV
	p.fmt.inDetail = true
	p.Write([]byte("\n"))
	return p.fmt.plusV
}
