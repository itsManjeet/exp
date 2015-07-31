// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !amd64

package swizzle

// BGRA converts a pixel buffer between Go's RGBA and other systems' BGRA byte
// orders.
func BGRA(p []byte) {
	if len(p)%4 != 0 {
		return
	}
	for i := 0; i < len(p); i += 4 {
		p[i+0], p[i+2] = p[i+2], p[i+0]
	}
}
