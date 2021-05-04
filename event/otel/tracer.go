// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package otel

import (
	"context"
	"fmt"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/exp/event"
	"golang.org/x/exp/event/keys"
)

type tracerProvider struct {
	tp *sdktrace.TracerProvider
}

func NewTracerProvider(tp *sdktrace.TracerProvider) trace.TracerProvider {
	return &tracerProvider{tp}
}

type tracer struct {
	tracer trace.Tracer
	info   map[uint64]cs // store the context and span returned by Tracer.Start, indexed by parent ID
}

type cs struct {
	ctx  context.Context
	span trace.Span
}

func (tp *tracerProvider) Tracer(instrumentationName string, opts ...trace.TracerOption) trace.Tracer {
	return &tracer{
		tracer: tp.tp.Tracer(instrumentationName, opts...),
		info:   map[uint64]cs{},
	}
}

// The otel Start method calls event.Start, which does all the work, including calling the actual sdk tracer.
// It communicates the results via labels that contain pointers.
func (t *tracer) Start(ctx context.Context, spanName string, opts ...trace.SpanOption) (context.Context, trace.Span) {
	var rctx context.Context
	var span trace.Span
	ectx, end := event.Start(ctx, spanName, keys.Value("_ctx").Of(&rctx), keys.Value("_span").Of(&span))
	_ = end // never called; the user will call span.End instead.
	return ectx, span
}

func (t *tracer) Handle(e *event.Event) {
	switch e.Kind {
	case event.StartKind:
		t.handleStart(e)
	case event.EndKind:
		cs := t.info[e.Parent]
		cs.span.End()

	default:
		fmt.Printf("got other event of kind %s\n", e.Kind)
	}
}

func (t *tracer) handleStart(e *event.Event) {
	// Get the context containing the parent otel span information.
	ctx := context.Background()
	if e.Parent != 0 {
		pcs := t.info[e.Parent]
		ctx = pcs.ctx
	}
	// Call the otel Start method to get a new context and a span.
	ctx, span := t.tracer.Start(ctx, e.Message)
	// Remember them so child events can find them.
	t.info[e.ID] = cs{ctx, span}
	// Set them into labels to return them.
	for _, l := range e.Labels {
		if l.Name == "_ctx" {
			*l.Value.Interface().(*context.Context) = ctx
		} else if l.Name == "_span" {
			*l.Value.Interface().(*trace.Span) = span
		}
	}
}
