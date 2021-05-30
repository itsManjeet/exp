// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !disable_events

package event

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Builder is a fluent builder for construction of new events.
type Builder struct {
	ctx      context.Context
	exporter *Exporter
	parent   uint64
	at       time.Time
	nLabels  int                      // number of labels
	pLabels  [preallocateLabels]Label // holds all labels when nLabels <= preallocateLabels
	aLabels  []Label                  // holds all labels when nLabels > preallocateLabels
}

// To initializes a builder from the values stored in a context.
func To(ctx context.Context) Builder {
	exporter, parent := fromContext(ctx)
	if exporter == nil {
		return Builder{ctx: ctx}
	}
	return Builder{
		ctx:      ctx,
		exporter: exporter,
		parent:   parent,
	}
}

// With adds a new label to the returned builder. All events delivered from it
// will have the label. The receiver is unchanged.
func (b Builder) With(l Label) Builder {
	if b.exporter == nil {
		return b
	}
	if b.nLabels < len(b.pLabels) {
		b.pLabels[b.nLabels] = l
	} else {
		if b.nLabels == len(b.pLabels) {
			// The line
			//    b.labels = append(b.plabels[:], l)
			// causes an allocation (presumably b escaping), even if it isn't
			// reached. So do the append manually.
			b.aLabels = make([]Label, len(b.pLabels), 2*len(b.pLabels))
			copy(b.aLabels, b.pLabels[:])
		}
		b.aLabels = append(b.aLabels, l)
	}
	b.nLabels++
	return b
}

// WithAll adds all the supplied labels to the event being constructed.
func (b Builder) WithAll(labels ...Label) Builder {
	if b.exporter == nil || len(labels) == 0 {
		return b
	}
	// TODO: optimize
	for _, l := range labels {
		b = b.With(l)
	}
	return b
}

func (b Builder) At(t time.Time) Builder {
	if b.exporter == nil {
		return b
	}
	b.at = t
	return b
}

// Log is a helper that calls Deliver with LogKind.
func (b Builder) Log(message string) {
	if b.exporter == nil || b.exporter.log == nil {
		return
	}
	b.log(message)
}

// Logf is a helper that uses fmt.Sprint to build the message and then
// calls Deliver with LogKind.
func (b Builder) Logf(template string, args ...interface{}) {
	if b.exporter == nil || b.exporter.log == nil {
		return
	}
	b.log(fmt.Sprintf(template, args...))
}

func (b Builder) log(message string) {
	e := newEvent(b.parent)
	defer freeEvent(e)
	e.Message = message
	b.prepare(e)
	b.exporter.mu.Lock()
	defer b.exporter.mu.Unlock()
	b.exporter.prepare(e)
	b.exporter.log.Log(b.ctx, e)
}

// Metric is a helper that calls Deliver with MetricKind.
func (b Builder) Metric() {
	if b.exporter == nil || b.exporter.metric == nil {
		return
	}
	e := newEvent(b.parent)
	defer freeEvent(e)
	b.prepare(e)
	b.exporter.mu.Lock()
	defer b.exporter.mu.Unlock()
	b.exporter.prepare(e)
	b.exporter.metric.Metric(b.ctx, e)
}

// Annotate is a helper that calls Deliver with AnnotateKind.
func (b Builder) Annotate() {
	if b.exporter == nil || b.exporter.annotate == nil {
		return
	}
	e := newEvent(b.parent)
	defer freeEvent(e)
	b.prepare(e)
	b.exporter.mu.Lock()
	defer b.exporter.mu.Unlock()
	b.exporter.prepare(e)
	b.exporter.annotate.Annotate(b.ctx, e)
}

// End is a helper that calls Deliver with EndKind.
func (b Builder) End() {
	if b.exporter == nil || b.exporter.trace == nil {
		return
	}
	e := newEvent(b.parent)
	defer freeEvent(e)
	b.prepare(e)
	b.exporter.mu.Lock()
	defer b.exporter.mu.Unlock()
	b.exporter.prepare(e)
	b.exporter.trace.End(b.ctx, e)
}

// Start delivers a start event with the given name and labels.
// Its second return value is a function that should be called to deliver the
// matching end event.
// All events created from the returned context will have this start event
// as their parent.
func (b Builder) Start(name string) (context.Context, func()) {
	if b.exporter == nil || b.exporter.trace == nil {
		return b.ctx, func() {}
	}
	exp := b.exporter
	se := newEvent(b.parent)
	defer freeEvent(se)
	se.Message = name
	b.prepare(se)
	exp.mu.Lock()
	defer exp.mu.Unlock()
	exp.prepare(se)
	ctx := newContext(b.ctx, exp, se.ID)
	ctx = exp.trace.Start(ctx, se)
	// Remember values to use in the end closure.
	var (
		pLabels [preallocateLabels]Label
		labels  []Label
	)
	nLabels := b.nLabels
	if nLabels <= len(b.pLabels) {
		copy(pLabels[:], b.pLabels[:b.nLabels])
	} else {
		labels = b.aLabels
	}
	parent := se.ID
	end := func() {
		exp.mu.Lock()
		defer exp.mu.Unlock()
		ee := newEvent(parent)
		defer freeEvent(ee)
		// Don't call b.prepare; we don't want to capture b.
		// Ignore b.at; use exp.Now.
		// Use the same labels as the start event.
		if nLabels <= len(pLabels) {
			ee.pLabels = pLabels
			ee.Labels = ee.pLabels[:nLabels]
		} else {
			ee.Labels = labels
		}
		exp.prepare(ee)
		exp.trace.End(ctx, ee)
	}
	return ctx, end
}

func (b Builder) prepare(e *Event) {
	e.At = b.at
	if b.nLabels <= len(b.pLabels) {
		// All the labels are in b.pLabels.
		e.pLabels = b.pLabels
		e.Labels = e.pLabels[:b.nLabels]
	} else {
		// All the labels are in b.aLabels; ignore b.pLabels.
		e.Labels = b.aLabels
	}
}

// This allocates. Used for tests.
func (b Builder) Labels() []Label {
	if b.nLabels <= len(b.pLabels) {
		return b.pLabels[:b.nLabels]
	}
	return b.aLabels
}

var eventPool = sync.Pool{New: func() interface{} { return &Event{} }}

func newEvent(parent uint64) *Event {
	e := eventPool.Get().(*Event)
	e.Parent = parent
	return e
}

func freeEvent(e *Event) {
	e.At = time.Time{}
	e.Message = ""
	eventPool.Put(e)
}
