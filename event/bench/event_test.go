// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bench_test

import (
	"context"
	"io/ioutil"
	"testing"

	"golang.org/x/exp/event"
	"golang.org/x/exp/event/export"
	"golang.org/x/exp/event/keys"
)

var (
	aValue  = keys.NewInt(aName, "")
	bValue  = keys.NewString(bName, "")
	aCount  = keys.NewInt64("aCount", "Count of time A is called.")
	aStat   = keys.NewInt("aValue", "A value.")
	bCount  = keys.NewInt64("B", "Count of time B is called.")
	bLength = keys.NewInt("BLen", "B length.")
)

func runEventLog(b *testing.B, ctx context.Context) {
	runBenchmark(b, ctx, Hooks{
		AStart: func(ctx context.Context, a int) context.Context {
			event.To(ctx).With(aValue.Of(a)).Log(aMsg)
			return ctx
		},
		AEnd: func(ctx context.Context) {},
		BStart: func(ctx context.Context, b string) context.Context {
			event.To(ctx).With(bValue.Of(b)).Log(bMsg)
			return ctx
		},
		BEnd: func(ctx context.Context) {},
	})
}

func runEventLogf(b *testing.B, ctx context.Context) {
	runBenchmark(b, ctx, Hooks{
		AStart: func(ctx context.Context, a int) context.Context {
			event.To(ctx).Logf(aMsgf, a)
			return ctx
		},
		AEnd: func(ctx context.Context) {},
		BStart: func(ctx context.Context, b string) context.Context {
			event.To(ctx).Logf(bMsgf, b)
			return ctx
		},
		BEnd: func(ctx context.Context) {},
	})
}

func runEventTrace(b *testing.B, ctx context.Context) {
	runBenchmark(b, ctx, Hooks{
		AStart: func(ctx context.Context, a int) context.Context {
			return event.To(ctx).With(aValue.Of(a)).Start(aMsg)
		},
		AEnd: func(ctx context.Context) {
			event.To(ctx).End()
		},
		BStart: func(ctx context.Context, b string) context.Context {
			return event.To(ctx).With(bValue.Of(b)).Start(bMsg)
		},
		BEnd: func(ctx context.Context) {
			event.To(ctx).End()
		},
	})
}

func runEventMetric(b *testing.B, ctx context.Context) {
	runBenchmark(b, ctx, Hooks{
		AStart: func(ctx context.Context, a int) context.Context {
			event.To(ctx).With(aStat.Of(a)).Metric()
			event.To(ctx).With(aCount.Of(1)).Metric()
			return ctx
		},
		AEnd: func(ctx context.Context) {},
		BStart: func(ctx context.Context, b string) context.Context {
			event.To(ctx).With(bLength.Of(len(b))).Metric()
			event.To(ctx).With(bCount.Of(1)).Metric()
			return ctx
		},
		BEnd: func(ctx context.Context) {},
	})
}

func runEventDisabled(b *testing.B, f func(b *testing.B, ctx context.Context)) {
	//TODO: disable the exporter
	event.Disable()
	ctx := context.Background()
	f(b, ctx)
}

func runEventNoExporter(b *testing.B, f func(b *testing.B, ctx context.Context)) {
	ctx := context.Background()
	// register an exporter to turn the system on, but not in this context
	_ = event.WithExporter(context.Background(), noopExporter{})
	f(b, ctx)
}

func runEventNoop(b *testing.B, f func(b *testing.B, ctx context.Context)) {
	ctx := event.WithExporter(context.Background(), noopExporter{})
	f(b, ctx)
}

func runEventDiscard(b *testing.B, f func(b *testing.B, ctx context.Context)) {
	ctx := event.WithExporter(context.Background(),
		export.LogWriter(event.NewPrinter(ioutil.Discard), false))
	f(b, ctx)
}

func BenchmarkLogEventDisabled(b *testing.B) {
	runEventDisabled(b, runEventLog)
}

func BenchmarkLogEventNoExporter(b *testing.B) {
	runEventNoExporter(b, runEventLog)
}

func BenchmarkLogEventNoop(b *testing.B) {
	runEventNoop(b, runEventLog)
}

func BenchmarkLogEventDiscard(b *testing.B) {
	runEventDiscard(b, runEventLog)
}

func BenchmarkLogEventfDiscard(b *testing.B) {
	runEventDiscard(b, runEventLogf)
}

func BenchmarkTraceEventNoop(b *testing.B) {
	runEventDiscard(b, runEventTrace)
}

func BenchmarkMetricEventNoop(b *testing.B) {
	runEventDiscard(b, runEventMetric)
}

type noopExporter struct{}

func (noopExporter) Export(ev *event.Event) {}
