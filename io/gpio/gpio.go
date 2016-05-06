// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package gpio allows users to communicate with GPIO pins.
package gpio

import "golang.org/x/exp/io/gpio/driver"

type Device struct {
	conn driver.Conn
}

// Direction determines the direction of the pin. A pin could be
// configured to be an input or an output.
type Direction string

const (
	In  = Direction("in")
	Out = Direction("out")
	// TODO(jbd): Out but initially high or initially low?
)

// TODO(jbd): How to support analog pins?

// TODO(jbd): Allow users to configure edge trigger type.

// Open opens a connection to the GPIO device with the given driver.
// If a nil driver is provided, it uses the Devfs implementation with
// the default settings.
// Opened devices should be closed by calling Close.
func Open(d driver.Opener) (*Device, error) {
	// TODO(jbd): Open pin rather than GPIO device? It would help
	// some driver implementations such as sysfs.
	if d == nil {
		d = &Devfs{}
	}
	conn, err := d.Open()
	if err != nil {
		return nil, err
	}
	return &Device{conn: conn}, nil
}

// Value returns the value for the pin. 0 for low, 1 for high values.
func (d *Device) Value(pin int) (int, error) {
	return d.conn.Value(pin)
}

// SetValue sets the value of the pin. 0 for low, 1 for high values.
func (d *Device) SetValue(pin int, v int) error {
	return d.conn.SetValue(pin, int(v))
}

// SetDirection configures the direction of the pin.
func (d *Device) SetDirection(pin int, dir Direction) error {
	return d.conn.SetDirection(pin, string(dir))
}

// SetActiveLow configures the GPIO polarity of the pin. Given true,
// the pin will be configured to be active low, otherwise active high.
func (d *Device) SetActiveLow(pin int, low bool) error {
	return d.conn.SetActiveLow(pin, low)
}

// Close closes the device and frees the resources.
func (d *Device) Close() error {
	return d.conn.Close()
}
