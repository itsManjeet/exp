// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gpio

import "golang.org/x/exp/io/gpio/driver"

// Devfs is the GPIO driver implementation that uses DMA via /dev/mem.
type Devfs struct {
	// BaseAddr represents the base memory address of the BCM2835 controller.
	// Given the zero value, Devfs will try to find the correct value by looking
	// at /proc/device-tree/soc/ranges.
	BaseAddr uint64
}

func (d *Devfs) Open() (driver.Conn, error) {
	panic("not implemented")
}

type devfsConn struct{}

func (d *devfsConn) Value(pin int) (int, error) {
	panic("not implemented")
}

func (d *devfsConn) SetValue(pin int, v int) error {
	panic("not implemented")
}

func (d *devfsConn) SetDirection(pin int, dir driver.Direction) error {
	panic("not implemented")
}

func (d *devfsConn) SetActive(pin int, v int) error {
	panic("not implemented")
}

func (d *devfsConn) Close() error {
	panic("not implemented")
}
