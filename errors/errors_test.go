// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errors_test

import (
	"testing"

	"golang.org/x/exp/errors"
)

func TestNewEqual(t *testing.T) {
	// Different allocations should not be equal.
	if errors.New("abc") == errors.New("abc") {
		t.Errorf(`New("abc") == New("abc")`)
	}
	if errors.New("abc") == errors.New("xyz") {
		t.Errorf(`New("abc") == New("xyz")`)
	}

	// Same allocation should be equal to itself (not crash).
	err := errors.New("jkl")
	if err != err {
		t.Errorf(`err != err`)
	}
}

func TestErrorMethod(t *testing.T) {
	err := errors.New("abc")
	if err.Error() != "abc" {
		t.Errorf(`New("abc").Error() = %q, want %q`, err.Error(), "abc")
	}
}

func TestAnnotateUnwrap(t *testing.T) {
	err1 := errors.New("abc")
	err2 := errors.Annotate(err1, "123")
	if got, want := errors.Unwrap(err2), err1; got != want {
		t.Errorf(`Unwrap(Annotate(New("abc"), "123")) = %v, want %v`, got, want)
	}
}

func TestAnnotateErrorMethod(t *testing.T) {
	err1 := errors.New("abc")
	err2 := errors.Annotate(err1, "123")
	if got, want := err2.Error(), "123: abc"; got != want {
		t.Errorf(`Annotate(New("abc"), "123") = %q, want %q`, got, want)
	}
}

func TestNewPredicate(t *testing.T) {
	err1 := errors.New("1")
	err2 := errors.New("2")
	errp := errors.NewPredicate("predicate", func(err error) bool {
		return err == err1
	})
	if !errors.Is(errp, err1) {
		t.Errorf(`Is(NewPredicate(...), err1) = false, want true`)
	}
	if !errors.Is(err1, errp) {
		t.Errorf(`Is(err1, NewPredicate(...)) = false, want true`)
	}
	if errors.Is(errp, err2) {
		t.Errorf(`Is(NewPredicate(...), err1) = true, want false`)
	}
	if errors.Is(err2, errp) {
		t.Errorf(`Is(err1, NewPredicate(...)) = true, want false`)
	}
}
