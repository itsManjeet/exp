// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fmt

import (
	"bytes"
	coreerrors "errors"
	"strings"

	"golang.org/x/exp/errors"
)

func errorf(format string, a []interface{}) error {
	if len(a) > 0 {
		p := len(a) - 1
		err, ok := a[p].(error)
		if !ok {
			goto noWrap
		}

		if strings.HasSuffix(format, ": %s") || strings.HasSuffix(format, ": %v") {
			// TODO: this is not entirely correct. The error value could be
			// printed elsewhere in format if it mixes numbered with unnumbered
			// substitutions. With relatively small changes to doPrintf we can
			// have it optionally ignore extra arguments and pass the argument
			// list in its entirety.
			format = format[:len(format)-len(": %s")]
			return &withChain{Sprintf(format, a[:p]...), err}
		}

		// TODO: allow the pattern where the last argument is an unused error
		// variable, eliminating the need to write the substitution. This can
		// be done with a relatively simple change to doPrintf. Errorf would
		// then be equivalent to Annotate(msg string, err error).

		// TODO: should we allow special syntax to distinguish between creating
		// errors that are error.Wrappers and not? If not, should the returned
		// error implement errors.Wrapper in addition to errors.Formatter?
	}
noWrap:
	return coreerrors.New(Sprintf(format, a...))
}

type withChain struct {
	// TODO: add frame information
	msg string
	err error
}

func (e *withChain) Error() string {
	return Sprint(e)
}

func (e *withChain) Format(p errors.Printer) (next error) {
	p.Print(e.msg)
	return e.err
}

func fmtError(p *pp, verb rune, f errors.Formatter) {
	// TODO: handle #v: will be useful to be able to print error value.
	var (
		sep = ": "
		w   = p
	)
	if p.fmt.plusV {
		sep = "\n--- "
		w.fmt.fmtFlags = fmtFlags{} // ignore flags in detail view
		w.fmt.detail = true
	} else {
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
			return
		}
	}

	for {
		w.fmt.inDetail = false
		var ok bool
		if err := f.Format((*errPP)(w)); err == nil {
			break
		} else if f, ok = err.(errors.Formatter); !ok {
			w.buf.WriteString(sep)
			w.fmtString(err.Error(), 's')
			break
		}
		w.buf.WriteString(sep)
	}

	if w != p {
		p.fmtString(string(w.buf), verb)
	}
}

type errPP pp

func (p *errPP) Write(b []byte) (n int, err error) {
	if !p.fmt.inDetail || p.fmt.detail {
		if p.fmt.indent {
			b = bytes.Replace(b, []byte("\n"), []byte("\n    "), -1)
		}
		p.buf.Write(b)
	}
	return len(b), nil
}

func (p *errPP) Print(args ...interface{}) {
	if !p.fmt.inDetail || p.fmt.detail {
		if p.fmt.indent {
			Fprint(p, args...)
		} else {
			(*pp)(p).doPrint(args)
		}
	}
}

func (p *errPP) Printf(format string, args ...interface{}) {
	if !p.fmt.inDetail || p.fmt.detail {
		if p.fmt.indent {
			Fprintf(p, format, args...)
		} else {
			(*pp)(p).doPrintf(format, args)
		}
	}
}

func (p *errPP) Detail() bool {
	p.fmt.indent = p.fmt.detail
	p.fmt.inDetail = true
	p.Write([]byte("\n"))
	return p.fmt.detail
}
