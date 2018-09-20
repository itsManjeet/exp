// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errors_test

import (
	"testing"

	"golang.org/x/exp/errors"
)

func TestIs(t *testing.T) {
	err1 := errors.New("1")
	erra := errors.Annotate(err1, "2")
	errp := errors.NewPredicate("predicate(err==1)", func(err error) bool {
		return err == err1
	})
	for _, e := range []error{err1, erra, errp} {
		if !errors.Is(erra, e) {
			t.Errorf("Is(%v, %v) = false, want true", err1, e)
		}
	}
	err3 := errors.New("3")
	for _, e := range []error{err1, erra, errp} {
		if errors.Is(e, err3) {
			t.Errorf("Is(%v, %v) = true, want false", e, err3)
		}
	}
}

func TestAs(t *testing.T) {
	err := errors.Annotate(errorT{}, "annotation")
	e, ok := errors.As(err, errorT{})
	if !ok {
		t.Errorf("As(Annotate(errorT{}, ...), errorT{}) = _, false; want errorT, true")
	} else if _, ok := e.(errorT); !ok {
		t.Errorf("As(Annotate(errorT{}, ...), errorT{}) = %T, true; want errorT, true", e)
	}
}

type errorT struct{}

func (errorT) Error() string { return "errorT" }
