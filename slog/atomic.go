// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slog

import "sync/atomic"

type atomicBool struct {
	b int32
}

func (a *atomicBool) get() bool {
	return atomic.LoadInt32(&a.b) != 0
}

func (a *atomicBool) set(b bool) {
	var i int32
	if b {
		i = 1
	}
	atomic.StoreInt32(&a.b, i)
}

type atomicValue[T any] struct {
	a atomic.Value
}

func (av *atomicValue[T]) get() (z T) {
	v := av.a.Load()
	if v == nil {
		return z
	}
	return v.(T)
}

func (av *atomicValue[T]) set(x T) {
	av.a.Store(x)
}
