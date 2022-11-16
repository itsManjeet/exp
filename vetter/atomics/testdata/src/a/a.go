// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains tests for the atomics checker.

package a

import (
	"sync/atomic"
)

func consistent() {
	var x int32
	_ = atomic.AddInt32(&x, 123)
	if atomic.LoadInt32(&x) != 123 {
		panic("expected a value of 123")
	}
}

func first() {
	var x int32
	_ = atomic.AddInt32(&x, 1) // want `variable x is inconsistently used in sync/atomic functions`
	_ = atomic.AddInt32(&x, 1) // only first location is reported
	print(x)
}

func field() {
	var x struct{ f int32 }
	_ = atomic.AddInt32(&x.f, 1) // want `field f is inconsistently used in sync/atomic functions`
	print(x.f)
}

func embeddedField() {
	type emb struct{ f int32 }
	var x struct{ emb }
	_ = atomic.AddInt32(&x.f, 1) // want `field f is inconsistently used in sync/atomic functions`
	print(x.f)
}

var g int64

func incG() {
	atomic.AddInt64(&g, int64(1)) // want `variable g is inconsistently used in sync/atomic functions`
}
func readG() int64 {
	return g
}

func compositeTrueNegative() {
	type t struct{ f int32 }
	x := t{f: 10}
	atomic.AddInt32(&x.f, 1) // no report expected
}

func compositeFalseNegative() {
	type t struct{ f int32 }
	var x t
	atomic.AddInt32(&x.f, 1)
	x = t{f: 10} // This should be reported. Current FP suppression excludes this.
	x = t{}      // This could also be reported.
}
