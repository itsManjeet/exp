// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package event_test

import (
	"context"
	"io/ioutil"
	"log"
	"testing"

	"golang.org/x/exp/event"
	"golang.org/x/exp/event/export"
	"golang.org/x/exp/event/keys"
)

type Hooks struct {
	AStart func(ctx context.Context, a int) context.Context
	AEnd   func(ctx context.Context)
	BStart func(ctx context.Context, b string) context.Context
	BEnd   func(ctx context.Context)
}

var (
	aValue  = keys.NewInt("a", "")
	bValue  = keys.NewString("b", "")
	aCount  = keys.NewInt64("aCount", "Count of time A is called.")
	aStat   = keys.NewInt("aValue", "A value.")
	bCount  = keys.NewInt64("B", "Count of time B is called.")
	bLength = keys.NewInt("BLen", "B length.")

	Baseline = Hooks{
		AStart: func(ctx context.Context, a int) context.Context { return ctx },
		AEnd:   func(ctx context.Context) {},
		BStart: func(ctx context.Context, b string) context.Context { return ctx },
		BEnd:   func(ctx context.Context) {},
	}

	StdLog = Hooks{
		AStart: func(ctx context.Context, a int) context.Context {
			log.Printf("A where a=%d", a)
			return ctx
		},
		AEnd: func(ctx context.Context) {},
		BStart: func(ctx context.Context, b string) context.Context {
			log.Printf("B where b=%q", b)
			return ctx
		},
		BEnd: func(ctx context.Context) {},
	}

	Log = Hooks{
		AStart: func(ctx context.Context, a int) context.Context {
			event.Log1(ctx, "A", aValue.Of(a))
			return ctx
		},
		AEnd: func(ctx context.Context) {},
		BStart: func(ctx context.Context, b string) context.Context {
			event.Log1(ctx, "B", bValue.Of(b))
			return ctx
		},
		BEnd: func(ctx context.Context) {},
	}

	Trace = Hooks{
		AStart: func(ctx context.Context, a int) context.Context {
			return event.Start1(ctx, "A", aValue.Of(a))
		},
		AEnd: func(ctx context.Context) {
			event.End(ctx)
		},
		BStart: func(ctx context.Context, b string) context.Context {
			return event.Start1(ctx, "B", bValue.Of(b))
		},
		BEnd: func(ctx context.Context) {
			event.End(ctx)
		},
	}

	Stats = Hooks{
		AStart: func(ctx context.Context, a int) context.Context {
			event.Metric1(ctx, aStat.Of(a))
			event.Metric1(ctx, aCount.Of(1))
			return ctx
		},
		AEnd: func(ctx context.Context) {},
		BStart: func(ctx context.Context, b string) context.Context {
			event.Metric1(ctx, bLength.Of(len(b)))
			event.Metric1(ctx, bCount.Of(1))
			return ctx
		},
		BEnd: func(ctx context.Context) {},
	}

	initialList = []int{0, 1, 22, 333, 4444, 55555, 666666, 7777777}
	stringList  = []string{
		"A value",
		"Some other value",
		"A nice longer value but not too long",
		"V",
		"",
		"ı",
		"prime count of values",
	}
)

type namedBenchmark struct {
	name string
	test func(ctx context.Context) func(*testing.B)
}

func Benchmark(b *testing.B) {
	ctx := context.Background()
	b.Run("Baseline", Baseline.runBenchmark(ctx))
	b.Run("StdLog", StdLog.runBenchmark(ctx))
	benchmarks := []namedBenchmark{
		{"Log", Log.runBenchmark},
		{"Trace", Trace.runBenchmark},
		{"Stats", Stats.runBenchmark},
	}

	for _, t := range benchmarks {
		b.Run(t.name+"NoExporter", t.test(ctx))
	}

	for _, t := range benchmarks {
		ctx := event.WithExporter(context.Background(), noopExporter{})
		b.Run(t.name+"Noop", t.test(ctx))
	}

	for _, t := range benchmarks {
		ctx := event.WithExporter(ctx,
			export.LogWriter(event.NewPrinter(ioutil.Discard), false))
		b.Run(t.name, t.test(ctx))
	}
}

func benchA(ctx context.Context, hooks Hooks, a int) int {
	ctx = hooks.AStart(ctx, a)
	defer hooks.AEnd(ctx)
	return benchB(ctx, hooks, a, stringList[a%len(stringList)])
}

func benchB(ctx context.Context, hooks Hooks, a int, b string) int {
	ctx = hooks.BStart(ctx, b)
	defer hooks.BEnd(ctx)
	return a + len(b)
}

func (hooks Hooks) runBenchmark(ctx context.Context) func(b *testing.B) {
	return func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		var acc int
		for i := 0; i < b.N; i++ {
			for _, value := range initialList {
				acc += benchA(ctx, hooks, value)
			}
		}
	}
}

func init() {
	log.SetOutput(ioutil.Discard)
}

type noopExporter struct{}

func (noopExporter) Export(ctx context.Context, ev event.Event) {}
