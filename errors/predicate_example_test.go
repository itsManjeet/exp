// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errors_test

import (
	"fmt"

	"golang.org/x/exp/errors"
)

// Sentinal errors for various testable conditions.
var (
	ErrRejected = errors.New("can't be done")
	ErrAsleep   = errors.New("restin' my eyes")
	ErrVacation = errors.New("gone fishin'")

	// ErrRetriable is an error matches several more specific conditions.
	ErrRetriable = errors.NewPredicate("retriable error", isRetriable)
)

func isRetriable(err error) bool {
	return err == ErrAsleep || err == ErrVacation
}

func Do(tries int) error {
	switch tries {
	case 0:
		return ErrAsleep
	case 1:
		return ErrVacation
	default:
		return ErrRejected
	}
}

func Example_predicates() {
	tries := 0
	for {
		if err := Do(tries); errors.Is(err, ErrRetriable) {
			tries++
			fmt.Printf("retriable error: %v\n", err)
			continue
		} else if err != nil {
			fmt.Printf("permanent error: %v\n", err)
		}
		return
	}
	// Output:
	// retriable error: restin' my eyes
	// retriable error: gone fishin'
	// permanent error: can't be done
}
