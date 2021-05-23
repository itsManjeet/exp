// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !disable_events

package event_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/exp/event"
)

func check(t *testing.T, e *event.Event, want []event.Label) {
	t.Helper()
	if got := e.Labels; !cmp.Equal(got, want, cmp.Comparer(valueEqual)) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func valueEqual(l1, l2 event.Value) bool {
	return fmt.Sprint(l1) == fmt.Sprint(l2)
}

func TestTraceBuilder(t *testing.T) {
	// Verify that the context returned from the handler is also returned from Start,
	// and is the context passed to End.
	ctx := event.WithExporter(context.Background(), event.NewExporter(&testTraceHandler{t}))
	ctx, end := event.Trace(ctx).Start("s")
	val := ctx.Value("x")
	if val != 1 {
		t.Fatal("context not returned from Start")
	}
	end()
}

type testTraceHandler struct {
	t *testing.T
}

func (*testTraceHandler) Start(ctx context.Context, _ *event.Event) context.Context {
	return context.WithValue(ctx, "x", 1)
}

func (t *testTraceHandler) End(ctx context.Context, _ *event.Event) {
	val := ctx.Value("x")
	if val != 1 {
		t.t.Fatal("Start context not passed to End")
	}
}

func TestDefaultNamespace(t *testing.T) {
	// Verify that NewBuilder's default namespace is the import path of its
	// caller's package.
	b := event.NewBuilder("")
	want := "golang.org/x/exp/event_test"
	if b.Namespace != want {
		t.Errorf("got %q, want %q", b.Namespace, want)
	}
}
