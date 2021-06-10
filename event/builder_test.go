// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !disable_events

package event_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"golang.org/x/exp/event"
	"golang.org/x/exp/event/eventtest"
)

func TestTraceBuilder(t *testing.T) {
	// Verify that the context returned from the handler is also returned from Start,
	// and is the context passed to End.
	ctx := event.WithExporter(context.Background(), event.NewExporter(&testTraceHandler{t: t}, eventtest.ExporterOptions()))
	t.Logf("start")
	ctx, eb := event.Start(ctx, "s")
	t.Logf("started")
	val := ctx.Value("x")
	if val != 1 {
		t.Fatal("context not returned from Start")
	}
	t.Logf("val was %v", val)
	eb.Send()
}

type testTraceHandler struct {
	event.NopHandler
	t *testing.T
}

func (t *testTraceHandler) Start(ctx context.Context, _ *event.Event) context.Context {
	t.t.Logf("handle start")
	return context.WithValue(ctx, "x", 1)
}

func (t *testTraceHandler) End(ctx context.Context, _ *event.Event) {
	t.t.Logf("handle end")
	val := ctx.Value("x")
	if val != 1 {
		t.t.Fatal("Start context not passed to End")
	}
}

func TestFailToClone(t *testing.T) {
	ctx := event.WithExporter(context.Background(), event.NewExporter(event.NopHandler{}, eventtest.ExporterOptions()))

	catch := func(f func()) {
		defer func() {
			r := recover()
			if r == nil {
				t.Error("expected panic, did not get one")
				return
			}
			const expect = "must only be called once"
			got, ok := r.(string)
			if !ok || !strings.Contains(got, expect) {
				t.Errorf("got panic(%v), want string with '%s'", r, expect)
			}
		}()

		f()
	}

	catch(func() {
		b1 := event.To(ctx).As(event.LogKind)
		b1.Message("msg1").Send()
		// Reuse of Builder without Clone; b1.data has been cleared.
		b1.Message("msg2").Send()
	})

	catch(func() {
		b1 := event.To(ctx).As(event.LogKind)
		b1.Message("msg1").Send()
		_ = event.To(ctx) // re-allocate the builder
		// b1.data is populated, but with the wrong information.
		b1.Message("msg2").Send()
	})
}

func TestTraceDuration(t *testing.T) {
	// Verify that a trace can can emit a latency metric.
	dur := event.NewDuration("test", "")
	want := time.Second

	check := func(t *testing.T, h *testTraceDurationHandler) {
		if !h.got.HasValue() {
			t.Fatal("no metric value")
		}
		got := h.got.Duration()
		if got != want {
			t.Fatalf("got %s, want %s", got, want)
		}
	}

	t.Run("returned builder", func(t *testing.T) {
		h := &testTraceDurationHandler{}
		ctx := event.WithExporter(context.Background(), event.NewExporter(h, eventtest.ExporterOptions()))
		ctx, eb := event.Start(ctx, "s")
		time.Sleep(want)
		eb.Label(event.DurationMetric.Of(dur)).Send()
		check(t, h)
	})
	t.Run("separate builder", func(t *testing.T) {
		h := &testTraceDurationHandler{}
		ctx := event.WithExporter(context.Background(), event.NewExporter(h, eventtest.ExporterOptions()))
		ctx, _ = event.Start(ctx, "s")
		time.Sleep(want)
		event.To(ctx).As(event.TraceKind).Label(event.DurationMetric.Of(dur)).Send()
		check(t, h)
	})
}

type testTraceDurationHandler struct {
	event.NopHandler
	got event.Value
}

func (t *testTraceDurationHandler) Metric(ctx context.Context, e *event.Event) {
	t.got, _ = event.MetricVal.Find(e)
}

func BenchmarkBuildContext(b *testing.B) {
	// How long does it take to deliver an event from a nested context?
	c := event.NewCounter("c", "")
	for _, depth := range []int{1, 5, 7, 10} {
		b.Run(fmt.Sprintf("depth %d", depth), func(b *testing.B) {
			ctx := event.WithExporter(context.Background(), event.NewExporter(event.NopHandler{}, eventtest.ExporterOptions()))
			for i := 0; i < depth; i++ {
				ctx = context.WithValue(ctx, i, i)
			}
			b.Run("direct", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					c.Record(ctx, 1)
				}
			})
			b.Run("cloned", func(b *testing.B) {
				bu := event.To(ctx)
				for i := 0; i < b.N; i++ {
					c.RecordTB(bu, 1).Name("foo").Send()
				}
			})
		})
	}
}
