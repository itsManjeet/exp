// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package eventtest supports logging events to a test.
// You can use NewContext to create a context that knows how to deliver
// telemetry events back to the test.
// You must use this context or a derived one anywhere you want telemetry to be
// correctly routed back to the test it was constructed with.
// Any events delivered to a background context will be dropped.
//
// Importing this package will cause it to register a new global telemetry
// exporter that understands the special contexts returned by NewContext.
// This means you should not import this package if you are not going to call
// NewContext.
package eventtest

import (
	"context"
	"strings"
	"testing"
	"time"

	"golang.org/x/exp/event"
)

// NewContext returns a context you should use for the active test.
func NewContext(ctx context.Context, tb testing.TB) context.Context {
	h := &testHandler{tb: tb}
	h.printer = event.NewPrinter(&h.buf)
	return event.WithExporter(ctx, event.NewExporter(h))
}

type testHandler struct {
	tb      testing.TB
	printer event.Printer
	buf     strings.Builder
}

func (w *testHandler) Handle(ev *event.Event) {
	// build our log message in buffer
	w.buf.Reset()
	w.printer.Handle(ev)
	// log to the testing.TB
	msg := w.buf.String()
	if len(msg) > 0 {
		w.tb.Log(msg)
	}
}

// Rewriter wraps an exporter with one that makes the times and events stable.
func Rewriter(wrap event.Handler) event.Handler {
	exporter := &rewriter{
		wrapped: wrap,
		ids:     make(map[uint64]uint64),
	}
	exporter.nextTime, _ = time.Parse(time.RFC3339Nano, "2020-03-05T14:27:48Z")
	return exporter
}

type rewriter struct {
	wrapped  event.Handler
	nextID   uint64
	nextTime time.Time
	ids      map[uint64]uint64
	event    event.Event
}

func (e *rewriter) Handle(ev *event.Event) {
	// rewrite the time to normalize it
	e.event = *ev
	// remap and advance the time
	e.event.At = e.nextTime
	e.nextTime = e.nextTime.Add(time.Second)
	// remap the parent id if present
	if e.event.Parent != 0 {
		e.event.Parent = e.ids[e.event.Parent]
	}
	// rewrite the id to be per exporter rather than per process
	e.nextID++
	e.ids[e.event.ID] = e.nextID
	e.event.ID = e.nextID
	// and export the rewritten event
	e.wrapped.Handle(&e.event)
}
