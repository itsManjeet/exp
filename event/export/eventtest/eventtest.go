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
	"bytes"
	"context"
	"sync"
	"testing"

	"golang.org/x/exp/event"
)

// NewContext returns a context you should use for the active test.
func NewContext(ctx context.Context, tb testing.TB) context.Context {
	e := &testExporter{tb: tb, buffer: &bytes.Buffer{}}
	e.printer = event.NewPrinter(e.buffer)
	return event.WithExporter(ctx, e)
}

type testExporter struct {
	mu      sync.Mutex
	tb      testing.TB
	buffer  *bytes.Buffer
	printer event.Printer
}

func (w *testExporter) Export(ctx context.Context, ev event.Event) {
	w.mu.Lock()
	defer w.mu.Unlock()
	// build our log message in buffer
	w.printer.Event(ev)
	// log to the testing.TB
	if w.buffer.Len() > 0 {
		w.tb.Log(w.buffer)
	}
	w.buffer.Truncate(0)
}
