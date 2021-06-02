// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package eventtest supports logging events to a test.
// You can use NewContext to create a context that knows how to deliver
// telemetry events back to the test.
// You must use this context or a derived one anywhere you want telemetry to be
// correctly routed back to the test it was constructed with.
package eventtest

import (
	"context"
	"os"
	"testing"
	"time"

	"golang.org/x/exp/event"
	"golang.org/x/exp/event/adapter/logfmt"
)

// NewContext returns a context you should use for the active test.
func NewContext(ctx context.Context, tb testing.TB) context.Context {
	h := &testHandler{tb: tb}
	return event.WithExporter(ctx, event.NewExporter(h))
}

type testHandler struct {
	tb      testing.TB
	printer logfmt.Printer
}

func (h *testHandler) Handle(ctx context.Context, ev *event.Event) context.Context {
	//TODO: choose between stdout and stderr based on the event
	//TODO: decide if we should be calling h.tb.Fail()
	h.printer.Event(os.Stdout, ev)
	return ctx
}

// FixedNow updates the exporter in the context to use a time function returned
// by TestNow to make tests reproducible.
func FixedNow(ctx context.Context) {
	nextTime, _ := time.Parse(time.RFC3339Nano, "2020-03-05T14:27:48Z")
	e, _ := event.FromContext(ctx)
	e.Now = func() time.Time {
		thisTime := nextTime
		nextTime = nextTime.Add(time.Second)
		return thisTime
	}
}
