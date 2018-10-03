// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

package fmt_test

import (
	"fmt"

	"golang.org/x/exp/errors"
)

func f() error {
	err := errors.New("baz flopped")
	err = errors.Annotate(err, "bar(nameserver 139)")
	return errors.Annotate(err, "foo")
}
func Example_formatting() {
	err := f()
	fmt.Println("Error:")
	fmt.Printf("%v\n", err)
	fmt.Println()
	fmt.Println("Detailed error:")
	fmt.Printf("%+v\n", err)
	// Output:
	// Error:
	// foo: bar(nameserver 139): baz flopped
	//
	// Detailed error:
	// foo
	//     format_example_test.go:16
	// --- bar(nameserver 139)
	//     format_example_test.go:15
	// --- baz flopped
	//     format_example_test.go:14
}
