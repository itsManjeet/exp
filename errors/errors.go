// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package errors implements functions to manipulate errors.
//
// This package implements the Go 2 draft designs for error inspection and printing:
//   https://go.googlesource.com/proposal/+/master/design/go2draft.md
//
// This is an EXPERIMENTAL package, and may change in arbitrary ways without notice.
package errors

import (
	"fmt"
)

type errorString struct {
	s      string
	caller Stack
}

// New returns an error that formats as the given text.
func New(text string) error {
	return &errorString{
		s:      text,
		caller: NewStack(),
	}
}

func (e *errorString) Error() string {
	return e.s
}

func (e *errorString) FormatError(p Printer) (next error) {
	p.Print(e.s)
	e.caller.FormatError(p)
	return nil
}

func (e *errorString) Format(s fmt.State, verb rune) {
	Format(e, s, verb)
}

type errorAnnotation struct {
	s      string
	err    error
	caller Stack
}

// Annotate annotates an error with the given text.
func Annotate(err error, text string) error {
	return &errorAnnotation{
		s:      text,
		err:    err,
		caller: NewStack(),
	}
}

func (e *errorAnnotation) Error() string {
	return e.s + ": " + e.err.Error()
}

func (e *errorAnnotation) FormatError(p Printer) (next error) {
	p.Print(e.s)
	e.caller.FormatError(p)
	return e.err
}

func (e *errorAnnotation) Format(s fmt.State, verb rune) {
	Format(e, s, verb)
}

func (e *errorAnnotation) Unwrap() error {
	return e.err
}

type errorPredicate struct {
	s      string
	f      func(error) bool
	caller Stack
}

// NewPredicate returns an error that is equivalent to any error for which
// f returns true
func NewPredicate(text string, f func(error) bool) error {
	return &errorPredicate{
		s:      text,
		f:      f,
		caller: NewStack(),
	}
}

func (e *errorPredicate) Error() string {
	return e.s
}

func (e *errorPredicate) FormatError(p Printer) (next error) {
	p.Print(e.s)
	e.caller.FormatError(p)
	return nil
}

func (e *errorPredicate) Is(target error) bool {
	return e.f(target)
}
