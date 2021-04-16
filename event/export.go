// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package event

import (
	"context"
	"sync/atomic"
)

// Exporter is a the type for something that exports events as they occur.
type Exporter interface {
	// Export is called for each event delivered to the system.
	// It is called inline, and should return quickly.
	Export(*Event)
}

// contextKey is used as the key for storing a contextValue on the context.
type contextKey struct{}

// contextValue is stored by value in the context to track the exporter and
// current parent event.
type contextValue struct {
	exporter Exporter
	parent   uint64
}

var (
	activeExporters int32  // used atomically to shortcut the entire system
	lastEvent       uint64 // used atomically go generate new event IDs
)

// WithExporter returns a context with the exporter attached.
// The exporter is called synchronously from the event call site, so it should
// return quickly so as not to hold up user code.
func WithExporter(ctx context.Context, e Exporter) context.Context {
	atomic.StoreInt32(&activeExporters, 1)
	return context.WithValue(ctx, contextKey{}, contextValue{exporter: e})
}

// FromContext returns the exporter and parent event set on the supplied
// context.
func FromContext(ctx context.Context) (Exporter, uint64) {
	b := To(ctx)
	e := b.Exporter
	var p uint64
	if b.Event != nil {
		p = b.Event.Parent
	}
	eventPool.Put(b.Event)
	return e, p
}

// Disable turns off the exporters, until the next WithExporter call.
func Disable() {
	atomic.StoreInt32(&activeExporters, 0)
}
