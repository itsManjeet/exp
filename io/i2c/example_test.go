// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package i2c_test

import (
	"golang.org/x/exp/io/i2c"
)

func Example_open() {
	d, err := i2c.Open(&i2c.Devfs{}, 1, i2c.NewAddr(0x39))
	if err != nil {
		panic(err)
	}
}
