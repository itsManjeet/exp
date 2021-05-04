// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package otel

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/exp/event"
)

func TestTrace(t *testing.T) {
	want := "root (f (g h) p (q r))"

	for _, tfunc := range []func(int) bool{
		func(int) bool { return true },
		func(int) bool { return false },
		func(i int) bool { return i%2 == 0 },
		func(i int) bool { return i%2 == 1 },
		func(i int) bool { return i%3 == 0 },
		func(i int) bool { return i%3 == 1 },
	} {
		ctx, tr, shutdown := setupOtel()
		tracers := make([]trace.Tracer, 7)
		for i := 0; i < len(tracers); i++ {
			if tfunc(i) {
				tracers[i] = tr
			}
		}
		s := makeTraceSpec(tracers)
		s.apply(ctx)
		got := shutdown()
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}

func makeTraceSpec(tracers []trace.Tracer) *traceSpec {
	return &traceSpec{
		name:   "root",
		tracer: tracers[0],
		children: []*traceSpec{
			{
				name:   "f",
				tracer: tracers[1],
				children: []*traceSpec{
					{name: "g", tracer: tracers[2]},
					{name: "h", tracer: tracers[3]},
				},
			},
			{
				name:   "p",
				tracer: tracers[4],
				children: []*traceSpec{
					{name: "q", tracer: tracers[5]},
					{name: "r", tracer: tracers[6]},
				},
			},
		},
	}
}

type traceSpec struct {
	name     string
	tracer   trace.Tracer // nil for event
	children []*traceSpec
}

func (s *traceSpec) apply(ctx context.Context) {
	if s.tracer != nil {
		var span trace.Span
		ctx, span = s.tracer.Start(ctx, s.name)
		defer span.End()
	} else {
		var end func()
		ctx, end = event.Start(ctx, s.name)
		defer end()
	}
	for _, c := range s.children {
		c.apply(ctx)
	}
}

func TestEventTrace(t *testing.T) {
	ctx, _, shutdown := setupOtel()
	defer shutdown()

	s := &traceSpec{
		name: "root",
		children: []*traceSpec{
			{
				name: "f",
				children: []*traceSpec{
					{name: "g"},
					{name: "h"},
				},
			},
			{
				name: "p",
				children: []*traceSpec{
					{name: "q"},
					{name: "r"},
				},
			},
		},
	}

	s.apply(ctx)
}

func TestMixTrace(t *testing.T) {
	ctx, tr, shutdown := setupOtel()
	defer shutdown()

	s := &traceSpec{
		name: "root",
		children: []*traceSpec{
			{
				name:   "f",
				tracer: tr,
				children: []*traceSpec{
					{name: "g"},
					{name: "h"},
				},
			},
			{
				name: "p",
				children: []*traceSpec{
					{name: "q"},
					{name: "r", tracer: tr},
				},
			},
		},
	}
	s.apply(ctx)
}

func setupOtel() (context.Context, trace.Tracer, func() string) {
	ctx := context.Background()
	e := newTestExporter()
	bsp := sdktrace.NewSimpleSpanProcessor(e)
	stp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(bsp))
	tp := NewTracerProvider(stp)
	tr := tp.Tracer("t")
	ee := event.NewExporter(tr.(*tracer))
	ctx = event.WithExporter(ctx, ee)
	return ctx, tr, func() string { stp.Shutdown(ctx); return e.got }
}

type testExporter struct {
	m   map[trace.SpanID][]*sdktrace.SpanSnapshot // key is parent SpanID
	got string
}

func newTestExporter() *testExporter {
	return &testExporter{m: map[trace.SpanID][]*sdktrace.SpanSnapshot{}}
}

func (e *testExporter) ExportSpans(ctx context.Context, ss []*sdktrace.SpanSnapshot) error {
	for _, s := range ss {
		sid := s.Parent.SpanID()
		e.m[sid] = append(e.m[sid], s)
	}
	return nil
}

func (e *testExporter) Shutdown(ctx context.Context) error {
	root := e.m[trace.SpanID{}][0]
	var buf bytes.Buffer
	e.print(&buf, root)
	e.got = buf.String()
	return nil
}

func (e *testExporter) print(w io.Writer, ss *sdktrace.SpanSnapshot) {
	fmt.Fprintf(w, "%s", ss.Name)
	children := e.m[ss.SpanContext.SpanID()]
	if len(children) > 0 {
		fmt.Fprint(w, " (")
		for i, ss := range children {
			if i != 0 {
				fmt.Fprint(w, " ")
			}
			e.print(w, ss)
		}
		fmt.Fprint(w, ")")
	}
}
