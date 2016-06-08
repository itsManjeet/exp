// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This is a mock implementation so developers using cross compilation
// can rely on local tools even though the real implementation isn't
// available on their platform.

// +build !linux

package i2c

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/exp/io/i2c/driver"
)

var (
	errNotImplemented = errors.New("not implemented")
)

type Devfs struct {
	Dev  string
	Addr int
}

const (
	i2c_SLAVE  = 0x0703
	i2c_TENBIT = 0x0704
)

func (d *Devfs) Open() (driver.Conn, error) {
	return &devfsConn{}, fmt.Errorf("not implemented on this platform")
}

type devfsConn struct {
	f *os.File
}

func (c *devfsConn) Tx(w, r []byte) error {
	panic(errNotImplemented)
}

func (c *devfsConn) Close() error {
	panic(errNotImplemented)
}

func (c *devfsConn) ioctl(arg1, arg2 uintptr) error {
	panic(errNotImplemented)
}
