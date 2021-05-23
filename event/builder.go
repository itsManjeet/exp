// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !disable_events

package event

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Namespace struct {
	Name   string
	Labels []Label
}

func NewNamespace(name string) *Namespace {
	if name == "" {
		var pcs [1]uintptr
		n := runtime.Callers(2 /* caller of NewNamespace */, pcs[:])
		frames := runtime.CallersFrames(pcs[:n])
		frame, _ := frames.Next()
		// Function is the fully-qualified function name. The name itself may
		// have dots (for a closure, for instance), but it can't have slashes.
		// So the package path ends at the first dot after the last slash.
		i := strings.LastIndexByte(frame.Function, '/')
		if i < 0 {
			i = 0
		}
		end := strings.IndexByte(frame.Function[i:], '.')
		if end >= 0 {
			end += i
		} else {
			end = len(frame.Function)
		}
		name = frame.Function[:end]
	}
	return &Namespace{Name: name}
}

func (n *Namespace) AddLabels(labels ...Label) {
	n.Labels = append(n.Labels, labels...)
}

// To initializes a builder from the values stored in a context.
func (n *Namespace) To(ctx context.Context) Builder {
	b := Builder{ctx: ctx, data: newBuilder(ctx, n.Name)}
	return b.WithAll(n.Labels...)
}

// Builder is a fluent builder for construction of new events.
type Builder struct {
	ctx  context.Context
	data *builder
}

// preallocateLabels controls the space reserved for labels in a builder.
// Storing the first few labels directly in builders can avoid an allocation at
// all for the very common cases of simple events. The length needs to be large
// enough to cope with the majority of events but no so large as to cause undue
// stack pressure.
const preallocateLabels = 4

type builder struct {
	exporter *Exporter
	Event    Event
	labels   [preallocateLabels]Label
}

var builderPool = sync.Pool{New: func() interface{} { return &builder{} }}

func newBuilder(ctx context.Context, namespace string) *builder {
	exporter, parent := fromContext(ctx)
	if exporter == nil {
		return nil
	}
	b := builderPool.Get().(*builder)
	b.exporter = exporter
	b.Event.Namespace = namespace
	b.Event.Labels = b.labels[:0]
	b.Event.Parent = parent
	return b
}

// With adds a new label to the event being constructed.
func (b Builder) With(label Label) Builder {
	if b.data != nil {
		b.data.Event.Labels = append(b.data.Event.Labels, label)
	}
	return b
}

// WithAll adds all the supplied labels to the event being constructed.
func (b Builder) WithAll(labels ...Label) Builder {
	if b.data == nil || len(labels) == 0 {
		return b
	}
	if len(b.data.Event.Labels) == 0 {
		b.data.Event.Labels = labels
	} else {
		b.data.Event.Labels = append(b.data.Event.Labels, labels...)
	}
	return b
}

func (b Builder) At(t time.Time) Builder {
	if b.data != nil {
		b.data.Event.At = t
	}
	return b
}

// Log is a helper that calls Deliver with LogKind.
func (b Builder) Log(message string) {
	if b.data == nil {
		return
	}
	if b.data.exporter.log != nil {
		b.log(message)
	}
	b.done()
}

// Logf is a helper that uses fmt.Sprint to build the message and then
// calls Deliver with LogKind.
func (b Builder) Logf(template string, args ...interface{}) {
	if b.data == nil {
		return
	}
	if b.data.exporter.log != nil {
		b.log(fmt.Sprintf(template, args...))
	}
	b.done()
}

func (b Builder) log(message string) {
	b.data.exporter.mu.Lock()
	defer b.data.exporter.mu.Unlock()
	b.data.Event.Message = message
	b.data.exporter.prepare(&b.data.Event)
	b.data.exporter.log.Log(b.ctx, &b.data.Event)
}

// Metric is a helper that calls Deliver with MetricKind.
func (b Builder) Metric() {
	if b.data == nil {
		return
	}
	if b.data.exporter.metric != nil {
		b.data.exporter.mu.Lock()
		defer b.data.exporter.mu.Unlock()
		b.data.exporter.prepare(&b.data.Event)
		b.data.exporter.metric.Metric(b.ctx, &b.data.Event)
	}
	b.done()
}

// Annotate is a helper that calls Deliver with AnnotateKind.
func (b Builder) Annotate() {
	if b.data == nil {
		return
	}
	if b.data.exporter.annotate != nil {
		b.data.exporter.mu.Lock()
		defer b.data.exporter.mu.Unlock()
		b.data.exporter.prepare(&b.data.Event)
		b.data.exporter.annotate.Annotate(b.ctx, &b.data.Event)
	}
	b.done()
}

// End is a helper that calls Deliver with EndKind.
func (b Builder) End() {
	if b.data == nil {
		return
	}
	if b.data.exporter.trace != nil {
		b.data.exporter.mu.Lock()
		defer b.data.exporter.mu.Unlock()
		b.data.exporter.prepare(&b.data.Event)
		b.data.exporter.trace.End(b.ctx, &b.data.Event)
	}
	b.done()
}

// Event returns a copy of the event currently being built.
func (b Builder) Event() *Event {
	clone := b.data.Event
	if len(b.data.Event.Labels) > 0 {
		clone.Labels = make([]Label, len(b.data.Event.Labels))
		copy(clone.Labels, b.data.Event.Labels)
	}
	return &clone
}

func (b Builder) done() {
	*b.data = builder{}
	builderPool.Put(b.data)
}

// Start delivers a start event with the given name and labels.
// Its second return value is a function that should be called to deliver the
// matching end event.
// All events created from the returned context will have this start event
// as their parent.
func (b Builder) Start(name string) (context.Context, func()) {
	if b.data == nil {
		return b.ctx, func() {}
	}
	ctx := b.ctx
	end := func() {}
	if b.data.exporter.trace != nil {
		b.data.exporter.mu.Lock()
		defer b.data.exporter.mu.Unlock()
		b.data.exporter.prepare(&b.data.Event)
		// create the end builder
		eb := Builder{}
		eb.data = builderPool.Get().(*builder)
		eb.data.exporter = b.data.exporter
		eb.data.Event.Parent = b.data.Event.ID
		// and now deliver the start event
		b.data.Event.Message = name
		ctx = newContext(ctx, b.data.exporter, b.data.Event.ID)
		ctx = b.data.exporter.trace.Start(ctx, &b.data.Event)
		eb.ctx = ctx
		end = eb.End
	}
	b.done()
	return ctx, end
}
