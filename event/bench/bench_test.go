// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bench_test

import (
	"context"
	"io/ioutil"
	"sync"
	"testing"
)

type Hooks struct {
	AStart func(ctx context.Context, a int) context.Context
	AEnd   func(ctx context.Context)
	BStart func(ctx context.Context, b string) context.Context
	BEnd   func(ctx context.Context)
}

var (
	initialList = []int{0, 1, 22, 333, 4444, 55555, 666666, 7777777}
	stringList  = []string{
		"A value",
		"Some other value",
		"A nice longer value but not too long",
		"V",
		"",
		"ı",
		"prime count of values",
	}
)

const (
	aName = "A"
	aMsg  = "A"
	aMsgf = aMsg + " where a=%d"
	bName = "B"
	bMsg  = "b"
	bMsgf = bMsg + " where b=%q"
)

type namedBenchmark struct {
	name string
	test func(ctx context.Context) func(*testing.B)
}

func benchA(ctx context.Context, hooks Hooks, a int) int {
	ctx = hooks.AStart(ctx, a)
	defer hooks.AEnd(ctx)
	return benchB(ctx, hooks, a, stringList[a%len(stringList)])
}

func benchB(ctx context.Context, hooks Hooks, a int, b string) int {
	ctx = hooks.BStart(ctx, b)
	defer hooks.BEnd(ctx)
	return a + len(b)
}

func runBenchmark(b *testing.B, ctx context.Context, hooks Hooks) {
	b.ReportAllocs()
	b.ResetTimer()
	var acc int
	for i := 0; i < b.N; i++ {
		for _, value := range initialList {
			acc += benchA(ctx, hooks, value)
		}
	}
}

type syncDiscardWriter struct {
	mu sync.Mutex
}

func (w *syncDiscardWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return ioutil.Discard.Write(b)
}

func (w *syncDiscardWriter) Sync() error { return nil }
