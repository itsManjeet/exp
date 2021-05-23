// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package event

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// preallocateLabels controls the space reserved for labels in an event.
// Storing the first few labels directly in builders can avoid an allocation at
// all for the very common cases of simple events. The length needs to be large
// enough to cope with the majority of events but no so large as to cause undue
// stack pressure.
const preallocateLabels = 4

// Event holds the information about an event that occurred.
// It combines the event metadata with the user supplied labels.
type Event struct {
	Namespace string
	ID        uint64    // unique for this process id of the event
	Parent    uint64    // id of the parent event for this event
	At        time.Time // time at which the event is delivered to the exporter.
	Message   string
	Labels    []Label

	ctx      context.Context
	exporter *Exporter
	labels   [preallocateLabels]Label
}

var eventPool = sync.Pool{New: func() interface{} { return &Event{} }}

func newEvent(ctx context.Context, namespace string, exp *Exporter, parent uint64, labels []Label) *Event {
	e := eventPool.Get().(*Event)
	e.ctx = ctx
	e.Namespace = namespace
	e.exporter = exp
	e.Parent = parent
	e.Labels = append(e.labels[:0], labels...)
	return e
}

func (e *Event) done() {
	*e = Event{}
	eventPool.Put(e)
}

func (e *Event) With(l Label) *Event {
	if e == nil || e.exporter == nil {
		return nil
	}
	e.Labels = append(e.Labels, l)
	return e
}

func (e *Event) WithAll(ls ...Label) *Event {
	if e == nil || e.exporter == nil || len(ls) == 0 {
		return nil
	}
	e.Labels = append(e.Labels, ls...)
	return e
}

func (e *Event) Logf(template string, args ...interface{}) {
	if e == nil {
		return
	}
	e.log(fmt.Sprintf(template, args...))
}

func (e *Event) Log(message string) {
	if e == nil {
		return
	}
	e.log(message)
}

func (e *Event) log(message string) {
	if e.exporter != nil && e.exporter.log != nil {
		e.Message = message
		e.deliver(e.exporter.log.Log)
	}
	e.done()
}

func (e *Event) Metric() {
	if e == nil {
		return
	}
	if e.exporter != nil && e.exporter.metric != nil {
		e.deliver(e.exporter.metric.Metric)
	}
	e.done()
}

func (e *Event) Annotate() {
	if e == nil {
		return
	}
	if e.exporter != nil && e.exporter.annotate != nil {
		e.deliver(e.exporter.annotate.Annotate)
	}
	e.done()
}

func (e *Event) Start(name string) (ctx context.Context, end func()) {
	if e == nil {
		panic("Event.Start called with nil *Event (use Builder.Trace, not Builder.To, before calling Start)")
	}
	defer e.done()
	if e.exporter == nil || e.exporter.trace == nil {
		return e.ctx, func() {}
	}
	e.exporter.mu.Lock()
	defer e.exporter.mu.Unlock()
	e.Message = name
	e.exporter.prepare(e) // assigns ID
	ctx = newContext(e.ctx, e.exporter, e.ID)
	ctx = e.exporter.trace.Start(ctx, e)
	// Construct the end function to return.
	// It will create an end event and deliver it.
	// If we allocated the event here and the user never called end (which is
	// valid), then we'd "leak" the event. If that happened a lot it would be a
	// subtle allocation drag on the program. So save values into local
	// variables and construct the event inside the function.
	ns := e.Namespace
	exp := e.exporter
	parent := e.ID
	end = func() {
		// The end event's context is the one returned by the Start call.
		// Its parent is the start event.
		// It has no labels. (Perhaps it should have the builder's labels, as if
		// constructed by the same builder as the Start event? We could do that
		// by having an Event remember the length of the builder labels used to
		// create it, but we'd have to copy them outside of this function).
		ee := newEvent(ctx, ns, exp, parent, nil)
		ee.End()
	}
	return ctx, end
}

func (e *Event) End() {
	if e == nil {
		return
	}
	if e.exporter != nil && e.exporter.trace != nil {
		e.deliver(e.exporter.trace.End)
	}
	e.done()
}

func (e *Event) deliver(f func(context.Context, *Event)) {
	e.exporter.mu.Lock()
	defer e.exporter.mu.Unlock()
	e.exporter.prepare(e)
	f(e.ctx, e)
}

// LogHandler is a the type for something that handles log events as they occur.
type LogHandler interface {
	// Log indicates a logging event.
	Log(context.Context, *Event)
}

// MetricHandler is a the type for something that handles metric events as they
// occur.
type MetricHandler interface {
	// Metric indicates a metric record event.
	Metric(context.Context, *Event)
}

// AnnotateHandler is a the type for something that handles annotate events as
// they occur.
type AnnotateHandler interface {
	// Annotate reports label values at a point in time.
	Annotate(context.Context, *Event)
}

// TraceHandler is a the type for something that handles start and end events as
// they occur.
type TraceHandler interface {
	// Start indicates a trace start event.
	Start(context.Context, *Event) context.Context
	// End indicates a trace end event.
	End(context.Context, *Event)
}

// WithExporter returns a context with the exporter attached.
// The exporter is called synchronously from the event call site, so it should
// return quickly so as not to hold up user code.
func WithExporter(ctx context.Context, e *Exporter) context.Context {
	return newContext(ctx, e, 0)
}

// SetDefaultExporter sets an exporter that is used if no exporter can be
// found on the context.
func SetDefaultExporter(e *Exporter) {
	setDefaultExporter(e)
}

// TODO: temporary, for tests; remove.
func To(ctx context.Context) *Event {
	return NewBuilder("test").To(ctx)
}

// TODO: temporary, for tests; remove.
func Trace(ctx context.Context) *Event {
	return NewBuilder("test").Trace(ctx)
}
