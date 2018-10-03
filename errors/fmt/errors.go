// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fmt

import (
	"bytes"

	experrors "golang.org/x/exp/errors"
)

func fmtError(p *pp, verb rune, f experrors.Formatter) {
	var (
		sep = ": "
		w   = p
	)
	if p.fmt.plusV {
		sep = "\n--- "
		// ignore flags in detail view
		w.fmt.fmtFlags = fmtFlags{}
		w.fmt.detail = true
	} else {
		// Use an intermediate buffer in the rare case that precision,
		// truncation, or one of the alternative verbs (q, +q, x, and X) is
		// specified.
		switch verb {
		case 's', 'v':
			if (!w.fmt.widPresent || w.fmt.wid == 0) && !w.fmt.precPresent {
				break
			}
			fallthrough
		case 'q', 'x', 'X':
			w = newPrinter()
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
		} else if f, ok = err.(experrors.Formatter); !ok {
			w.buf.WriteString(sep)
			w.fmtString(err.Error(), 's')
			break
		}
		w.buf.WriteString(sep)
	}

	if w != p {
		p.fmtString(string(w.buf), verb)
		w.free()
	}
}

type errPP pp

var nlSep = []byte("\n    ")

func (p *errPP) Write(b []byte) (n int, err error) {
	if !p.fmt.inDetail || p.fmt.detail {
		if p.fmt.indent {
			b = bytes.Replace(b, []byte("\n"), nlSep, -1)
		}
		p.buf.Write(b)
	}
	return len(b), nil
}

func (p *errPP) WriteString(s string) (n int, err error) {
	if !p.fmt.inDetail || p.fmt.detail {
		if p.fmt.indent {
			return p.Write([]byte(s))
		}
		p.buf.WriteString(s)
	}
	return len(s), nil
}

func (p *errPP) Print(args ...interface{}) {
	if !p.fmt.inDetail || p.fmt.detail {
		if p.fmt.indent {
			Fprint(p, args...)
		} else {
			((*pp)(p)).doPrint(args)
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
