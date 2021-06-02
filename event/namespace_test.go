// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package event_test

import (
	"context"
	"runtime"
	"testing"

	"golang.org/x/exp/event"
)

const thisImportPath = "golang.org/x/exp/event_test"

func TestNamespace(t *testing.T) {
	var h nsHandler
	ctx := event.WithExporter(context.Background(), event.NewExporter(&h))
	event.To(ctx).Log("msg")
	if got, want := h.ns, thisImportPath; got != want {
		t.Errorf("got namespace %q, want, %q", got, want)
	}
}

type nsHandler struct {
	ns string
}

func (h *nsHandler) Handle(ctx context.Context, e *event.Event) context.Context {
	h.ns = e.Namespace
	return ctx
}

func BenchmarkRuntimeCallers(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var pcs [1]uintptr
		_ = runtime.Callers(2, pcs[:])
	}
}

func BenchmarkCallersFrames(b *testing.B) {
	var pcs [1]uintptr
	n := runtime.Callers(2, pcs[:])
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		frames := runtime.CallersFrames(pcs[:n])
		frame, _ := frames.Next()
		_ = frame.Function //namespace(frame.Function)
	}
}

func TestStablePCs(t *testing.T) {
	// The pc is stable regardless of the call stack.
	pc1 := f()
	pc2 := g()
	if pc1 != pc2 {
		t.Fatal("pcs differ")
	}
	// We can recover frame information after the function has returned.
	frames := runtime.CallersFrames([]uintptr{pc1})
	frame, _ := frames.Next()
	want := thisImportPath + ".h"
	if got := frame.Function; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func f() uintptr {
	return h()
}

func g() uintptr {
	return h()
}

func h() uintptr {
	var pcs [1]uintptr
	runtime.Callers(1, pcs[:])
	return pcs[0]
}
