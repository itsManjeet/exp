// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slog

import "sync/atomic"

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
