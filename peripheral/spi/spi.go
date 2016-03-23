// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package spi allows users to read from and write to an SPI device.
package spi

import (
	"os"
	"syscall"
	"time"
	"unsafe"
)

const (
	Mode0 = 0x00
	Mode1 = 0x04
	Mode2 = 0x08
	Mode3 = 0x0C
)

const (
	magic = 107

	nrbits   = 8
	typebits = 8
	sizebits = 13
	dirbits  = 3

	nrshift   = 0
	typeshift = nrshift + nrbits
	sizeshift = typeshift + typebits
	dirshift  = sizeshift + sizebits

	none  = 0
	read  = 2
	write = 4
)

type Device struct {
	f           *os.File
	mode        uint8
	speedHz     uint32
	bitsPerWord uint8
}

type payload struct {
	tx          uint64
	rx          uint64
	length      uint32
	speedHz     uint32
	delay       uint16
	bitsPerWord uint8
	csChange    uint8
	txNBits     uint8
	rxNBits     uint8
	pad         uint16
}

// SetMode sets the SPI mode. SPI mode is a combination of polarity and phases.
// CPOL is the high order bit, CPHA is the low order. Pre-computed mode
// values are Mode0, Mode1, Mode2 and Mode3.
func (d *Device) SetMode(mode int) error {
	m := uint8(mode)
	if err := d.ioctl(ioc(write, magic, 1, 1), uintptr(unsafe.Pointer(&m))); err != nil {
		return err
	}
	d.mode = m
	return nil
}

// SetSpeed sets the maximum clock speed in Hz.
func (d *Device) SetSpeed(speedHz int) error {
	s := uint32(speedHz)
	if err := d.ioctl(ioc(write, magic, 4, 4), uintptr(unsafe.Pointer(&s))); err != nil {
		return err
	}
	d.speedHz = s
	return nil
}

// SetBitsPerWord sets how many bits it takes to represent a word.
// e.g. 8 represents 8-bit words.
func (d *Device) SetBitsPerWord(bits int) error {
	b := uint8(bits)
	if err := d.ioctl(ioc(write, magic, 3, 1), uintptr(unsafe.Pointer(&b))); err != nil {
		return err
	}
	d.bitsPerWord = b
	return nil
}

// Do does a duplex transmission to write to the SPI device and read
// len(buf) numbers of bytes.
func (d *Device) Do(buf []byte, delay time.Duration) error {
	v := buf[:]
	p := payload{
		tx:          uint64(uintptr(unsafe.Pointer(&v[0]))),
		rx:          uint64(uintptr(unsafe.Pointer(&v[0]))),
		length:      uint32(len(v)),
		speedHz:     d.speedHz,
		delay:       uint16(delay.Nanoseconds() / 1000),
		bitsPerWord: d.bitsPerWord,
	}
	// TODO: Rename Do as Transfer and provide bidirectional transfer.
	return d.ioctl(msgArg(1), uintptr(unsafe.Pointer(&p)))
}

// Open opens an SPI device by the specified name.
// The name must be the device name of the SPI bus,
// e.g. /dev/spidev0.1.
func Open(name string) (*Device, error) {
	// TODO(jbd): Don't depend on devfs. Allow multiple backends and
	// those who may depend on proprietary APIs. devfs backend
	// could be the default backend.
	f, err := os.OpenFile(name, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	return &Device{f: f}, nil
}

// Close closes the SPI device and releases the related resources.
func (d *Device) Close() error {
	return d.f.Close()
}

func ioc(dir, typ, nr, size uintptr) uintptr {
	return (dir << dirshift) | (typ << typeshift) | (nr << nrshift) | (size << sizeshift)
}

func msgArg(n uint32) uintptr {
	return uintptr(0x40006B00 + (n * 0x200000))
}

func (d *Device) ioctl(a1, a2 uintptr) error {
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL, d.f.Fd(), a1, a2,
	)
	if errno != 0 {
		return syscall.Errno(errno)
	}
	return nil
}
