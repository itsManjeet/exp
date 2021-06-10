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

// Target is a bound exporter.
// Normally you get a target by looking in the context using To.
type Target struct {
	ctx  context.Context
	data *target
}

type target struct {
	exporter  *Exporter
	parent    uint64
	startTime time.Time // for trace latency
}

// Prototype can be used to prepare information for an event.
// You combine Prototype with a Target to form a Builder.
type Prototype struct {
	Event Event
}

// Builder is used to create and send an event.
// The full way to create a Builder is to get a Target and combine it with a
// Prototype.
// There are helpers that do this for you for the common use cases.
type Builder struct {
	ctx       context.Context
	data      *builder
	builderID uint64 // equals data.id if all is well
}

type builder struct {
	target *target
	Event  Event
	labels [preallocateLabels]Label
	id     uint64
}

// preallocateLabels controls the space reserved for labels in a builder.
// Storing the first few labels directly in builders can avoid an allocation at
// all for the very common cases of simple events. The length needs to be large
// enough to cope with the majority of events but no so large as to cause undue
// stack pressure.
const preallocateLabels = 6

var builderPool = sync.Pool{New: func() interface{} { return &builder{} }}

// To returns the Target for the supplied context.
func To(ctx context.Context) Target {
	var t *target
	if v, ok := ctx.Value(contextKey).(*target); ok {
		t = v
	} else {
		t = getDefaultTarget()
	}
	return Target{ctx: ctx, data: t}
}

func (t Target) With(p Prototype) Builder {
	b := t.builder()
	if p.Event.Namespace != "" {
		b.data.Event.Namespace = p.Event.Namespace
	}
	if p.Event.Kind != unknownKind {
		b.data.Event.Kind = p.Event.Kind
	}
	if p.Event.Message != "" {
		b.data.Event.Message = p.Event.Message
	}
	if p.Event.Name != "" {
		b.data.Event.Name = p.Event.Name
	}
	if p.Event.Error != nil {
		b.data.Event.Error = p.Event.Error
	}
	// TODO: we know we are dealing with an empty event, we can probably fill it faster than this
	b = b.Labels(p.Event.Labels...)
	return b
}

func (t Target) As(kind Kind) Builder {
	return t.builder().As(kind)
}

func (t Target) In(namespace string) Builder {
	return t.builder().In(namespace)
}

func (t Target) Name(name string) Builder {
	return t.builder().Name(name)
}

func (t Target) Message(msg string) Builder {
	return t.builder().Message(msg)
}

func (t Target) Label(label Label) Builder {
	return t.builder().Label(label)
}

func (t Target) String(key string, value string) Builder {
	return t.builder().String(key, value)
}

func (t Target) Int(key string, value int) Builder {
	return t.builder().Int(key, value)
}

func (t Target) builder() Builder {
	b := Builder{ctx: t.ctx}
	if t.data != nil {
		b.data = builderPool.Get().(*builder)
		b.data.id = atomic.AddUint64(&builderID, 1)
		b.builderID = b.data.id
		b.data.target = t.data
		b.data.Event.Labels = b.data.labels[:0]
		b.data.Event.Parent = t.data.parent
	}
	return b
}

func (p Prototype) As(kind Kind) Prototype {
	p.Event.Kind = kind
	return p
}

func (p Prototype) In(namespace string) Prototype {
	p.Event.Namespace = namespace
	return p
}

func (p Prototype) Capture() Prototype {
	//TODO: currently the pc lookup map is on the exporter, need away to do it here
	return p
}

func (p Prototype) Name(name string) Prototype {
	p.Event.Name = name
	return p
}

func (p Prototype) Message(msg string) Prototype {
	p.Event.Message = msg
	return p
}

func (p Prototype) Label(label Label) Prototype {
	p.Event.Labels = append(p.Event.Labels, label)
	return p
}

func (p Prototype) String(key string, value string) Prototype {
	p.Event.Labels = append(p.Event.Labels, Label{Name: key, Value: StringOf(value)})
	return p
}

func (p Prototype) Int(key string, value int) Prototype {
	p.Event.Labels = append(p.Event.Labels, Label{Name: key, Value: Int64Of(int64(value))})
	return p
}

func (b Builder) Active() bool {
	return b.data != nil
}

// Send must be called on every builder exactly once
func (b Builder) Send() context.Context {
	ctx := b.ctx
	if b.data == nil {
		return ctx
	}
	if b.data.target == nil || b.data.id != b.builderID {
		panic("Builder already sent the event; must only be called once")
	}

	// get the event ready to send
	b.data.target.exporter.prepare(&b.data.Event)

	// this must come before we take the locks
	if b.data.Event.Kind == TraceKind && b.data.Event.TraceID == 0 {
		// this was an end event, do we need to send a duration?
		if v, ok := DurationMetric.Find(&b.data.Event); ok {
			//TODO: do we want the rest of the values from the end event?
			v.(*Duration).Record(ctx, b.data.Event.At.Sub(b.data.target.startTime))
		}
	}
	b.deliver()
	return ctx
}

// start is for internal use, called instead of send for start events.
func (b Builder) start() Builder {
	// finish preparing the start event
	b.data.target.exporter.prepare(&b.data.Event)
	b.data.Event.TraceID = atomic.AddUint64(&b.data.target.exporter.lastEvent, 1)
	b.ctx = newContext(b.ctx, b.data.target.exporter, b.data.Event.TraceID, b.data.Event.At)
	// build an end builder
	endB := Target{data: b.data.target}.builder()
	endB.data.Event.Kind = TraceKind
	endB.data.Event.Name = b.data.Event.Name
	endB.data.target.startTime = b.data.Event.At
	endB.data.Event.Parent = b.data.Event.TraceID
	// and deliver the start event, capturing the resulting context for the end
	endB.ctx = b.deliver()
	return endB
}

func (b Builder) deliver() context.Context {
	// now hold the lock while we deliver the event
	ctx := b.ctx
	b.data.target.exporter.mu.Lock()
	defer b.data.target.exporter.mu.Unlock()
	switch b.data.Event.Kind {
	case LogKind:
		b.data.target.exporter.handler.Log(ctx, &b.data.Event)
	case MetricKind:
		b.data.target.exporter.handler.Metric(ctx, &b.data.Event)
	case TraceKind:
		if b.data.Event.TraceID != 0 {
			ctx = b.data.target.exporter.handler.Start(ctx, &b.data.Event)
		} else {
			b.data.target.exporter.handler.End(ctx, &b.data.Event)
		}
	default:
		b.data.target.exporter.handler.Annotate(ctx, &b.data.Event)
	}
	*b.data = builder{}
	builderPool.Put(b.data)
	return ctx
}

func (b Builder) As(kind Kind) Builder {
	if b.data == nil {
		return b
	}
	enabled := true
	switch kind {
	case LogKind:
		enabled = b.data.target.exporter.loggingEnabled()
	case MetricKind:
		enabled = b.data.target.exporter.metricsEnabled()
	case TraceKind:
		enabled = b.data.target.exporter.tracingEnabled()
	default:
		enabled = b.data.target.exporter.annotationsEnabled()
	}
	if !enabled {
		b.data = nil
		return b
	}
	b.data.Event.Kind = kind
	return b
}

func (b Builder) In(namespace string) Builder {
	b.data.Event.Namespace = namespace
	return b
}

func (b Builder) At(at time.Time) Builder {
	b.data.Event.At = at
	return b
}

func (b Builder) Capture() Builder {
	if b.Active() {
		//TODO: a better way of working out the stack depth
		// Get the pc of the user function that delivered the event.
		// This is sensitive to the call stack.
		// 0: runtime.Callers
		// 1: importPath
		// 2: Exporter.capture
		// 3: Exporter.Capture (this function)
		// 4: user function
		b.data.target.exporter.capture(&b.data.Event, 4)
	}
	return b
}

func (b Builder) Name(name string) Builder {
	if b.Active() {
		b.data.Event.Name = name
	}
	return b
}

func (b Builder) Message(msg string) Builder {
	if b.Active() {
		b.data.Event.Message = msg
	}
	return b
}

func (b Builder) Error(err error) Builder {
	if b.Active() {
		b.data.Event.Error = err
	}
	return b
}

func (b Builder) Label(label Label) Builder {
	if b.Active() {
		b.data.Event.Labels = append(b.data.Event.Labels, label)
	}
	return b
}

func (b Builder) Labels(labels ...Label) Builder {
	if b.Active() {
		b.data.Event.Labels = append(b.data.Event.Labels, labels...)
	}
	return b
}

func (b Builder) String(key string, value string) Builder {
	if b.Active() {
		b.data.Event.Labels = append(b.data.Event.Labels, Label{Name: key, Value: StringOf(value)})
	}
	return b
}

func (b Builder) Int(key string, value int) Builder {
	if b.Active() {
		b.data.Event.Labels = append(b.data.Event.Labels, Label{Name: key, Value: Int64Of(int64(value))})
	}
	return b
}

var builderID uint64 // atomic

func clone(old *builder) *builder {
	if old == nil {
		return nil
	}
	data := builderPool.Get().(*builder)
	*data = *old
	data.id = atomic.AddUint64(&builderID, 1)
	data.Event.Labels = data.labels[:0]
	if len(old.Event.Labels) == 0 || &old.labels[0] == &old.Event.Labels[0] {
		data.Event.Labels = data.labels[:len(old.Event.Labels)]
	} else {
		data.Event.Labels = make([]Label, len(old.Event.Labels))
		copy(data.Event.Labels, old.Event.Labels)
	}
	return data
}
