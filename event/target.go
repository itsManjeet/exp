// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package event

import (
	"context"
	"sync"
	"time"
)

type Target struct {
	ctx       context.Context
	builder   *Builder
	cv        *contextValue
	name      string
	namespace string
	at        time.Time
	err       error
}

func To(ctx context.Context) Target {
	t := Target{ctx: ctx}
	t.cv = fromContext(ctx)
	return t
}

func (t Target) At(tm time.Time) Target {
	t.at = tm
	return t
}

// Question: what does Name mean for events other than Start?
func (t Target) Name(n string) Target {
	t.name = n
	return t
}

func (t Target) Namespace(ns string) Target {
	t.namespace = ns
	return t
}

func (t Target) Error(err error) Target {
	t.err = err
	return t
}

func (t Target) Log(msg string, labels ...Label) {
	if t.cv.exporter == nil || !t.cv.exporter.loggingEnabled() {
		return
	}
	e := allocEvent(LogKind)
	defer freeEvent(e)
	e.Message = msg
	t.populate(e)
	copyLabels(e, t.builder, labels)
	t.cv.exporter.mu.Lock()
	defer t.cv.exporter.mu.Unlock()
	t.cv.exporter.prepare(e)
	t.cv.exporter.handler.Log(t.ctx, e)
}

// TODO: make sure this gets inlined to avoid copy of Target, or measure its
// impact.
func (t Target) populate(e *Event) {
	e.Parent = t.cv.parent
	if !t.at.IsZero() {
		e.At = t.at
	}
	e.Name = t.name
	if t.namespace != "" {
		e.Namespace = t.namespace
	} else if t.builder != nil {
		e.Namespace = t.builder.Namespace
	}
}

func copyLabels(e *Event, b *Builder, ls []Label) {
	var bls []Label
	if b != nil {
		bls = b.Labels
	}
	if len(bls)+len(ls) > preallocateLabels {
		e.Labels = make([]Label, 0, len(bls)+len(ls))
	}
	e.Labels = append(e.Labels, bls...)
	e.Labels = append(e.Labels, ls...)
}

func (t Target) Annotate(labels ...Label) {
	if t.cv.exporter == nil || !t.cv.exporter.annotationsEnabled() {
		return
	}
	e := allocEvent(TraceKind)
	e.Kind = Kind(0) // Question: why not TraceKind?
	defer freeEvent(e)
	t.populate(e)
	copyLabels(e, t.builder, labels)
	t.cv.exporter.mu.Lock()
	defer t.cv.exporter.mu.Unlock()
	t.cv.exporter.prepare(e)
	t.cv.exporter.handler.Annotate(t.ctx, e)
}

func (t Target) Metric(mv MetricValue, labels ...Label) {
	if t.cv.exporter == nil || !t.cv.exporter.metricsEnabled() {
		return
	}
	e := allocEvent(MetricKind)
	defer freeEvent(e)
	t.populate(e)
	if e.Namespace == "" {
		e.Namespace = mv.m.Descriptor().Namespace()
	}
	copyLabels(e, t.builder, labels)
	e.Labels = append(e.Labels, MetricVal.Of(mv.v), MetricKey.Of(mv.m))
	t.cv.exporter.prepare(e)
	t.cv.exporter.handler.Metric(t.ctx, e)
}

func (t Target) Start(name string, labels ...Label) (ctx context.Context, _ Target) {
	if t.cv.exporter == nil || !t.cv.exporter.tracingEnabled() {
		return t.ctx, Target{ctx: t.ctx}
	}
	e := allocEvent(TraceKind)
	defer freeEvent(e)
	t.populate(e)
	e.Name = name
	copyLabels(e, t.builder, labels)
	endTarget := t
	t.cv.exporter.mu.Lock()
	defer t.cv.exporter.mu.Unlock()
	t.cv.exporter.lastEvent++
	e.TraceID = t.cv.exporter.lastEvent
	t.cv.exporter.prepare(e)
	// and now deliver the start event
	now := time.Now()
	cv := &contextValue{t.cv.exporter, e.TraceID, now}
	endTarget.cv = cv
	ctx = newContext(t.ctx, cv)
	ctx = t.cv.exporter.handler.Start(ctx, e)
	endTarget.ctx = ctx
	return ctx, endTarget
}

func (t Target) End(labels ...Label) {
	if t.cv.exporter == nil || !t.cv.exporter.tracingEnabled() {
		return
	}
	e := allocEvent(TraceKind)
	defer freeEvent(e)
	t.populate(e)
	copyLabels(e, t.builder, labels)
	// If there is a DurationMetric label, emit a Metric event
	// with the time since Start was called.
	if v, ok := DurationMetric.Find(e); ok {
		m := v.(*Duration)
		t.Metric(m.Record(time.Since(t.cv.startTime)), labels...)
	}
	t.cv.exporter.mu.Lock()
	defer t.cv.exporter.mu.Unlock()
	t.cv.exporter.prepare(e)
	t.cv.exporter.handler.End(t.ctx, e)
}

var eventPool = sync.Pool{New: func() interface{} { return &Event{} }}

func allocEvent(k Kind) *Event {
	e := eventPool.Get().(*Event)
	e.Kind = k
	e.Labels = e.labels[:0]
	return e
}

func freeEvent(e *Event) {
	*e = Event{}
	eventPool.Put(e)
}
