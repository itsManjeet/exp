// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package event

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Builder is a fluent builder for construction of new events.
type Builder struct {
	Context  context.Context
	Exporter Exporter
	Event    *Event
}

var eventPool = sync.Pool{New: func() interface{} { return &Event{} }}

// To initializes a builder from the values stored in a context.
func To(ctx context.Context) Builder {
	b := Builder{Context: ctx}
	if atomic.LoadInt32(&activeExporters) == 0 {
		return b
	}
	v, ok := ctx.Value(contextKey{}).(contextValue)
	if !ok {
		return b
	}
	b.Exporter = v.exporter
	if b.Exporter != nil {
		b.Event = eventPool.Get().(*Event)
		*b.Event = Event{
			Parent: v.parent,
			ID:     atomic.AddUint64(&lastEvent, 1),
		}
	}
	return b
}

// With adds a new label to the event being constructed.
func (b Builder) With(label Label) Builder {
	if b.Event == nil {
		return b
	}
	//the loop is manually unrolled otherwise the inliner fails
	if b.Event.Static[0].key == nil {
		b.Event.Static[0] = label
		return b
	}
	if b.Event.Static[1].key == nil {
		b.Event.Static[1] = label
		return b
	}
	b.Event.Dynamic = append(b.Event.Dynamic, label)
	return b
}

// Error adds a new label to the event being constructed with key ErrorKey.
func (b Builder) Error(err error) Builder {
	return b.With(OfValue(ErrorKey{}, err))
}

// WithAll adds all the supplied labels to the event being constructed.
func (b Builder) WithAll(labels ...Label) Builder {
	if b.Event == nil {
		return b
	}
	if len(labels) == 0 {
		return b
	}
	if len(b.Event.Dynamic) == 0 {
		b.Event.Dynamic = labels
		return b
	}
	b.Event.Dynamic = append(b.Event.Dynamic, labels...)
	return b
}

// Deliver sends the constructed event to the exporter.
func (b Builder) Deliver(kind Kind, message string) {
	if b.Exporter == nil {
		return
	}
	b.Event.At = time.Now()
	b.Event.Kind = kind
	b.Event.Message = message
	b.Exporter.Export(b.Event)
	eventPool.Put(b.Event)
}

func (b Builder) Log(message string) {
	b.Deliver(LogKind, message)
}

func (b Builder) Logf(message string, args ...interface{}) {
	b.Deliver(LogKind, fmt.Sprintf(message, args...))
}

func (b Builder) Start(name string) context.Context {
	var ctx context.Context
	if b.Event == nil {
		return b.Context
	}
	if b.Context != nil {
		ctx = context.WithValue(b.Context, contextKey{}, contextValue{
			exporter: b.Exporter,
			parent:   b.Event.ID,
		})
	}
	b.Deliver(StartKind, name)
	return ctx
}

func (b Builder) End() {
	b.Deliver(EndKind, "")
}

func (b Builder) Metric() {
	b.Deliver(MetricKind, "")
}

func (b Builder) Annotate() {
	b.Deliver(AnnotateKind, "")
}

// ErrorKey is a key used to add error values to label lists.
type ErrorKey struct{}

func (k ErrorKey) Name() string { return "error" }

func (k ErrorKey) Print(p Printer, l Label) {
	p.String(l.UnpackValue().(error).Error())
}

func (k ErrorKey) From(l Label) (error, bool) {
	if l.Key() != k {
		return nil, false
	}
	err, _ := l.UnpackValue().(error)
	return err, true
}
