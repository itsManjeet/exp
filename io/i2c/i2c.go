// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package i2c allows users to read from an write to a slave I2C device.
package i2c // import "golang.org/x/exp/io/i2c"

import (
	"fmt"

	"golang.org/x/exp/io/i2c/driver"
)

// Device represents an I2C device. Devices must be closed once
// they are no longer in use.
type Device struct {
	conn driver.Conn
}

// Addr represents an I2C address.
type Addr struct {
	addr   int
	tenbit bool
}

// NewAddr returns a 7-bit I2C address.
func NewAddr(addr int) Addr {
	return Addr{addr: addr}
}

// New10BitAddr returns a 10-bit I2C address.
func New10BitAddr(addr int) Addr {
	return Addr{addr: addr, tenbit: true}
}

// TOOD(jbd): Do we need higher level I2C packet writers and readers?
// TODO(jbd): Support bidirectional communication.
// TODO(jbd): How do we support 10-bit addresses and how to enable 10-bit on devfs?

// Read reads len(buf) bytes from the device.
func (d *Device) Read(buf []byte) error {
	// TODO(jbd): Support reading from a register.
	if err := d.conn.Read(buf); err != nil {
		return fmt.Errorf("error reading from device: %v", err)
	}
	return nil
}

// Write writes the buffer to the device. If it is required to write to a
// specific register, the register should be passed as the first byte in the
// given buffer.
func (d *Device) Write(buf []byte) (err error) {
	if err := d.conn.Write(buf); err != nil {
		return fmt.Errorf("error writing to the device: %v", err)
	}
	return nil
}

// Close closes the device and releases the underlying sources.
// All devices must be closed once they are no longer in use.
func (d *Device) Close() error {
	return d.conn.Close()
}

// Open opens an I2C device with the given I2C address on the specified bus.
// If tenbit is true, the address is treated as a 10-bit I2C address.
func Open(o driver.Opener, bus int, addr Addr) (*Device, error) {
	if o == nil {
		o = &Devfs{}
	}
	conn, err := o.Open(bus, addr.addr, addr.tenbit)
	if err != nil {
		return nil, err
	}
	return &Device{conn: conn}, nil
}
