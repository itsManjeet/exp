// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errors_test

import (
	"os"
	"testing"

	"golang.org/x/exp/errors"
	"golang.org/x/exp/errors/fmt"
)

func TestIs(t *testing.T) {
	err1 := errors.New("1")
	erra := fmt.Errorf("wrap 2: %v", err1)
	errb := fmt.Errorf("wrap 3: %v", erra)
	erro := errors.Opaque(err1)
	errco := fmt.Errorf("opaque: %v", erro)

	err3 := errors.New("3")

	poser := &poser{"either 1 or 3", func(err error) bool {
		return err == err1 || err == err3
	}}

	testCases := []struct {
		err    error
		target error
		match  bool
	}{
		{nil, nil, false},
		{err1, nil, false},

		{err1, err1, true},
		{erra, err1, true},
		{errb, err1, true},

		{errco, erro, false},
		{errco, err1, false},
		{erro, erro, false},

		{err1, err3, false},
		{erra, err3, false},
		{errb, err3, false},

		{poser, err1, true},
		{poser, err3, true},

		{poser, erra, false},
		{poser, errb, false},
		{poser, erro, false},
		{poser, errco, false},
	}
	for _, tc := range testCases {
		if got := errors.Is(tc.err, tc.target); got != tc.match {
			t.Errorf("Is(%v, %v) = %v, want %v", tc.err, tc.target, got, tc.match)
		}
	}
}

type poser struct {
	msg string
	f   func(error) bool
}

func (p *poser) Error() string     { return p.msg }
func (p *poser) Is(err error) bool { return p.f(err) }

func TestAs(t *testing.T) {
	var errT errorT
	var errP *os.PathError
	_, errF := os.Open("non-existing")

	testCases := []struct {
		err    error
		target interface{}
		match  bool
	}{{
		fmt.Errorf("pittied the fool: %v", errorT{}),
		&errT,
		true,
	}, {
		errF,
		&errP,
		true,
	}}
	for _, tc := range testCases {
		name := fmt.Sprintf("As(Errorf(..., %v), %v)", tc.err, tc.target)
		t.Run(name, func(t *testing.T) {
			match := errors.As(tc.err, tc.target)
			if match != tc.match {
				t.Fatalf("match: got %v; want %v", match, tc.match)
			}
			if !match {
				return
			}
			if tc.target == nil {
				t.Fatalf("non-nil result after match")
			}
		})
	}
}

type errorT struct{}

func (errorT) Error() string { return "errorT" }
