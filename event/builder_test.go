// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !disable_events

package event_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/exp/event"
	"golang.org/x/exp/event/keys"
)

func valueEqual(l1, l2 event.Value) bool {
	return fmt.Sprint(l1) == fmt.Sprint(l2)
}

func TestTraceBuilder(t *testing.T) {
	// Verify that the context returned from the handler is also returned from Start,
	// and is the context passed to End.
	ctx := event.WithExporter(context.Background(), event.NewExporter(&testTraceHandler{t: t}, nil))
	ctx, eb := event.To(ctx).Start("s")
	val := ctx.Value("x")
	if val != 1 {
		t.Fatal("context not returned from Start")
	}
	eb.End()
}

type testTraceHandler struct {
	event.NopHandler
	t *testing.T
}

func (*testTraceHandler) Start(ctx context.Context, _ *event.Event) context.Context {
	return context.WithValue(ctx, "x", 1)
}

func (t *testTraceHandler) End(ctx context.Context, _ *event.Event) {
	val := ctx.Value("x")
	if val != 1 {
		t.t.Fatal("Start context not passed to End")
	}
}

func TestTraceDuration(t *testing.T) {
	// Verify that a trace can can emit a latency metric.
	dur := event.NewDuration("test", "")
	want := 200 * time.Millisecond

	check := func(t *testing.T, h *testTraceDurationHandler) {
		if !h.got.HasValue() {
			t.Fatal("no metric value")
		}
		got := h.got.Duration().Round(50 * time.Millisecond)
		if got != want {
			t.Fatalf("got %s, want %s", got, want)
		}
	}

	t.Run("returned builder", func(t *testing.T) {
		h := &testTraceDurationHandler{}
		ctx := event.WithExporter(context.Background(), event.NewExporter(h, nil))
		ctx, eb := event.To(ctx).Start("s")
		time.Sleep(want)
		eb.End(event.DurationMetric.Of(dur))
		check(t, h)
	})
	t.Run("separate builder", func(t *testing.T) {
		h := &testTraceDurationHandler{}
		ctx := event.WithExporter(context.Background(), event.NewExporter(h, nil))
		ctx, _ = event.To(ctx).Start("s")
		time.Sleep(want)
		event.To(ctx).End(event.DurationMetric.Of(dur))
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
			ctx := event.WithExporter(context.Background(), event.NewExporter(event.NopHandler{}, nil))
			for i := 0; i < depth; i++ {
				ctx = context.WithValue(ctx, i, i)
			}
			b.Run("direct", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					event.To(ctx).Name("foo").Metric(c.Record(1))
				}
			})
		})
	}
}

func TestBuilder(t *testing.T) {
	l1 := keys.Int("i").Of(3)
	l2 := keys.String("s").Of("x")
	b := event.NewBuilder(l1, l2)
	if got, want := b.Labels, []event.Label{l1, l2}; !cmp.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := b.Namespace, "golang.org/x/exp/event_test"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
