// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package windriver

import (
	"syscall"
)

var (
	hDefaultIcon   syscall.Handle
	hDefaultCursor syscall.Handle
	hThisInstance  syscall.Handle
)

func initCommon() (err error) {
	hDefaultIcon, err = loadIcon(0, xIDI_APPLICATION)
	if err != nil {
		return err
	}
	hDefaultCursor, err = loadCursor(0, xIDC_ARROW)
	if err != nil {
		return err
	}
	// TODO(andlabs) hThisInstance
	return nil
}
