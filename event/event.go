// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package event

import (
	"context"
	"sync"
	"time"
)

// Event holds the information about an event that occurred.
// It combines the event metadata with the user supplied labels.
type Event struct {
	TraceID   uint64
	Parent    uint64    // id of the parent event for this event
	Namespace string    // namespace of event; if empty, set by exporter to import path
	At        time.Time // time at which the event is delivered to the exporter.
	Kind      Kind
	Message   string
	Name      string
	Error     error
	Labels    []Label

	ctx    context.Context
	target *Target
	labels [preallocateLabels]Label
}

// Handler is a the type for something that handles events as they occur.
type Handler interface {
	// Event is called with each event.
	Event(context.Context, *Event) context.Context
}

//TODO: work out what we do with prototypes
type Prototype struct {
	labels []Label
}

// preallocateLabels controls the space reserved for labels in a builder.
// Storing the first few labels directly in builders can avoid an allocation at
// all for the very common cases of simple events. The length needs to be large
// enough to cope with the majority of events but no so large as to cause undue
// stack pressure.
const preallocateLabels = 6

var eventPool = sync.Pool{New: func() interface{} { return &Event{} }}

// WithExporter returns a context with the exporter attached.
// The exporter is called synchronously from the event call site, so it should
// return quickly so as not to hold up user code.
func WithExporter(ctx context.Context, e *Exporter) context.Context {
	return newContext(ctx, e, 0, time.Time{})
}

// SetDefaultExporter sets an exporter that is used if no exporter can be
// found on the context.
func SetDefaultExporter(e *Exporter) {
	setDefaultExporter(e)
}

// New prepares a new event.
// This is intended to avoid allocations in the steady state case, to do this
// it uses a pool of events.
// Events are returned to the pool when Deliver is called.
// It returns nil if there is no active exporter for this kind of event.
func New(ctx context.Context, kind Kind) *Event {
	var target *Target
	if v, ok := ctx.Value(contextKey).(*Target); ok {
		target = v
	} else {
		target = getDefaultTarget()
	}
	if target == nil {
		return nil
	}
	//TODO: check if kind is enabled
	ev := eventPool.Get().(*Event)
	*ev = Event{
		ctx:    ctx,
		target: target,
		Kind:   kind,
		Parent: target.parent,
	}
	ev.Labels = ev.labels[:0]
	return ev
}

// Deliver the event to the exporter that was found in New.
// This also returns the event to the pool, it is an error to do anything
// with the event after it is delivered.
func (ev *Event) Deliver() context.Context {
	// get the event ready to send
	ev.target.exporter.prepare(ev)
	// now hold the lock while we deliver the event
	ev.target.exporter.mu.Lock()
	defer ev.target.exporter.mu.Unlock()
	ctx := ev.target.exporter.handler.Event(ev.ctx, ev)
	eventPool.Put(ev)
	return ctx
}

func (p Prototype) Label(label Label) Prototype {
	//TODO: do we need to clone the slice?
	p.labels = append(p.labels, label)
	return p
}

func (p Prototype) Apply(ev *Event) {
	ev.Labels = append(ev.Labels, p.labels...)
}
