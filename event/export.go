// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package event

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Handler is a the type for something that handles events as they occur.
type Handler interface {
	// Handle is called for each event delivered to the system.
	Handle(*Event)
}

// Exporter synchronizes the delivery of events to handlers.
type Exporter struct {
	mu      sync.Mutex
	handler Handler
}

// contextKey is used as the key for storing a contextValue on the context.
type contextKey struct{}

// contextValue is stored by value in the context to track the exporter and
// current parent event.
type contextValue struct {
	exporter *Exporter
	parent   uint64
}

var (
	activeExporters int32  // used atomically to shortcut the entire system
	lastEvent       uint64 // used atomically go generate new event IDs
)

// NewExporter creates an Exporter using the supplied handler.
// Event delivery is serialized to enable safe atomic handling.
// It also marks the event system as active.
func NewExporter(h Handler) *Exporter {
	atomic.StoreInt32(&activeExporters, 1)
	return &Exporter{handler: h}
}

// WithExporter returns a context with the exporter attached.
// The exporter is called synchronously from the event call site, so it should
// return quickly so as not to hold up user code.
func WithExporter(ctx context.Context, e *Exporter) context.Context {
	atomic.StoreInt32(&activeExporters, 1)
	return context.WithValue(ctx, contextKey{}, contextValue{exporter: e})
}

// Disable turns off the exporters, until the next WithExporter call.
func Disable() {
	atomic.StoreInt32(&activeExporters, 0)
}

// Start delivers a start event and also updates the context with the event id.
func Start(ctx context.Context, name string) context.Context {
	b := To(ctx)
	if b.exporter == nil {
		return ctx
	}
	v := contextValue{exporter: b.exporter}
	v.parent = b.Start(name)
	return context.WithValue(ctx, contextKey{}, v)
}

// deliver events to the underlying handler.
func (e *Exporter) deliver(ev *Event) uint64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	lastEvent++
	id := lastEvent
	ev.ID = id
	ev.At = time.Now()
	e.handler.Handle(ev)
	return id
}
