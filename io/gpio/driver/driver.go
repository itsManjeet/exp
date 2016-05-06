// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package driver contains interfaces that needs to be implemented by
// various GPIO implementations.
package driver

// Opener is an interface to be implemented by GPIO drivers.
type Opener interface {
	Open() (Conn, error)
}

// Conn represents an open GPIO connection. Each driver should implement
// this interface to provide a full implementation of the GPIO protocol.
type Conn interface {
	// Value returns the value of the pin. 0 for low values, 1 for high.
	Value(pin int) (int, error)

	// SetValue sets the value of the pin. 0 for low values, 1 for high.
	SetValue(pin int, v int) error

	// SetDirection sets the direction of the pin. It could be either "in" or "out".
	SetDirection(pin int, dir string) error

	// SetActiveLow configures the GPIO polarity for the pin. Given true, pin
	// becomes active low; otherwise active high.
	SetActiveLow(pin int, low bool) error

	// Close closes the connection and frees the underlying resources.
	Close() error
}
