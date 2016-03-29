// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package driver contains interfaces to be implemented by various SPI implementations.
package driver // import "golang.org/x/exp/io/spi/driver"

import "time"

// Opener is a function to be implemented by the SPI driver to open
// a connection an SPI device with the specified bus and chip number.
type Opener func(bus, chip int) (Conn, error)

// Conn is a connection to an SPI device.
type Conn interface {
	// Configure configures the SPI mode, bits per word and max clock
	// speed to be used. SPI device can override these values.
	Configure(mode, bits, speed int) error

	// Transfer transfers tx and reads into rx.
	Transfer(tx, rx []byte, delay time.Duration) error

	// Close frees the underlying resources and closes the connection.
	Close() error
}
