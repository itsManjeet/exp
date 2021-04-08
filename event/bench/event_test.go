// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bench_test

import (
	"context"
	"io"
	"testing"

	"golang.org/x/exp/event"
	"golang.org/x/exp/event/eventtest"
	"golang.org/x/exp/event/keys"
)

var (
	aValue  = keys.NewInt(aName, "")
	bValue  = keys.NewString(bName, "")
	aCount  = keys.NewInt64("aCount", "Count of time A is called.")
	aStat   = keys.NewInt("aValue", "A value.")
	bCount  = keys.NewInt64("B", "Count of time B is called.")
	bLength = keys.NewInt("BLen", "B length.")

	eventLog = Hooks{
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
	}

	eventLogf = Hooks{
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
	}

	eventTrace = Hooks{
		AStart: func(ctx context.Context, a int) context.Context {
			ctx = event.Start(ctx, aMsg)
			event.To(ctx).With(aValue.Of(a)).Annotate()
			return ctx
		},
		AEnd: func(ctx context.Context) {
			event.To(ctx).End()
		},
		BStart: func(ctx context.Context, b string) context.Context {
			ctx = event.Start(ctx, bMsg)
			event.To(ctx).With(bValue.Of(b)).Annotate()
			return ctx
		},
		BEnd: func(ctx context.Context) {
			event.To(ctx).End()
		},
	}

	eventMetric = Hooks{
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
	}
)

func eventDisabled() context.Context {
	event.Disable()
	return context.Background()
}

func eventNoExporter() context.Context {
	// register an exporter to turn the system on, but not in this context
	event.NewExporter(noopHandler{})
	return context.Background()
}

func eventNoop() context.Context {
	return event.WithExporter(context.Background(),
		event.NewExporter(noopHandler{}))
}

func eventPrint(w io.Writer) context.Context {
	return event.WithExporter(context.Background(),
		event.NewExporter(event.NewPrinter(w)))
}

func eventRewrite(w io.Writer) context.Context {
	return event.WithExporter(context.Background(),
		event.NewExporter(eventtest.Rewriter(event.NewPrinter(w))))
}

func BenchmarkLogEventDisabled(b *testing.B) {
	runBenchmark(b, eventDisabled(), eventLog)
}

func BenchmarkLogEventNoExporter(b *testing.B) {
	runBenchmark(b, eventNoExporter(), eventLog)
}

func BenchmarkLogEventNoop(b *testing.B) {
	runBenchmark(b, eventNoop(), eventLog)
}

func BenchmarkLogEventDiscard(b *testing.B) {
	runBenchmark(b, eventPrint(io.Discard), eventLog)
}

func BenchmarkLogEventfDiscard(b *testing.B) {
	runBenchmark(b, eventPrint(io.Discard), eventLogf)
}

func TestLogEventf(t *testing.T) {
	testBenchmark(t, eventRewrite, eventLogf, `
2020/03/05 14:27:48 [log:1] A where a=0
2020/03/05 14:27:49 [log:2] b where b="A value"
2020/03/05 14:27:50 [log:3] A where a=1
2020/03/05 14:27:51 [log:4] b where b="Some other value"
2020/03/05 14:27:52 [log:5] A where a=22
2020/03/05 14:27:53 [log:6] b where b="Some other value"
2020/03/05 14:27:54 [log:7] A where a=333
2020/03/05 14:27:55 [log:8] b where b=""
2020/03/05 14:27:56 [log:9] A where a=4444
2020/03/05 14:27:57 [log:10] b where b="prime count of values"
2020/03/05 14:27:58 [log:11] A where a=55555
2020/03/05 14:27:59 [log:12] b where b="V"
2020/03/05 14:28:00 [log:13] A where a=666666
2020/03/05 14:28:01 [log:14] b where b="A value"
2020/03/05 14:28:02 [log:15] A where a=7777777
2020/03/05 14:28:03 [log:16] b where b="A value"
`)
}

func BenchmarkTraceEventNoop(b *testing.B) {
	runBenchmark(b, eventPrint(io.Discard), eventTrace)
}

func BenchmarkMetricEventNoop(b *testing.B) {
	runBenchmark(b, eventPrint(io.Discard), eventMetric)
}

type noopHandler struct{}

func (noopHandler) Handle(ev *event.Event) {}
