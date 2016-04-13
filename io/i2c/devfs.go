// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package i2c

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/exp/io/i2c/driver"
)

// Devfs is an I2C driver that works against the devfs.
// You need to load the "i2c-dev" kernel module to use this driver.
type Devfs struct{}

const (
	i2c_SMBUS_READ           = 1
	i2c_SMBUS_WRITE          = 0
	i2c_SMBUS_I2C_BLOCK_DATA = 8

	i2c_SMBUS = 0x0720

	i2c_SLAVE = 0x0703 // TODO(jbd): Allow users to use I2C_SLAVE_FORCE?
)

type i2c_smbus_ioctl_data struct {
	readwrite uint8
	command   uint8
	size      int
	data      uintptr
}

// TODO(jbd): Support I2C_RETRIES and I2C_TIMEOUT at the driver and implementation level.

func (d *Devfs) Open(bus, addr int) (driver.Conn, error) {
	f, err := os.OpenFile(fmt.Sprintf("/dev/i2c-%d", bus), os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	conn := &devfsConn{f: f}
	if err := conn.ioctl(i2c_SLAVE, uintptr(addr)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("error opening the address (%v) on the bus (%v): %v", addr, bus, err)
	}
	return conn, nil
}

type devfsConn struct {
	f *os.File
}

func (c *devfsConn) Read(buf []byte) (int, error) {
	return c.f.Read(buf)
}

func (c *devfsConn) Write(cmd byte, buf []byte) error {
	size := len(buf)
	buffer := append([]byte{byte(size)}, buf...)
	data := i2c_smbus_ioctl_data{
		readwrite: i2c_SMBUS_WRITE,
		command:   cmd,
		size:      i2c_SMBUS_I2C_BLOCK_DATA,
		data:      uintptr(unsafe.Pointer(&buffer[0])),
	}
	return c.ioctl(i2c_SMBUS, uintptr(unsafe.Pointer(&data)))
}

func (c *devfsConn) Close() error {
	return c.f.Close()
}

func (c *devfsConn) ioctl(arg1, arg2 uintptr) error {
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, c.f.Fd(), arg1, arg2); errno != 0 {
		return syscall.Errno(errno)
	}
	return nil
}
