// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package driver // import "golang.org/x/exp/io/spi/driver"

import "time"

type Driver func(bus, chip int) (Conn, error)

type Conn interface {
	Configure(mode, bits, speed int) error
	Transfer(tx, rx []byte, delay time.Duration) error
	Close() error
}
