// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package spi_test

import "golang.org/x/exp/io/spi"

// Example illustrates a program that drives an APA-102 LED strip.
func Example() {
	dev, err := spi.Open("/dev/spidev0.1")
	if err != nil {
		panic(err)
	}
	defer dev.Close()

	if err := dev.SetMode(spi.Mode3); err != nil {
		panic(err)
	}
	if err := dev.SetSpeed(500000); err != nil {
		panic(err)
	}
	if err := dev.SetBitsPerWord(8); err != nil {
		panic(err)
	}
	if err := dev.Do([]byte{
		0, 0, 0, 0,
		0xff, 200, 0, 200,
		0xff, 200, 0, 200,
		0xe0, 200, 0, 200,
		0xff, 200, 0, 200,
		0xff, 8, 50, 0,
		0xff, 200, 0, 0,
		0xff, 0, 0, 0,
		0xff, 200, 0, 200,
		0xff, 0xff, 0xff, 0xff,
	}, 0); err != nil {
		panic(err)
	}
}
