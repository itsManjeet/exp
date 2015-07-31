// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package swizzle

// BGRA converts a pixel buffer between Go's RGBA and other systems' BGRA byte
// orders.
func BGRA(p []byte)
