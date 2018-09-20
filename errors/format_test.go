// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errors_test

import (
	"fmt"
	"testing"

	"golang.org/x/exp/errors"
)

func TestFormat(t *testing.T) {
	err := errors.Annotate(detailed{}, "can't adumbrate elephant")
	if got, want := fmt.Sprintf("%v", err), "can't adumbrate elephant: out of peanuts"; got != want {
		t.Errorf(`Sprintf("%%v", err) = %q; want %q`, got, want)
	}
}

func TestFormatDetailed(t *testing.T) {
	err := errors.Annotate(detailed{}, "can't adumbrate elephant")
	want := `can't adumbrate elephant
    format_test.go:22
--- out of peanuts
    the elephant is on strike
    and the monkeys are laughing`
	if got := fmt.Sprintf("%+v", err); got != want {
		t.Errorf(`Sprintf("%%+v", err) = %q; want %q`, got, want)
	}
}

type detailed struct{}

func (e detailed) Error() string                 { return "error" }
func (e detailed) Format(s fmt.State, verb rune) { errors.Format(e, s, verb) }

func (detailed) FormatError(p errors.Printer) (next error) {
	p.Print("out of peanuts")
	p.Detail()
	p.Print("the elephant is on strike\nand the monkeys are laughing")
	return nil
}
